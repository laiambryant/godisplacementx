package gen

import "testing"

// integerFrom golden vectors ported from src/utils/random.test.ts, which mocked
// Math.random() and asserted the resulting integers.
func TestIntegerFrom(t *testing.T) {
	// randomInteger(0, 10) with mocked random 0.0..0.9 -> 0..9.
	for i := 0; i <= 9; i++ {
		f := float64(i) / 10
		got := integerFrom(f, 0, 10)
		if got != i {
			t.Errorf("integerFrom(%v, 0, 10) = %d, want %d", f, got, i)
		}
	}
	// randomColorRGB used range [0,255]: 0->0, 0.05->12, 0.1->25.
	cases := []struct {
		f    float64
		want int
	}{
		{0, 0},
		{0.05, 12},
		{0.1, 25},
		{0.999, 255},
	}
	for _, c := range cases {
		if got := integerFrom(c.f, 0, 255); got != c.want {
			t.Errorf("integerFrom(%v, 0, 255) = %d, want %d", c.f, got, c.want)
		}
	}
}

func TestIntegerBounds(t *testing.T) {
	g := NewRNG(42)
	for i := 0; i < 100000; i++ {
		v := g.Integer(2, 10)
		if v < 2 || v > 10 {
			t.Fatalf("Integer(2,10) out of range: %d", v)
		}
	}
}

func TestBooleanThreshold(t *testing.T) {
	if integerFrom(0.5, 0, 1) != 1 { // sanity: floor(0.5*2)=1
		t.Fatal("formula sanity failed")
	}
}

// TestDeterminism verifies that the same seed reproduces the same sequence.
func TestDeterminism(t *testing.T) {
	a := NewRNG(123)
	b := NewRNG(123)
	for i := 0; i < 1000; i++ {
		if a.Integer(0, 1_000_000) != b.Integer(0, 1_000_000) {
			t.Fatalf("sequences diverged at %d", i)
		}
	}
}
