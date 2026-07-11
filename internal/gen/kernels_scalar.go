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
// It seeds one pixel and then replicates the pattern with exponentially growing
// copy() calls, which lower to the runtime's vectorised memmove and run several
// times faster than a per-byte store loop while staying pure Go.
func fillScalar(pix []uint8, gray uint8) {
	if len(pix) < 4 {
		return
	}
	pix[0], pix[1], pix[2], pix[3] = gray, gray, gray, 255
	for filled := 4; filled < len(pix); filled *= 2 {
		copy(pix[filled:], pix[:filled])
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

// normalScalar converts rows [y0, y1) of the height field in src into
// OpenGL-style normal-map pixels in dst via a Sobel-like difference of
// neighbours. src must be a snapshot of the original buffer (dst and src may
// not alias); w, h are the full canvas dimensions, so bands can be computed
// concurrently (each band only writes its own rows and reads src, which is
// immutable during the pass). Edge pixels clamp to themselves, exactly as the
// original per-pixel modulo formulation did.
func normalScalar(dst, src []uint8, w, h, y0, y1 int) {
	stride := w * 4
	for y := y0; y < y1; y++ {
		row := y * stride
		up, down := row-stride, row+stride
		if y == 0 {
			up = row
		}
		if y == h-1 {
			down = row
		}
		for x := 0; x < w; x++ {
			i := row + x*4
			left, right := i-4, i+4
			if x == 0 {
				left = i
			}
			if x == w-1 {
				right = i
			}
			dst[i] = clampByte(int(src[left]) - int(src[right]) + 127)
			dst[i+1] = clampByte(int(src[up+x*4]) - int(src[down+x*4]) + 127)
			dst[i+2] = 255
			dst[i+3] = 255
		}
	}
}
