package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"godisplacementx/internal/gen"
)

// loadParams reads a Params JSON config file.
func loadParams(path string) (gen.Params, error) {
	p := gen.Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("parse config %s: %w", path, err)
	}
	return p, nil
}

// parseGradient parses a comma-separated list of hex colours (e.g.
// "#00ffff,#9500ff,#ffe500") into colour stops. Returns nil for an empty string.
func parseGradient(s string) ([]gen.ColorRGB, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	stops := make([]gen.ColorRGB, 0, len(parts))
	for _, p := range parts {
		c, err := parseHexColor(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		stops = append(stops, c)
	}
	return stops, nil
}

func parseHexColor(s string) (gen.ColorRGB, error) {
	s = strings.TrimPrefix(s, "#")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return gen.ColorRGB{}, fmt.Errorf("invalid hex colour %q", s)
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return gen.ColorRGB{}, fmt.Errorf("invalid hex colour %q: %w", s, err)
	}
	return gen.ColorRGB{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}, nil
}

// parsePacks converts pack names to typed sprite packs, validating each.
func parsePacks(names []string) ([]gen.SpritesPack, error) {
	valid := map[string]gen.SpritesPack{
		"classic":   gen.PackClassic,
		"bigdata":   gen.PackBigdata,
		"aggromaxx": gen.PackAggromaxx,
		"crappack":  gen.PackCrappack,
	}
	out := make([]gen.SpritesPack, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		p, ok := valid[n]
		if !ok {
			return nil, fmt.Errorf("unknown sprite pack %q (valid: classic, bigdata, aggromaxx, crappack)", n)
		}
		out = append(out, p)
	}
	return out, nil
}

// parseModes converts composition mode names to typed modes, validating each.
func parseModes(names []string) ([]gen.CompositionMode, error) {
	out := make([]gen.CompositionMode, 0, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if !gen.IsValidCompositionMode(n) {
			return nil, fmt.Errorf("unknown composition mode %q", n)
		}
		out = append(out, gen.CompositionMode(n))
	}
	return out, nil
}
