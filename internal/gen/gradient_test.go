package gen

import "testing"

func TestBuildPaletteEndpoints(t *testing.T) {
	p := BuildPalette(DefaultGradient())
	// Index 0 is the first stop {0,255,255}; index 255 is the last {255,229,0}.
	if p.R[0] != 0 || p.G[0] != 255 || p.B[0] != 255 {
		t.Errorf("palette[0] = {%d,%d,%d}, want {0,255,255}", p.R[0], p.G[0], p.B[0])
	}
	if p.R[255] != 255 || p.G[255] != 229 || p.B[255] != 0 {
		t.Errorf("palette[255] = {%d,%d,%d}, want {255,229,0}", p.R[255], p.G[255], p.B[255])
	}
	// Midpoint should be near the middle stop {149,0,255}.
	if p.B[128] < 200 {
		t.Errorf("palette[128].B = %d, expected high (near middle stop)", p.B[128])
	}
}

func TestBuildPaletteFallback(t *testing.T) {
	p := BuildPalette([]ColorRGB{{R: 10, G: 20, B: 30}}) // < 2 stops -> default
	def := BuildPalette(DefaultGradient())
	if p.R[200] != def.R[200] {
		t.Errorf("single-stop palette did not fall back to default")
	}
}
