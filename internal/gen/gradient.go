package gen

// ColorRGB matches the original ColorRGB type (0-255 per channel). JSON tags
// match the gradient editor's shape.
type ColorRGB struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

// DefaultGradient mirrors the colorsDefault in Gradient.tsx.
func DefaultGradient() []ColorRGB {
	return []ColorRGB{
		{R: 0, G: 255, B: 255},
		{R: 149, G: 0, B: 255},
		{R: 255, G: 229, B: 0},
	}
}

// Palette is a 256-entry RGB lookup table.
type Palette struct {
	R, G, B [256]uint8
}

// BuildPalette builds a 256-entry LUT by linearly interpolating the colour
// stops, reproducing the original's 256px linear gradient (stops placed at
// i/max(n-1,1)). With fewer than two stops it falls back to the default.
func BuildPalette(stops []ColorRGB) Palette {
	if len(stops) < 2 {
		stops = DefaultGradient()
	}
	n := len(stops)
	denom := float64(n - 1)
	var p Palette
	for x := 0; x < 256; x++ {
		t := float64(x) / 255 // position along [0,1]
		// Find the segment [j, j+1] containing t.
		seg := int(t * denom)
		if seg >= n-1 {
			seg = n - 2
		}
		pos0 := float64(seg) / denom
		pos1 := float64(seg+1) / denom
		local := 0.0
		if pos1 > pos0 {
			local = (t - pos0) / (pos1 - pos0)
		}
		c0, c1 := stops[seg], stops[seg+1]
		p.R[x] = lerp8(c0.R, c1.R, local)
		p.G[x] = lerp8(c0.G, c1.G, local)
		p.B[x] = lerp8(c0.B, c1.B, local)
	}
	return p
}

func lerp8(a, b uint8, t float64) uint8 {
	v := float64(a) + (float64(b)-float64(a))*t
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
