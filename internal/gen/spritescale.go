package gen

import "image"

// Specialised bilinear sprite scaling. x/image/draw's ApproxBiLinear has no
// fast path for an *image.NRGBA destination, so it falls back to a generic
// per-pixel path built on the image.Image / color.Color interfaces — boxing
// every sample — which dominated the whole Generate profile once sprites were
// enabled. scaleRotateNRGBA reproduces that path's arithmetic bit for bit
// (premultiplied 16-bit samples, float64 tap weights, nrgbaModel write-back;
// pinned by TestScaleRotateNRGBAMatchesXDraw), reads and writes pixel bytes
// directly, folds the 90-degree sprite rotation into the store offset so the
// separate rotation pass disappears, and splits destination rows across
// goroutines.

// premul16 tables color.NRGBA.RGBA: premul16[a][v] is the alpha-premultiplied
// 16-bit value of straight 8-bit channel v under alpha a.
var premul16 = func() (t [256][256]uint16) {
	for a := 0; a < 256; a++ {
		for v := 0; v < 256; v++ {
			w := uint32(v)
			w |= w << 8
			t[a][v] = uint16(w * uint32(a) / 0xff)
		}
	}
	return
}()

// bilinearTap is one destination coordinate's source taps along one axis:
// blend source indices s0 and s1 with weights f1 and f0 respectively.
type bilinearTap struct {
	s0, s1 int
	f0, f1 float64
}

// bilinearTaps computes the per-axis taps exactly as the x/image/draw
// ablInterpolator does (including the edge clamping).
func bilinearTaps(dstN, srcN int) []bilinearTap {
	scale := float64(srcN) / float64(dstN)
	last := srcN - 1
	taps := make([]bilinearTap, dstN)
	for d := range taps {
		s := (float64(d)+0.5)*scale - 0.5
		s0 := int(s)
		f0 := s - float64(s0)
		f1 := 1 - f0
		s1 := s0 + 1
		if s < 0 {
			s0, s1 = 0, 0
			f0, f1 = 0, 1
		} else if s1 > last {
			s0, s1 = last, last
			f0, f1 = 1, 0
		}
		taps[d] = bilinearTap{s0: s0, s1: s1, f0: f0, f1: f1}
	}
	return taps
}

// rotatedRowStart returns the byte offset of destination pixel (dx=0, dy) and
// the byte step per dx, for a clockwise rotation of the n×n scaled image by
// rot degrees (0/90/180/270).
func rotatedRowStart(rot, n, dy int) (start, step int) {
	switch rot {
	case 90:
		return (n - 1 - dy) * 4, 4 * n
	case 180:
		return ((n-1-dy)*n + n - 1) * 4, -4
	case 270:
		return ((n-1)*n + dy) * 4, -4 * n
	default:
		return dy * n * 4, 4
	}
}

// samplePremul reads source pixel index si (in pixels) from row as
// premultiplied 16-bit float64 channels.
func samplePremul(row []uint8, si int) (r, g, b, a float64) {
	o := si * 4
	al := &premul16[row[o+3]]
	return float64(al[row[o]]), float64(al[row[o+1]]), float64(al[row[o+2]]), float64(premul16[row[o+3]][0xff])
}

// storeUnpremul writes a premultiplied 16-bit pixel as straight NRGBA bytes,
// matching color.nrgbaModel exactly.
func storeUnpremul(pix []uint8, o int, r, g, b, a uint32) {
	switch {
	case a == 0xffff:
		pix[o] = uint8(r >> 8)
		pix[o+1] = uint8(g >> 8)
		pix[o+2] = uint8(b >> 8)
		pix[o+3] = 0xff
	case a == 0:
		pix[o] = 0
		pix[o+1] = 0
		pix[o+2] = 0
		pix[o+3] = 0
	default:
		pix[o] = uint8((r * 0xffff / a) >> 8)
		pix[o+1] = uint8((g * 0xffff / a) >> 8)
		pix[o+2] = uint8((b * 0xffff / a) >> 8)
		pix[o+3] = uint8(a >> 8)
	}
}

// scaleRotateNRGBA scales a square straight-alpha sprite to n×n and rotates it
// clockwise by rot (a multiple of 90 degrees) in a single fused pass.
func scaleRotateNRGBA(src *image.NRGBA, n, rot int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	sw := src.Rect.Dx()
	sh := src.Rect.Dy()
	if sw <= 0 || sh <= 0 || n <= 0 {
		return dst
	}
	xTaps := bilinearTaps(n, sw)
	yTaps := bilinearTaps(n, sh)

	parallelBands(n, minRowsPerBand(n), func(y0, y1 int) {
		for dy := y0; dy < y1; dy++ {
			ty := &yTaps[dy]
			row0 := src.Pix[src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+ty.s0):]
			row1 := src.Pix[src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+ty.s1):]
			off, step := rotatedRowStart(rot, n, dy)
			for dx := 0; dx < n; dx++ {
				tx := &xTaps[dx]
				s00r, s00g, s00b, s00a := samplePremul(row0, tx.s0)
				s10r, s10g, s10b, s10a := samplePremul(row0, tx.s1)
				s01r, s01g, s01b, s01a := samplePremul(row1, tx.s0)
				s11r, s11g, s11b, s11a := samplePremul(row1, tx.s1)

				s10r = tx.f1*s00r + tx.f0*s10r
				s10g = tx.f1*s00g + tx.f0*s10g
				s10b = tx.f1*s00b + tx.f0*s10b
				s10a = tx.f1*s00a + tx.f0*s10a

				s11r = tx.f1*s01r + tx.f0*s11r
				s11g = tx.f1*s01g + tx.f0*s11g
				s11b = tx.f1*s01b + tx.f0*s11b
				s11a = tx.f1*s01a + tx.f0*s11a

				s11r = ty.f1*s10r + ty.f0*s11r
				s11g = ty.f1*s10g + ty.f0*s11g
				s11b = ty.f1*s10b + ty.f0*s11b
				s11a = ty.f1*s10a + ty.f0*s11a

				storeUnpremul(dst.Pix, off, uint32(s11r), uint32(s11g), uint32(s11b), uint32(s11a))
				off += step
			}
		}
	})
	return dst
}
