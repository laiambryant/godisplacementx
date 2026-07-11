package gen

import (
	"image"
	"sync"
	"testing"
)

// TestParallelBandsCoversRangeExactlyOnce asserts the split is a disjoint
// cover of [0, n) for sizes around the banding thresholds.
func TestParallelBandsCoversRangeExactlyOnce(t *testing.T) {
	for _, n := range []int{0, 1, 7, parallelMinPixels - 1, parallelMinPixels, parallelMinPixels*3 + 17} {
		seen := make([]int, n)
		var mu sync.Mutex
		parallelBands(n, 3, func(lo, hi int) {
			mu.Lock()
			defer mu.Unlock()
			for i := lo; i < hi; i++ {
				seen[i]++
			}
		})
		for i, count := range seen {
			if count != 1 {
				t.Fatalf("n=%d: unit %d processed %d times, want 1", n, i, count)
			}
		}
	}
}

// TestFillRectParallelMatchesReference pins the banded FillRect against a
// serial per-row scalar reference on a canvas large enough to split across
// several goroutines, over opaque and translucent backdrops.
func TestFillRectParallelMatchesReference(t *testing.T) {
	const w, h = 1024, 512 // w*h >> parallelMinPixels, so rows split into bands
	modes := []CompositionMode{ModeSourceOver, ModeMultiply, ModeSoftLight, ModeLuminosity, ModeXor}

	for _, opaque := range []bool{true, false} {
		for _, mode := range modes {
			got := variedBackdrop(w, h, opaque)
			want := variedBackdrop(w, h, opaque)

			got.FillRect(-8, -8, w+16, h+16, 200, 60, mode)
			for y := range h {
				blendRunScalar(want.Pix, y*w*4, w, 200.0/255, 0.6, mode)
			}

			for i := range got.Pix {
				if got.Pix[i] != want.Pix[i] {
					t.Fatalf("mode=%s opaque=%v: byte %d got=%d want=%d", mode, opaque, i, got.Pix[i], want.Pix[i])
				}
			}
		}
	}
}

// drawImageReference is the original serial per-pixel DrawImage, kept as the
// oracle for the clipped, banded, fast-pathed implementation.
func drawImageReference(c *Canvas, src *image.NRGBA, dx, dy int, mode CompositionMode) {
	b := src.Bounds()
	for sy := 0; sy < b.Dy(); sy++ {
		ty := dy + sy
		if ty < 0 || ty >= c.H {
			continue
		}
		srow := src.PixOffset(b.Min.X, b.Min.Y+sy)
		for sx := 0; sx < b.Dx(); sx++ {
			tx := dx + sx
			if tx < 0 || tx >= c.W {
				continue
			}
			si := srow + sx*4
			sa := float64(src.Pix[si+3]) / 255
			if sa <= 0 {
				continue
			}
			blendInto(c.Pix, ty*c.W*4+tx*4,
				float64(src.Pix[si])/255,
				float64(src.Pix[si+1])/255,
				float64(src.Pix[si+2])/255,
				sa, mode)
		}
	}
}

// spriteLike builds an n×n source with fully transparent, fully opaque and
// partially translucent regions, exercising every branch of drawImageRow.
func spriteLike(n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := range n {
		for x := range n {
			i := img.PixOffset(x, y)
			img.Pix[i] = uint8((x * 5) % 256)
			img.Pix[i+1] = uint8((y * 11) % 256)
			img.Pix[i+2] = uint8((x + y) % 256)
			switch (x + y) % 3 {
			case 0:
				img.Pix[i+3] = 0
			case 1:
				img.Pix[i+3] = 255
			default:
				img.Pix[i+3] = uint8((x*y)%254 + 1)
			}
		}
	}
	return img
}

// TestDrawImageMatchesReference pins DrawImage (clipping hoisted, opaque
// source-over copy fast path, banded rows) against the per-pixel oracle,
// including placements that clip on every edge.
func TestDrawImageMatchesReference(t *testing.T) {
	const cw, ch, sn = 640, 480, 400 // sn*sn >> parallelMinPixels
	src := spriteLike(sn)
	modes := []CompositionMode{ModeSourceOver, ModeMultiply, ModeLuminosity, ModeXor, ModeLighter}
	offsets := [][2]int{{40, 30}, {-100, -50}, {cw - 60, ch - 60}, {-10, ch - sn/2}}

	for _, mode := range modes {
		for _, off := range offsets {
			for _, opaque := range []bool{true, false} {
				got := variedBackdrop(cw, ch, opaque)
				want := variedBackdrop(cw, ch, opaque)

				got.DrawImage(src, off[0], off[1], mode)
				drawImageReference(want, src, off[0], off[1], mode)

				for i := range got.Pix {
					if got.Pix[i] != want.Pix[i] {
						t.Fatalf("mode=%s off=%v opaque=%v: byte %d got=%d want=%d",
							mode, off, opaque, i, got.Pix[i], want.Pix[i])
					}
				}
			}
		}
	}
}

// normalReference is the original full-frame modulo-and-switch normal-map
// pass, kept as the oracle for the banded row-structured kernel.
func normalReference(dst, src []uint8, w, h int) {
	stride := w * 4
	for i := 0; i < w*h*4; i += 4 {
		var x1, x2, y1, y2 int
		col := i % stride
		switch {
		case col == 0:
			x1, x2 = int(src[i]), int(src[i+4])
		case col == (w-1)*4:
			x1, x2 = int(src[i-4]), int(src[i])
		default:
			x1, x2 = int(src[i-4]), int(src[i+4])
		}
		switch {
		case i < stride:
			y1, y2 = int(src[i]), int(src[i+stride])
		case i >= (h-1)*stride:
			y1, y2 = int(src[i-stride]), int(src[i])
		default:
			y1, y2 = int(src[i-stride]), int(src[i+stride])
		}
		dst[i] = clampByte(x1 - x2 + 127)
		dst[i+1] = clampByte(y1 - y2 + 127)
		dst[i+2] = 255
		dst[i+3] = 255
	}
}

// TestApplyNormalMatchesReference pins the banded normal-map pass against the
// original full-frame oracle, at a size that splits into several bands and at
// an edge-heavy tiny size. Widths and heights below 2 are excluded: the oracle
// (like the code it was extracted from) indexes past the buffer there, while
// the banded kernel clamps such edges safely.
func TestApplyNormalMatchesReference(t *testing.T) {
	for _, size := range [][2]int{{1024, 512}, {3, 3}, {2, 2}} {
		w, h := size[0], size[1]
		c, err := Generate(Default(), w, h, NewRNG(99))
		if err != nil {
			t.Fatal(err)
		}
		want := make([]uint8, len(c.Pix))
		normalReference(want, c.Pix, w, h)

		ApplyNormal(c)

		for i := range c.Pix {
			if c.Pix[i] != want[i] {
				t.Fatalf("%dx%d: byte %d got=%d want=%d", w, h, i, c.Pix[i], want[i])
			}
		}
	}
}

// BenchmarkApplyNormal tracks the normal-map post-processing pass.
func BenchmarkApplyNormal(b *testing.B) {
	c, err := Generate(Default(), 2048, 2048, NewRNG(12345))
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(c.Pix)))
	for b.Loop() {
		ApplyNormal(c)
	}
}
