package gen

import "testing"

func TestGenerateAllLayersDisabled(t *testing.T) {
	p := tinyParams()
	p.RectEnabled = false
	p.GridEnabled = false
	p.ColsEnabled = false
	p.RowsEnabled = false
	p.LinesEnabled = false
	p.Iterations = 60
	c, err := Generate(p, 16, 16, NewRNG(1))
	if err != nil {
		t.Fatal(err)
	}
	bg := uint8(p.BackgroundBrightness)
	for i := 0; i < len(c.Pix); i += 4 {
		if c.Pix[i] != bg {
			t.Fatal("disabled layers must leave the background untouched")
		}
	}
}

func TestGenerateWithSprites(t *testing.T) {
	p := tinyParams()
	p.SpritesEnabled = true
	p.SpritesPacks = []SpritesPack{PackClassic}
	p.Iterations = 40
	if _, err := Generate(p, 32, 32, NewRNG(2)); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateSpritesNoRotation(t *testing.T) {
	p := tinyParams()
	p.SpritesEnabled = true
	p.SpritesRotationEnabled = false
	p.Iterations = 40
	if _, err := Generate(p, 32, 32, NewRNG(3)); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateSeamlessCoversAllPrimitives(t *testing.T) {
	p := tinyParams()
	p.SeamlessTextureEnabled = true
	p.SpritesEnabled = true
	p.Iterations = 120
	if _, err := Generate(p, 32, 32, NewRNG(4)); err != nil {
		t.Fatal(err)
	}
}

func TestDrawSpriteNilSet(t *testing.T) {
	c := NewCanvas(8, 8)
	drawSprite(c, NewRNG(1), tinyParams(), nil, ModeSourceOver)
}

func TestDrawSpriteEmptySet(t *testing.T) {
	c := NewCanvas(8, 8)
	drawSprite(c, NewRNG(1), tinyParams(), &SpriteSet{}, ModeSourceOver)
}

func TestDrawSpriteNilImage(t *testing.T) {
	s, err := LoadSprites([]SpritesPack{PackClassic})
	if err != nil {
		t.Fatal(err)
	}
	c := NewCanvas(0, 0)
	drawSprite(c, NewRNG(1), tinyParams(), s, ModeSourceOver)
}

func TestClampInt(t *testing.T) {
	if clampInt(-1, 0, 255) != 0 {
		t.Fatal("low clamp")
	}
	if clampInt(300, 0, 255) != 255 {
		t.Fatal("high clamp")
	}
	if clampInt(7, 0, 255) != 7 {
		t.Fatal("mid pass-through")
	}
}

func TestDrawSeamless(t *testing.T) {
	var calls [][4]int
	record := func(x, y, rw, rh int) { calls = append(calls, [4]int{x, y, rw, rh}) }

	drawSeamless(16, 16, 3, 4, 5, 6, false, record)
	if len(calls) != 1 || calls[0] != [4]int{3, 4, 5, 6} {
		t.Fatalf("non-seamless draw = %v", calls)
	}

	calls = nil
	drawSeamless(16, 16, 14, 15, 8, 8, true, record)
	if len(calls) != 4 {
		t.Fatalf("wrap-around corner must draw 4 tiles, got %d", len(calls))
	}
}
