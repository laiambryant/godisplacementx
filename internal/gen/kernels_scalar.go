package gen

// Scalar batch kernels. These operate on a run of pixels at once and are the
// reference implementation: they are always compiled (no build tag) and are
// used directly by the default build, as the fallback of the SIMD build, and as
// the baseline in the SIMD-vs-scalar benchmarks.
//
// The public methods on Canvas and the post-processing functions call the
// dispatch wrappers (blendRun, fillRun, invertRun, colorRun, normalRun), which
// resolve to these functions in the default build and to their SIMD
// counterparts under the "simd" build tag on amd64.

// fillScalar sets every pixel to an opaque grayscale value (R=G=B=gray, A=255).
func fillScalar(pix []uint8, gray uint8) {
	for i := 0; i < len(pix); i += 4 {
		pix[i] = gray
		pix[i+1] = gray
		pix[i+2] = gray
		pix[i+3] = 255
	}
}

// blendRunScalar composites a constant grayscale source (g,g,g in [0,1], alpha
// sa in [0,1]) onto n consecutive destination pixels starting at byte offset di,
// using the given composition mode. It mirrors a run of blendInto calls with a
// grayscale source, which is exactly what FillRect needs.
func blendRunScalar(pix []uint8, di, n int, g, sa float64, mode CompositionMode) {
	for k := 0; k < n; k++ {
		blendInto(pix, di+k*4, g, g, g, sa, mode)
	}
}

// invertScalar inverts the RGB channels (255 - v), leaving alpha unchanged.
func invertScalar(pix []uint8) {
	for i := 0; i < len(pix); i += 4 {
		pix[i] = 255 - pix[i]
		pix[i+1] = 255 - pix[i+1]
		pix[i+2] = 255 - pix[i+2]
	}
}

// colorScalar remaps each channel through the gradient palette, leaving alpha
// unchanged (the source is grayscale, so R=G=B index the same colour).
func colorScalar(pix []uint8, p Palette) {
	for i := 0; i < len(pix); i += 4 {
		pix[i] = p.R[pix[i]]
		pix[i+1] = p.G[pix[i+1]]
		pix[i+2] = p.B[pix[i+2]]
	}
}

// normalScalar converts the height field in src into an OpenGL-style normal map
// via a Sobel-like difference of neighbours, writing into dst. src must be a
// snapshot of the original buffer (dst and src may not alias). w, h are the
// canvas dimensions; the row stride is w*4.
func normalScalar(dst, src []uint8, w, h int) {
	stride := w * 4
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

		dst[i] = clampByte(x1 - x2 + 127)
		dst[i+1] = clampByte(y1 - y2 + 127)
		dst[i+2] = 255
		dst[i+3] = 255
	}
}
