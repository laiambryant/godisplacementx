package gen

import (
	"fmt"
	"testing"
)

// variedBackdrop builds a w*h straight-RGBA canvas with per-channel ramps. When
// opaque is false, roughly a third of the pixels are left translucent so the LUT
// path's non-opaque scalar fallback is exercised.
func variedBackdrop(w, h int, opaque bool) *Canvas {
	c := NewCanvas(w, h)
	n := w * h
	for i := 0; i < n; i++ {
		c.Pix[i*4] = uint8(i % 256)
		c.Pix[i*4+1] = uint8((i * 7) % 256)
		c.Pix[i*4+2] = uint8((i * 13) % 256)
		a := uint8(255)
		if !opaque && i%3 == 0 {
			a = uint8((i*37)%254 + 1) // 1..254
		}
		c.Pix[i*4+3] = a
	}
	return c
}

// TestFillRectLUTMatchesScalar asserts that FillRect (which takes the LUT fast
// path for a full-canvas rectangle) is byte-identical to the reference scalar
// compositor for every mode, over both opaque and translucent backdrops.
func TestFillRectLUTMatchesScalar(t *testing.T) {
	const w, h = 40, 40 // area 1600 >= blendLUTMinArea, so the LUT path is taken
	grays := []uint8{0, 1, 64, 128, 191, 254, 255}
	alphas := []int{1, 25, 50, 75, 100}

	for _, opaque := range []bool{true, false} {
		for _, mode := range AllCompositionModes() {
			for _, gray := range grays {
				for _, aPct := range alphas {
					got := variedBackdrop(w, h, opaque)
					want := variedBackdrop(w, h, opaque)

					// FillRect over the whole canvas takes the LUT path where eligible.
					got.FillRect(0, 0, w, h, gray, aPct, mode)
					// Reference: a full-buffer scalar run (contiguous, so equivalent to
					// FillRect's per-row runs since blendInto is per-pixel independent).
					blendRunScalar(want.Pix, 0, w*h, float64(gray)/255, float64(aPct)/100, mode)

					for i := range got.Pix {
						if got.Pix[i] != want.Pix[i] {
							t.Fatalf("mode=%s gray=%d alpha=%d opaque=%v: byte %d got=%d want=%d",
								mode, gray, aPct, opaque, i, got.Pix[i], want.Pix[i])
						}
					}
				}
			}
		}
	}
}

// BenchmarkFillRect measures the end-to-end FillRect compositor (LUT fast path
// where eligible) across a light/medium/heavy sample of modes and resolutions.
func BenchmarkFillRect(b *testing.B) {
	for _, mode := range benchModes {
		for _, res := range benchResolutions {
			b.Run(fmt.Sprintf("%s/%d", mode, res), func(b *testing.B) {
				c := &Canvas{W: res, H: res, Pix: opaqueBuf(res)}
				b.SetBytes(int64(len(c.Pix)))
				for b.Loop() {
					c.FillRect(0, 0, res, res, 128, 50, mode)
				}
			})
		}
	}
}
