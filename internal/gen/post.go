package gen

// ApplyInvert inverts the RGB channels (255 - v), leaving alpha unchanged.
// Port of drawInvert.ts.
func ApplyInvert(c *Canvas) {
	for i := 0; i < len(c.Pix); i += 4 {
		c.Pix[i] = 255 - c.Pix[i]
		c.Pix[i+1] = 255 - c.Pix[i+1]
		c.Pix[i+2] = 255 - c.Pix[i+2]
	}
}

// ApplyColor remaps each channel through the gradient palette, leaving alpha
// unchanged. Port of drawColor.ts (the source is grayscale, so R=G=B index the
// same colour).
func ApplyColor(c *Canvas, p Palette) {
	for i := 0; i < len(c.Pix); i += 4 {
		c.Pix[i] = p.R[c.Pix[i]]
		c.Pix[i+1] = p.G[c.Pix[i+1]]
		c.Pix[i+2] = p.B[c.Pix[i+2]]
	}
}

// ApplyNormal converts the height field into an OpenGL-style normal map via a
// Sobel-like difference of neighbours. Port of drawNormal.ts. The original
// assumes a square canvas; for a square canvas the row stride (w*4) equals the
// original's h*4, so results match. Out-of-range values are clamped to 0-255
// (matching the Uint8ClampedArray semantics of ImageData).
func ApplyNormal(c *Canvas) {
	w, h := c.W, c.H
	stride := w * 4
	src := make([]uint8, len(c.Pix))
	copy(src, c.Pix)

	for i := 0; i < w*h*4; i += 4 {
		var x1, x2, y1, y2 int

		col := i % stride
		switch {
		case col == 0: // left edge
			x1 = int(src[i])
			x2 = int(src[i+4])
		case col == (w-1)*4: // right edge
			x1 = int(src[i-4])
			x2 = int(src[i])
		default:
			x1 = int(src[i-4])
			x2 = int(src[i+4])
		}

		switch {
		case i < stride: // top edge
			y1 = int(src[i])
			y2 = int(src[i+stride])
		case i >= (h-1)*stride: // bottom edge
			y1 = int(src[i-stride])
			y2 = int(src[i])
		default:
			y1 = int(src[i-stride])
			y2 = int(src[i+stride])
		}

		c.Pix[i] = clampByte(x1 - x2 + 127)
		c.Pix[i+1] = clampByte(y1 - y2 + 127)
		c.Pix[i+2] = 255
		c.Pix[i+3] = 255
	}
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
