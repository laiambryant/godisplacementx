//go:build simd

package gen

import (
	"fmt"
	"testing"
)

// Single-binary A/B benchmarks: available only under the SIMD build, they call
// the scalar reference and the SIMD kernel directly in the same run so scalar vs
// simd can be compared side by side (e.g. `go test -tags simd -bench AB`).

func BenchmarkFillAB(b *testing.B) {
	for _, res := range benchResolutions {
		pix := make([]uint8, res*res*4)
		b.Run(fmt.Sprintf("scalar/%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				fillScalar(pix, 128)
			}
		})
		b.Run(fmt.Sprintf("simd/%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				fillSIMD(pix, 128)
			}
		})
	}
}

func BenchmarkInvertAB(b *testing.B) {
	for _, res := range benchResolutions {
		pix := opaqueBuf(res)
		b.Run(fmt.Sprintf("scalar/%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				invertScalar(pix)
			}
		})
		b.Run(fmt.Sprintf("simd/%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				invertSIMD(pix)
			}
		})
	}
}

// BenchmarkSourceOverAB compares the scalar source-over compositor against the
// dedicated integer fixed-point kernel that production actually dispatches to.
func BenchmarkSourceOverAB(b *testing.B) {
	for _, res := range benchResolutions {
		n := res * res
		pix := opaqueBuf(res)
		b.Run(fmt.Sprintf("scalar/%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				blendRunScalar(pix, 0, n, 0.5, 0.5, ModeSourceOver)
			}
		})
		b.Run(fmt.Sprintf("simd/%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				blendSourceOverSIMD(pix, 0, n, 0.5, 0.5)
			}
		})
	}
}

// BenchmarkSourceOverL2AB measures the compute-bound speed of the source-over
// kernels on a cache-resident buffer (256x256 = 256 KiB, fits L2), isolating the
// per-pixel work from main-memory bandwidth and sustained-AVX2 power throttling.
func BenchmarkSourceOverL2AB(b *testing.B) {
	const res = 256
	n := res * res
	pix := opaqueBuf(res)
	b.Run("scalar", func(b *testing.B) {
		b.SetBytes(int64(len(pix)))
		for b.Loop() {
			blendRunScalar(pix, 0, n, 0.5, 0.5, ModeSourceOver)
		}
	})
	b.Run("simd", func(b *testing.B) {
		b.SetBytes(int64(len(pix)))
		for b.Loop() {
			blendSourceOverSIMD(pix, 0, n, 0.5, 0.5)
		}
	})
}

func BenchmarkBlendAB(b *testing.B) {
	for _, mode := range benchModes {
		for _, res := range benchResolutions {
			n := res * res
			pix := opaqueBuf(res)
			b.Run(fmt.Sprintf("scalar/%s/%d", mode, res), func(b *testing.B) {
				b.SetBytes(int64(len(pix)))
				for b.Loop() {
					blendRunScalar(pix, 0, n, 0.5, 0.5, mode)
				}
			})
			b.Run(fmt.Sprintf("simd/%s/%d", mode, res), func(b *testing.B) {
				b.SetBytes(int64(len(pix)))
				for b.Loop() {
					blendRunSIMD(pix, 0, n, 0.5, 0.5, mode)
				}
			})
		}
	}
}
