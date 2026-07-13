package gen

import "image"

// Fast-mode sprite scaler: same tap layout and fused rotation as
// scaleRotateNRGBA, but the float64 tap weights become Q8 fixed point and the
// two lerp stages stay in integers. Each stage rounds once, so results sit
// within a couple of units of the exact scaler in premultiplied space (the
// differential test compares there — unpremultiplying amplifies any error at
// low alpha, which is why the straight-alpha bytes of nearly transparent
// pixels may differ more while remaining visually identical).

// fixedTap is bilinearTap with Q8 weights (w0 + w1 == 256).
type fixedTap struct {
	s0, s1 int
	w0, w1 uint32
}

func fixedTaps(dstN, srcN int) []fixedTap {
	taps := bilinearTaps(dstN, srcN)
	fixed := make([]fixedTap, len(taps))
	for i, t := range taps {
		w0 := uint32(t.f0*256 + 0.5)
		fixed[i] = fixedTap{s0: t.s0, s1: t.s1, w0: w0, w1: 256 - w0}
	}
	return fixed
}

// samplePremulFixed reads source pixel index si from row as premultiplied
// 16-bit integer channels.
func samplePremulFixed(row []uint8, si int) (r, g, b, a uint32) {
	o := si * 4
	al := &premul16[row[o+3]]
	return uint32(al[row[o]]), uint32(al[row[o+1]]), uint32(al[row[o+2]]), uint32(premul16[row[o+3]][0xff])
}

func lerpQ8(w1, v0, w0, v1 uint32) uint32 {
	return (w1*v0 + w0*v1 + 128) >> 8
}

// scaleRotateNRGBAFast scales a square straight-alpha sprite to n×n and
// rotates it clockwise by rot (a multiple of 90 degrees), fused, in fixed
// point. Only used in fast mode.
func scaleRotateNRGBAFast(src *image.NRGBA, n, rot int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	sw := src.Rect.Dx()
	sh := src.Rect.Dy()
	if sw <= 0 || sh <= 0 || n <= 0 {
		return dst
	}
	xTaps := fixedTaps(n, sw)
	yTaps := fixedTaps(n, sh)

	parallelBands(n, minRowsPerBand(n), func(y0, y1 int) {
		for dy := y0; dy < y1; dy++ {
			ty := &yTaps[dy]
			row0 := src.Pix[src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+ty.s0):]
			row1 := src.Pix[src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+ty.s1):]
			off, step := rotatedRowStart(rot, n, dy)
			for dx := 0; dx < n; dx++ {
				tx := &xTaps[dx]
				s00r, s00g, s00b, s00a := samplePremulFixed(row0, tx.s0)
				s10r, s10g, s10b, s10a := samplePremulFixed(row0, tx.s1)
				s01r, s01g, s01b, s01a := samplePremulFixed(row1, tx.s0)
				s11r, s11g, s11b, s11a := samplePremulFixed(row1, tx.s1)

				r := lerpQ8(ty.w1, lerpQ8(tx.w1, s00r, tx.w0, s10r), ty.w0, lerpQ8(tx.w1, s01r, tx.w0, s11r))
				g := lerpQ8(ty.w1, lerpQ8(tx.w1, s00g, tx.w0, s10g), ty.w0, lerpQ8(tx.w1, s01g, tx.w0, s11g))
				b := lerpQ8(ty.w1, lerpQ8(tx.w1, s00b, tx.w0, s10b), ty.w0, lerpQ8(tx.w1, s01b, tx.w0, s11b))
				a := lerpQ8(ty.w1, lerpQ8(tx.w1, s00a, tx.w0, s10a), ty.w0, lerpQ8(tx.w1, s01a, tx.w0, s11a))

				storeUnpremul(dst.Pix, off, r, g, b, a)
				off += step
			}
		}
	})
	return dst
}
