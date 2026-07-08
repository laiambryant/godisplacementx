package gen

import (
	"fmt"
	"testing"
)

// Benchmarks that compile in every build. They exercise the dispatch wrappers
// and the end-to-end pipeline, so the same benchmark measures the scalar kernels
// in the default build and the SIMD kernels under `-tags simd`. Run both and
// diff with benchstat (see the Makefile `bench-compare` target) for the
// two-build comparison; the SIMD build additionally has the single-binary A/B
// benchmarks in kernels_simd_bench_test.go.

// benchResolutions are square canvas edge lengths (pixels) for the micro
// kernels, spanning the GUI's resolution range.
var benchResolutions = []int{1024, 2048, 4096}

// benchModes is a light / medium / heavy sample of composition modes.
var benchModes = []CompositionMode{ModeSourceOver, ModeMultiply, ModeSoftLight}

// opaqueBuf builds a res*res opaque RGBA buffer with varied channel values.
func opaqueBuf(res int) []uint8 {
	pix := make([]uint8, res*res*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i] = uint8(i % 256)
		pix[i+1] = uint8((i / 4) % 256)
		pix[i+2] = uint8((i / 16) % 256)
		pix[i+3] = 255
	}
	return pix
}

func BenchmarkFill(b *testing.B) {
	for _, res := range benchResolutions {
		b.Run(fmt.Sprintf("%d", res), func(b *testing.B) {
			pix := make([]uint8, res*res*4)
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				fillRun(pix, 128)
			}
		})
	}
}

func BenchmarkInvert(b *testing.B) {
	for _, res := range benchResolutions {
		b.Run(fmt.Sprintf("%d", res), func(b *testing.B) {
			pix := opaqueBuf(res)
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				invertRun(pix)
			}
		})
	}
}

func BenchmarkColor(b *testing.B) {
	pal := BuildPalette(DefaultGradient())
	for _, res := range benchResolutions {
		b.Run(fmt.Sprintf("%d", res), func(b *testing.B) {
			pix := opaqueBuf(res)
			b.SetBytes(int64(len(pix)))
			for b.Loop() {
				colorRun(pix, pal)
			}
		})
	}
}

func BenchmarkBlend(b *testing.B) {
	for _, mode := range benchModes {
		for _, res := range benchResolutions {
			b.Run(fmt.Sprintf("%s/%d", mode, res), func(b *testing.B) {
				pix := opaqueBuf(res)
				n := res * res
				b.SetBytes(int64(len(pix)))
				for b.Loop() {
					blendRun(pix, 0, n, 0.5, 0.5, mode)
				}
			})
		}
	}
}

// BenchmarkGenerate measures the full generation pipeline (Fill + the primitive
// draw loop, which is dominated by blendRun) at a fixed seed.
func BenchmarkGenerate(b *testing.B) {
	p := Default()
	for _, res := range []int{512, 1024, 2048} {
		b.Run(fmt.Sprintf("%d", res), func(b *testing.B) {
			for b.Loop() {
				g := NewRNG(12345)
				if _, err := Generate(p, res, res, g); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
