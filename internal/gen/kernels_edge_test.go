package gen

import "testing"

func TestApplyInvert(t *testing.T) {
	c := NewCanvas(4, 4)
	c.Fill(10)
	ApplyInvert(c)
	if c.Pix[0] != 245 || c.Pix[1] != 245 || c.Pix[2] != 245 || c.Pix[3] != 255 {
		t.Fatalf("invert = %v", c.Pix[:4])
	}
}

func TestInvertScalarLeavesAlpha(t *testing.T) {
	pix := []uint8{0, 128, 255, 42}
	invertScalar(pix)
	want := []uint8{255, 127, 0, 42}
	for i := range want {
		if pix[i] != want[i] {
			t.Fatalf("pix = %v, want %v", pix, want)
		}
	}
}

func TestFillScalarShortBuffer(t *testing.T) {
	pix := []uint8{1, 2, 3}
	fillScalar(pix, 9)
	if pix[0] != 1 {
		t.Fatal("sub-pixel buffer must stay untouched")
	}
}

func TestBlendIntoZeroAlphaNoop(t *testing.T) {
	pix := []uint8{10, 20, 30, 255}
	blendInto(pix, 0, 1, 1, 1, 0, ModeSourceOver)
	if pix[0] != 10 {
		t.Fatal("zero source alpha must be a no-op")
	}
}

func TestSepBlendDefaultReturnsSource(t *testing.T) {
	if got := sepBlend(0.25, 0.75, ModeSourceOver); got != 0.75 {
		t.Fatalf("default branch = %v, want cs", got)
	}
}

func TestParallelBandsMinPerBandClamp(t *testing.T) {
	total := 0
	parallelBands(8, 0, func(lo, hi int) { total += hi - lo })
	if total != 8 {
		t.Fatalf("processed %d units, want 8", total)
	}
}

func TestMinRowsPerBandTinyRows(t *testing.T) {
	if minRowsPerBand(0) != 1 {
		t.Fatal("zero-width rows must clamp to 1")
	}
	if minRowsPerBand(parallelMinPixels) != 1 {
		t.Fatal("full-band rows must be 1")
	}
}

func TestDrawImageFullyClipped(t *testing.T) {
	c := NewCanvas(8, 8)
	c.Fill(0)
	s, err := LoadSprites([]SpritesPack{PackClassic})
	if err != nil {
		t.Fatal(err)
	}
	img := s.Render(0, 16, 0, false)
	c.DrawImage(img, 100, 100, ModeSourceOver)
	if c.Pix[0] != 0 {
		t.Fatal("fully clipped draw must not touch pixels")
	}
}
