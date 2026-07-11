//go:build simd && !amd64

package gen

// A SIMD-tagged build for a non-amd64 target (e.g. the arm64 release binaries):
// simd/archsimd is amd64-only, so the SIMD symbols alias the scalar kernels.
// This keeps `-tags simd` compiling for every GOARCH with a correct scalar
// fallback, and lets the SIMD benchmarks/tests reference the *SIMD names on any
// platform.

func fillRun(pix []uint8, gray uint8) { fillScalar(pix, gray) }

func blendRun(pix []uint8, di, n int, g, sa float64, mode CompositionMode) {
	blendRunScalar(pix, di, n, g, sa, mode)
}

func invertRun(pix []uint8) { invertScalar(pix) }

func colorRun(pix []uint8, p Palette) { colorScalar(pix, p) }

func normalRun(dst, src []uint8, w, h, y0, y1 int) { normalScalar(dst, src, w, h, y0, y1) }

func fillSIMD(pix []uint8, gray uint8) { fillScalar(pix, gray) }

func blendRunSIMD(pix []uint8, di, n int, g, sa float64, mode CompositionMode) {
	blendRunScalar(pix, di, n, g, sa, mode)
}

func blendSourceOverSIMD(pix []uint8, di, n int, g, sa float64) {
	blendRunScalar(pix, di, n, g, sa, ModeSourceOver)
}

func invertSIMD(pix []uint8) { invertScalar(pix) }
