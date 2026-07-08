//go:build simd

package gen

import (
	"fmt"
	"testing"
)

// These tests run only under the SIMD build (-tags simd, GOEXPERIMENT=simd) and
// assert that the SIMD kernels agree with the scalar reference. Blend results
// use float32 lanes and are allowed to differ by at most 1 per channel; fill and
// invert must be bit-exact.

const blendTolerance = 1

// makeBackdrop builds a straight-RGBA buffer of n pixels with a varied RGB
// gradient. If opaque is true every pixel is opaque; otherwise alpha varies so
// the SIMD kernel's non-opaque scalar fallback is exercised.
func makeBackdrop(n int, opaque bool) []uint8 {
	pix := make([]uint8, n*4)
	den := n - 1
	if den < 1 {
		den = 1
	}
	for i := 0; i < n; i++ {
		// Linear ramps so cb spans the full range including the 0 and 255
		// endpoints that trigger the color-dodge/color-burn edge cases.
		pix[i*4] = uint8(i * 255 / den)
		pix[i*4+1] = uint8((den - i) * 255 / den)
		pix[i*4+2] = uint8((i * 29) % 256)
		if opaque {
			pix[i*4+3] = 255
		} else {
			pix[i*4+3] = uint8((i*37)%254 + 1) // 1..254, never 0/255 mixed with some 255
			if i%3 == 0 {
				pix[i*4+3] = 255
			}
		}
	}
	return pix
}

func maxDiff(a, b []uint8) (idx, diff int) {
	for i := range a {
		d := int(a[i]) - int(b[i])
		if d < 0 {
			d = -d
		}
		if d > diff {
			diff, idx = d, i
		}
	}
	return idx, diff
}

func TestBlendRunSIMDMatchesScalar(t *testing.T) {
	// 37 pixels: not a multiple of 2, so the SIMD tail path is exercised.
	const n = 37
	grays := []uint8{0, 1, 64, 128, 191, 254, 255}
	alphas := []int{1, 25, 50, 75, 100}

	for _, opaque := range []bool{true, false} {
		for _, mode := range AllCompositionModes() {
			for _, gray := range grays {
				for _, aPct := range alphas {
					g := float64(gray) / 255
					sa := float64(aPct) / 100

					want := makeBackdrop(n, opaque)
					got := makeBackdrop(n, opaque)
					blendRunScalar(want, 0, n, g, sa, mode)
					blendRunSIMD(got, 0, n, g, sa, mode)

					if idx, d := maxDiff(want, got); d > blendTolerance {
						t.Errorf("mode=%s gray=%d alpha=%d opaque=%v: max byte diff %d at %d (scalar=%d simd=%d)",
							mode, gray, aPct, opaque, d, idx, want[idx], got[idx])
					}
				}
			}
		}
	}
}

func TestFillSIMDMatchesScalar(t *testing.T) {
	for _, n := range []int{0, 1, 7, 8, 9, 100, 257} {
		for _, gray := range []uint8{0, 17, 128, 255} {
			want := make([]uint8, n*4)
			got := make([]uint8, n*4)
			fillScalar(want, gray)
			fillSIMD(got, gray)
			if idx, d := maxDiff(want, got); d != 0 {
				t.Fatalf("n=%d gray=%d: fill mismatch at %d (scalar=%d simd=%d)", n, gray, idx, want[idx], got[idx])
			}
		}
	}
}

func TestInvertSIMDMatchesScalar(t *testing.T) {
	for _, n := range []int{0, 1, 7, 8, 9, 100, 257} {
		want := makeBackdrop(n, false)
		got := make([]uint8, len(want))
		copy(got, want)
		invertScalar(want)
		invertSIMD(got)
		if idx, d := maxDiff(want, got); d != 0 {
			t.Fatalf("n=%d: invert mismatch at %d (scalar=%d simd=%d)", n, idx, want[idx], got[idx])
		}
	}
}

// TestBuildVariantIsSIMD documents that this test binary is the SIMD variant.
func TestBuildVariantIsSIMD(t *testing.T) {
	if BuildVariant != "simd" {
		t.Fatalf("BuildVariant = %q, want simd (are you building with -tags simd?)", BuildVariant)
	}
	fmt.Println("SIMD build variant under test:", BuildVariant)
}
