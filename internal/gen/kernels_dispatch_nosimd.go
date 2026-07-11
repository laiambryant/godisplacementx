//go:build !simd

package gen

// Default (pure-Go) build: the batch-kernel dispatch wrappers resolve directly
// to the scalar implementations. This file carries no SIMD dependency, so the
// default build stays portable and cross-compiles to every GOARCH exactly as
// before. The SIMD build (-tags simd) provides its own wrappers.

func fillRun(pix []uint8, gray uint8) { fillScalar(pix, gray) }

func blendRun(pix []uint8, di, n int, g, sa float64, mode CompositionMode) {
	blendRunScalar(pix, di, n, g, sa, mode)
}

func invertRun(pix []uint8) { invertScalar(pix) }

func colorRun(pix []uint8, p Palette) { colorScalar(pix, p) }

func normalRun(dst, src []uint8, w, h, y0, y1 int) { normalScalar(dst, src, w, h, y0, y1) }
