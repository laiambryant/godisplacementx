package gen

import "testing"

func TestRandomItem(t *testing.T) {
	g := NewRNG(1)
	if _, ok := RandomItem(g, []int{}); ok {
		t.Fatal("empty slice must report ok=false")
	}
	v, ok := RandomItem(g, []int{42})
	if !ok || v != 42 {
		t.Fatalf("single item = %d, %v", v, ok)
	}
}

func TestRandomizeStaysInRange(t *testing.T) {
	p := Default()
	p.Randomize(NewRNG(99))
	if p.Iterations < rIterations.Min || p.Iterations > rIterations.Max {
		t.Fatalf("iterations %d out of range", p.Iterations)
	}
	if p.BackgroundBrightness < 0 || p.BackgroundBrightness > 255 {
		t.Fatalf("background %d out of range", p.BackgroundBrightness)
	}
	for _, d := range []Dual{p.RectBrightness, p.RectAlpha, p.GridAmount, p.LinesWidth} {
		if d[0] < 0 || d[1] < 0 {
			t.Fatalf("dual %v out of range", d)
		}
	}
	for _, pack := range p.SpritesPacks {
		if spritePackCounts[pack] == 0 {
			t.Fatalf("unknown pack %q", pack)
		}
	}
	for _, m := range p.CompositionModes {
		if !IsValidCompositionMode(string(m)) {
			t.Fatalf("unknown mode %q", m)
		}
	}
}

func TestRandomizeDeterministic(t *testing.T) {
	a, b := Default(), Default()
	a.Randomize(NewRNG(5))
	b.Randomize(NewRNG(5))
	if a.Iterations != b.Iterations || a.GridGap != b.GridGap || len(a.CompositionModes) != len(b.CompositionModes) {
		t.Fatal("Randomize must be deterministic for a fixed seed")
	}
}

func TestIsValidCompositionMode(t *testing.T) {
	if !IsValidCompositionMode("multiply") {
		t.Fatal("multiply must be valid")
	}
	if IsValidCompositionMode("nope") {
		t.Fatal("nope must be invalid")
	}
}

func TestAllCompositionModesCopies(t *testing.T) {
	modes := AllCompositionModes()
	if len(modes) != len(allCompositionModes) {
		t.Fatal("length mismatch")
	}
	modes[0] = "mutated"
	if allCompositionModes[0] == "mutated" {
		t.Fatal("returned slice must be a copy")
	}
}

func TestAllSpritePacksCopies(t *testing.T) {
	packs := AllSpritePacks()
	if len(packs) != len(spritePackOrder) {
		t.Fatal("length mismatch")
	}
	packs[0] = "mutated"
	if spritePackOrder[0] == "mutated" {
		t.Fatal("returned slice must be a copy")
	}
}
