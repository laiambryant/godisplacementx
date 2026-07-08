//go:build simd && amd64

// Package gen SIMD kernels, built only with `GOEXPERIMENT=simd` and `-tags simd`
// on amd64. They accelerate the pixel-buffer hot paths using simd/archsimd.
//
// Only AVX2 is targeted: this keeps the kernels usable on the broad amd64
// baseline and avoids the AVX-512-only operations in the API (mask registers,
// VPMOV* narrowing, unsigned int->float conversion). All ops used here lower to
// AVX/AVX2 instructions; the runtime `useAVX2` guard falls back to scalar on
// CPUs without AVX2.
//
// Results are allowed to differ from the scalar reference by at most 1 per
// channel (float32 lanes vs the reference float64), which the SIMD tests assert.
//
// Design notes:
//   - The pixel buffer is straight RGBA in a flat []uint8. FillRect's source is
//     a constant grayscale colour (csR=csG=csB) drawn onto an opaque backdrop,
//     so every RGB byte in a row undergoes the *same* scalar function f(in) and
//     the alpha byte is simply left untouched. That lets us vectorise the flat
//     byte stream (2 pixels / 8 float32 lanes per step) with no RGBA
//     de-interleaving.
//   - Correctness is guaranteed by falling back to the exact scalar kernel for
//     any 2-pixel step whose backdrop is not fully opaque, for the run tail, and
//     for the modes that are not separable (source-atop, xor, lighter,
//     luminosity).

package gen

import "simd/archsimd"

// useAVX2 gates the SIMD path on the CPU actually supporting AVX2 at runtime;
// otherwise we run the scalar reference. Evaluated once at package init.
var useAVX2 = archsimd.X86.AVX2()

// --- dispatch wrappers (resolved by the caller via canvas.go / post.go) ---

func fillRun(pix []uint8, gray uint8) {
	if useAVX2 {
		fillSIMD(pix, gray)
		return
	}
	fillScalar(pix, gray)
}

// blendRun dispatches the per-row compositor. It stays scalar in production for
// every mode. Two vectorised compositors exist but are kept off the hot path,
// each for its own benchmark-proven reason (both are exercised only by the A/B
// benchmarks and tolerance tests; see docs/SIMD.md):
//
//   - blendRunSIMD (float32) vectorises all 16 modes but is ~25-40x slower than
//     the tight scalar float64 loop -- at 2 px/step the uint8<->float32
//     conversions and the long archsimd chain dominate.
//   - blendSourceOverSIMD (integer Q8) is the opposite: on a single wide run it
//     is ~2-3x faster than scalar (no float conversion). But FillRect calls
//     blendRun once per scanline of every primitive, and the real draw loop is
//     dominated by many short runs (grid/line/column cells a few dozen pixels
//     wide). There the per-call vector setup (coefficient + shuffle-index vectors)
//     is not amortised and end-to-end Generate regresses ~15x. A production win
//     would need that setup hoisted to FillRect (once per rect, not per row).
//
// Only the wide, conversion-free, setup-free fill/invert passes use SIMD in
// production, where they are ~6x faster.
func blendRun(pix []uint8, di, n int, g, sa float64, mode CompositionMode) {
	blendRunScalar(pix, di, n, g, sa, mode)
}

func invertRun(pix []uint8) {
	if useAVX2 {
		invertSIMD(pix)
		return
	}
	invertScalar(pix)
}

// colorRun (256-entry LUT gather) and normalRun (strided neighbour access) have
// no clean AVX2 lowering in this API surface, so they stay scalar even in the
// SIMD build. They remain here as dispatch wrappers for symmetry.
func colorRun(pix []uint8, p Palette) { colorScalar(pix, p) }

func normalRun(dst, src []uint8, w, h int) { normalScalar(dst, src, w, h) }

// rgbKeepMask / alphaKeepMask are 32-byte vector masks selecting the R,G,B lanes
// and the alpha lanes of an 8-pixel block, used for bitwise (AVX2) lane
// selection without AVX-512 mask registers.
func rgbAlphaMasks() (rgb, alpha archsimd.Uint8x32) {
	var r, a [32]uint8
	for k := 0; k < 32; k++ {
		if k%4 == 3 {
			a[k] = 0xFF
		} else {
			r[k] = 0xFF
		}
	}
	return archsimd.LoadUint8x32(&r), archsimd.LoadUint8x32(&a)
}

// fillSIMD sets every pixel to (gray,gray,gray,255) via a broadcast store of a
// 32-byte RGBA pattern.
func fillSIMD(pix []uint8, gray uint8) {
	var patArr [32]uint8
	for k := 0; k < 8; k++ {
		patArr[k*4], patArr[k*4+1], patArr[k*4+2], patArr[k*4+3] = gray, gray, gray, 255
	}
	pat := archsimd.LoadUint8x32(&patArr)
	i := 0
	for ; i+32 <= len(pix); i += 32 {
		pat.StoreSlice(pix[i : i+32])
	}
	for ; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = gray, gray, gray, 255
	}
}

// invertSIMD inverts the RGB channels (255-v) leaving alpha unchanged, selecting
// lanes with bitwise AND/OR against vector masks (AVX2, no mask registers).
func invertSIMD(pix []uint8) {
	rgbMask, alphaMask := rgbAlphaMasks()
	i := 0
	for ; i+32 <= len(pix); i += 32 {
		v := archsimd.LoadUint8x32Slice(pix[i : i+32])
		// RGB = (^v) & rgbMask ; A = v & alphaMask ; OR the two together.
		v.Not().And(rgbMask).Or(v.And(alphaMask)).StoreSlice(pix[i : i+32])
	}
	for ; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2] = 255-pix[i], 255-pix[i+1], 255-pix[i+2]
	}
}

// blendSourceOverSIMD composites a constant grayscale source (g,g,g with alpha
// sa) onto n consecutive pixels at byte offset di using the source-over rule, in
// integer fixed-point (Q8) uint16 lanes with no float conversion. It is the
// integer counterpart of the fill/invert wins.
//
// On an opaque backdrop source-over reduces, per byte, to the affine map
//
//	out = to8(g*sa + (pix/255)*(1-sa)) = round(A + B*pix)
//
// with A = g*255*sa and B = 1-sa constant for the whole run and pix the backdrop
// byte in [0,255]. Working at Q8, out = (Bf*pix + Af) >> 8 with Bf = round(B*256)
// and Af = round(A*256) + 128 (the +128 is the >>8 rounding bias).
//
// Each step widens 2 pixels (8 bytes) to uint16 lanes, applies the affine map,
// then narrows back to bytes entirely in-register: AVX2 has no uint16->uint8 pack
// in this API (the VPMOV* narrowing is AVX-512-only, which the kernels avoid), so
// the low byte of each lane is gathered with a VPSHUFB byte shuffle
// (PermuteOrZero) and written with a single 8-byte store -- no scalar per-byte
// write-back (that would stall on store-forwarding, as the float blendRunSIMD
// does). The alpha lanes carry Bf=0, Af=255<<8, so an opaque backdrop stays
// opaque with no separate masking step.
//
// The affine form assumes ab==1, so only 2-pixel chunks whose backdrop is fully
// opaque take the vector path; any non-opaque chunk and the run tail fall back to
// the exact scalar kernel. Results match the scalar reference within +/-1 per
// channel (Q8 quantisation vs the reference float64).
//
// This kernel is a per-pixel win over the scalar compositor on a single wide run
// (see BenchmarkSourceOverAB / BenchmarkSourceOverL2AB) but is NOT on the
// production path: the real draw loop issues many short runs and the per-call
// vector setup is not amortised (see blendRun). It documents the integer approach
// and is validated by the tolerance test.
func blendSourceOverSIMD(pix []uint8, di, n int, g, sa float64) {
	if n <= 0 || sa <= 0 {
		return
	}
	// Patterned Q8 coefficients for 2 pixels (8 lanes). af/bf are non-negative, so
	// uint16(x + 0.5) is round-to-nearest. Bounds: bf <= 256, af+128 <= 65408, and
	// the per-lane sum Bf*pix + (Af+128) <= 65535 except in the single degenerate
	// case A+B*pix == 255, which AddSaturated caps at 65535 (>>8 = 255, correct).
	af := uint16(g*255*sa*256+0.5) + 128
	bf := uint16((1-sa)*256 + 0.5)
	var aArr, bArr [8]uint16
	for k := 0; k < 8; k++ {
		if k%4 == 3 { // alpha lane: keep an opaque backdrop opaque
			aArr[k], bArr[k] = 255<<8, 0
		} else {
			aArr[k], bArr[k] = af, bf
		}
	}
	afV := archsimd.LoadUint16x8(&aArr)
	bfV := archsimd.LoadUint16x8(&bArr)
	// VPSHUFB index: gather the low byte of each of the 8 uint16 lanes (even byte
	// positions of the reinterpreted vector) into the low 8 output bytes; a set
	// sign bit (-128) zeroes the unused high 8 bytes.
	idxArr := [16]int8{0, 2, 4, 6, 8, 10, 12, 14, -128, -128, -128, -128, -128, -128, -128, -128}
	idx := archsimd.LoadInt8x16(&idxArr)

	p := di
	rem := n
	// A full 16-byte load is cheaper than an 8-byte partial load, so we only take
	// the vector path while a whole 16 bytes remain; the low 8 bytes (2 pixels)
	// are used and the rest is re-read on the next step.
	for rem >= 2 && p+16 <= len(pix) {
		// The 8-lane vector spans 2 pixels; both backdrops must be opaque for the
		// ab==1 affine form to hold, else use the exact scalar kernel.
		if pix[p+3] == 255 && pix[p+7] == 255 {
			in := archsimd.LoadUint8x16Slice(pix[p : p+16]).ExtendLo8ToUint16()
			// out = (Bf*pix + Af) >> 8; the result is in the low byte of each lane.
			r := bfV.Mul(in).AddSaturated(afV).ShiftAllRight(8)
			r.AsUint8x16().PermuteOrZero(idx).StoreSlicePart(pix[p : p+8])
		} else {
			blendRunScalar(pix, p, 2, g, sa, ModeSourceOver)
		}
		p += 8
		rem -= 2
	}
	if rem > 0 {
		blendRunScalar(pix, p, rem, g, sa, ModeSourceOver)
	}
}

// simdBlendSupported reports whether blendRunSIMD vectorises a mode. The
// non-separable / special Porter-Duff modes fall back to the scalar kernel.
func simdBlendSupported(mode CompositionMode) bool {
	switch mode {
	case ModeSourceAtop, ModeXor, ModeLighter, ModeLuminosity:
		return false
	}
	return true
}

// blendRunSIMD composites a constant grayscale source (g,g,g, alpha sa) onto n
// consecutive pixels at byte offset di, vectorising 2 pixels (8 float32 lanes)
// at a time. See the file header for the correctness contract.
func blendRunSIMD(pix []uint8, di, n int, g, sa float64, mode CompositionMode) {
	if n <= 0 {
		return
	}
	if !simdBlendSupported(mode) || sa <= 0 {
		blendRunScalar(pix, di, n, g, sa, mode)
		return
	}

	g01 := float32(g)
	// Constants hoisted out of the pixel loop (and out of sepBlendSIMD). They
	// are kept as individual values, not a struct, per the archsimd guidance to
	// avoid placing vector types in aggregates.
	oneV := archsimd.BroadcastFloat32x8(1)
	twoV := archsimd.BroadcastFloat32x8(2)
	csV := archsimd.BroadcastFloat32x8(g01)
	saV := archsimd.BroadcastFloat32x8(float32(sa))
	invSaV := archsimd.BroadcastFloat32x8(float32(1 - sa))
	inv255 := archsimd.BroadcastFloat32x8(1.0 / 255)
	f255 := archsimd.BroadcastFloat32x8(255)
	halfV := archsimd.BroadcastFloat32x8(0.5)
	zeroV := archsimd.BroadcastFloat32x8(0)

	var tmp [8]int32
	p := di
	rem := n
	// A full 16-byte load is much faster than an 8-byte partial load, so we only
	// take the vector path while a whole 16 bytes remain in the buffer; the low 8
	// bytes (2 pixels) are used, the rest is re-read on the next step.
	for rem >= 2 && p+16 <= len(pix) {
		// The 8-lane vector spans 2 pixels; both backdrops must be opaque for
		// the ab==1 formula to hold, else fall back to the exact scalar kernel.
		if pix[p+3] == 255 && pix[p+7] == 255 {
			inF := archsimd.LoadUint8x16Slice(pix[p : p+16]).
				ExtendLo8ToUint32().AsInt32x8().ConvertToFloat32()
			in01 := inF.Mul(inv255)
			bF := sepBlendSIMD(in01, g01, mode, oneV, twoV, csV)
			// opaque backdrop, default Fa/Fb: out01 = sa*B + (1-sa)*in01.
			out01 := saV.Mul(bF).Add(invSaV.Mul(in01))
			// to8: floor(out01*255 + 0.5), clamped to [0,255].
			out01.Mul(f255).Add(halfV).Floor().Max(zeroV).Min(f255).ConvertToInt32().Store(&tmp)
			// Write RGB lanes; leave the alpha bytes (lanes 3,7) at their opaque
			// value so no alpha-restore step is needed.
			pix[p], pix[p+1], pix[p+2] = uint8(tmp[0]), uint8(tmp[1]), uint8(tmp[2])
			pix[p+4], pix[p+5], pix[p+6] = uint8(tmp[4]), uint8(tmp[5]), uint8(tmp[6])
		} else {
			blendRunScalar(pix, p, 2, g, sa, mode)
		}
		p += 8
		rem -= 2
	}
	if rem > 0 {
		blendRunScalar(pix, p, rem, g, sa, mode)
	}
}

// sepBlendSIMD applies the separable blend function B(cb, cs) per lane, matching
// sepBlend (compose.go) on [0,1]. cs is the constant grayscale source (scalar),
// so branches that depend only on cs are resolved in Go, and only cb-dependent
// branches use lane masks (AVX2 blend, via Float32x8.Merge). one/two/csV are the
// caller's hoisted constants (1, 2, and broadcast cs).
func sepBlendSIMD(cb archsimd.Float32x8, cs float32, mode CompositionMode, one, two, csV archsimd.Float32x8) archsimd.Float32x8 {
	switch mode {
	case ModeMultiply:
		return cb.Mul(csV)
	case ModeScreen:
		return cb.Add(csV).Sub(cb.Mul(csV))
	case ModeOverlay:
		// cb<=0.5 ? 2*cs*cb : 1-2*(1-cs)*(1-cb)
		lo := two.Mul(csV).Mul(cb)
		hi := one.Sub(two.Mul(one.Sub(csV)).Mul(one.Sub(cb)))
		return lo.Merge(hi, cb.LessEqual(archsimd.BroadcastFloat32x8(0.5)))
	case ModeDarken:
		return cb.Min(csV)
	case ModeLighten:
		return cb.Max(csV)
	case ModeColorDodge:
		// scalar: cb==0 ? 0 : cs==1 ? 1 : min(1, cb/(1-cs)). The cb==0 check
		// wins over cs==1, so for cs>=1 the result is 0 where cb==0 else 1.
		if cs >= 1 {
			zero := archsimd.BroadcastFloat32x8(0)
			return one.Merge(zero, cb.Greater(zero)) // cb>0 -> 1, cb==0 -> 0
		}
		return one.Min(cb.Div(archsimd.BroadcastFloat32x8(1 - cs)))
	case ModeColorBurn:
		// scalar: cb==1 ? 1 : cs==0 ? 0 : 1-min(1,(1-cb)/cs). The cb==1 check
		// wins over cs==0, so for cs<=0 the result is 1 where cb==1 else 0.
		if cs <= 0 {
			zero := archsimd.BroadcastFloat32x8(0)
			return one.Merge(zero, cb.GreaterEqual(one)) // cb>=1 -> 1, else 0
		}
		return one.Sub(one.Min(one.Sub(cb).Div(csV)))
	case ModeHardLight:
		// cs scalar selects the branch for the whole vector.
		if cs <= 0.5 {
			return two.Mul(csV).Mul(cb)
		}
		return one.Sub(two.Mul(one.Sub(csV)).Mul(one.Sub(cb)))
	case ModeSoftLight:
		if cs <= 0.5 {
			k := archsimd.BroadcastFloat32x8(1 - 2*cs)
			return cb.Sub(k.Mul(cb).Mul(one.Sub(cb)))
		}
		// D(cb) = cb<=0.25 ? ((16cb-12)cb+4)cb : sqrt(cb)
		poly := cb.Mul(archsimd.BroadcastFloat32x8(16)).Sub(archsimd.BroadcastFloat32x8(12)).
			Mul(cb).Add(archsimd.BroadcastFloat32x8(4)).Mul(cb)
		d := poly.Merge(cb.Sqrt(), cb.LessEqual(archsimd.BroadcastFloat32x8(0.25)))
		return cb.Add(archsimd.BroadcastFloat32x8(2*cs - 1).Mul(d.Sub(cb)))
	case ModeDifference:
		return cb.Sub(csV).Max(csV.Sub(cb))
	case ModeExclusion:
		return cb.Add(csV).Sub(two.Mul(cb).Mul(csV))
	default:
		// source-over and every Porter-Duff normal: B(cb,cs) = cs.
		return csV
	}
}
