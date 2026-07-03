package gen

import "testing"

// blendOne composites a single grayscale source over a single opaque grayscale
// backdrop and returns the resulting R channel (R=G=B for grayscale).
func blendOne(dst uint8, srcGray float64, sa float64, mode CompositionMode) uint8 {
	pix := []uint8{dst, dst, dst, 255}
	blendInto(pix, 0, srcGray, srcGray, srcGray, sa, mode)
	return pix[0]
}

func TestBlendModes(t *testing.T) {
	cases := []struct {
		name string
		dst  uint8
		src  float64
		sa   float64
		mode CompositionMode
		want uint8
	}{
		{"source-over half white over black", 0, 1, 0.5, ModeSourceOver, 128},
		{"source-over opaque white over black", 0, 1, 1, ModeSourceOver, 255},
		{"multiply 0.5x0.5", 128, 0.5, 1, ModeMultiply, 64},
		{"screen 0.5,0.5", 128, 0.5, 1, ModeScreen, 191},
		{"darken keeps min", 64, 0.5, 1, ModeDarken, 64},
		{"lighten keeps max", 64, 0.5, 1, ModeLighten, 128},
		{"difference", 200, 0.2, 1, ModeDifference, 149},
	}
	for _, c := range cases {
		got := blendOne(c.dst, c.src, c.sa, c.mode)
		// Allow a tolerance of 1 for rounding.
		if diff(int(got), int(c.want)) > 1 {
			t.Errorf("%s: got %d, want ~%d", c.name, got, c.want)
		}
	}
}

func diff(a, b int) int {
	if a < b {
		return b - a
	}
	return a - b
}
