package gen

// ApplyInvert inverts the RGB channels (255 - v), leaving alpha unchanged.
// Port of drawInvert.ts.
func ApplyInvert(c *Canvas) {
	parallelBands(len(c.Pix)/4, parallelMinPixels, func(lo, hi int) {
		invertRun(c.Pix[lo*4 : hi*4])
	})
}

// ApplyColor remaps each channel through the gradient palette, leaving alpha
// unchanged. Port of drawColor.ts (the source is grayscale, so R=G=B index the
// same colour).
func ApplyColor(c *Canvas, p Palette) {
	parallelBands(len(c.Pix)/4, parallelMinPixels, func(lo, hi int) {
		colorRun(c.Pix[lo*4:hi*4], p)
	})
}

// ApplyNormal converts the height field into an OpenGL-style normal map via a
// Sobel-like difference of neighbours. Port of drawNormal.ts. The original
// assumes a square canvas; for a square canvas the row stride (w*4) equals the
// original's h*4, so results match. Out-of-range values are clamped to 0-255
// (matching the Uint8ClampedArray semantics of ImageData). Row bands run
// concurrently: each band writes only its own rows and reads the immutable
// snapshot, so the output is identical to a single full-frame pass.
func ApplyNormal(c *Canvas) {
	src := make([]uint8, len(c.Pix))
	copy(src, c.Pix)
	parallelBands(c.H, minRowsPerBand(c.W), func(y0, y1 int) {
		normalRun(c.Pix, src, c.W, c.H, y0, y1)
	})
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
