package gen

import "math"

// CompositionMode mirrors the canvas globalCompositeOperation values used by the
// original generator (constants.ts CompositionMode).
type CompositionMode string

const (
	ModeColorBurn  CompositionMode = "color-burn"
	ModeColorDodge CompositionMode = "color-dodge"
	ModeDarken     CompositionMode = "darken"
	ModeDifference CompositionMode = "difference"
	ModeExclusion  CompositionMode = "exclusion"
	ModeHardLight  CompositionMode = "hard-light"
	ModeLighten    CompositionMode = "lighten"
	ModeLighter    CompositionMode = "lighter"
	ModeLuminosity CompositionMode = "luminosity"
	ModeMultiply   CompositionMode = "multiply"
	ModeOverlay    CompositionMode = "overlay"
	ModeScreen     CompositionMode = "screen"
	ModeSoftLight  CompositionMode = "soft-light"
	ModeSourceAtop CompositionMode = "source-atop"
	ModeSourceOver CompositionMode = "source-over"
	ModeXor        CompositionMode = "xor"
)

// IsValidCompositionMode reports whether s names a supported composition mode.
func IsValidCompositionMode(s string) bool {
	for _, m := range allCompositionModes {
		if string(m) == s {
			return true
		}
	}
	return false
}

// AllCompositionModes returns every supported composition mode.
func AllCompositionModes() []CompositionMode {
	out := make([]CompositionMode, len(allCompositionModes))
	copy(out, allCompositionModes)
	return out
}

// AllSpritePacks returns every available sprite pack in canonical order.
func AllSpritePacks() []SpritesPack {
	out := make([]SpritesPack, len(spritePackOrder))
	copy(out, spritePackOrder)
	return out
}

// isPorterDuff reports whether a mode is a pure Porter-Duff operator (normal
// blend) rather than a separable/non-separable blend mode.
func isPorterDuff(m CompositionMode) bool {
	switch m {
	case ModeSourceOver, ModeSourceAtop, ModeXor, ModeLighter:
		return true
	}
	return false
}

// to8 converts a normalized [0,1] component to an 8-bit value with rounding.
func to8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// blendInto composites a straight-alpha source colour (csR,csG,csB in [0,1],
// sa in [0,1]) onto the destination straight-RGBA pixel at pix[di:di+4], using
// the W3C Compositing & Blending model so canvas blend modes are reproduced.
func blendInto(pix []uint8, di int, csR, csG, csB, sa float64, mode CompositionMode) {
	if sa <= 0 {
		return
	}

	ab := float64(pix[di+3]) / 255

	// Fast path: plain source-over onto an opaque backdrop (the default case).
	if mode == ModeSourceOver && ab >= 1 {
		inv := 1 - sa
		pix[di] = to8(csR*sa + float64(pix[di])/255*inv)
		pix[di+1] = to8(csG*sa + float64(pix[di+1])/255*inv)
		pix[di+2] = to8(csB*sa + float64(pix[di+2])/255*inv)
		// alpha stays 255
		return
	}

	rb := float64(pix[di]) / 255
	gb := float64(pix[di+1]) / 255
	bb := float64(pix[di+2]) / 255

	// Porter-Duff coefficients (Fa, Fb).
	var fa, fb float64
	switch mode {
	case ModeSourceAtop:
		fa, fb = ab, 1-sa
	case ModeXor:
		fa, fb = 1-ab, 1-sa
	case ModeLighter:
		fa, fb = 1, 1
	default: // source-over and every blend mode composite as source-over
		fa, fb = 1, 1-sa
	}

	// B(Cb, Cs): the blended colour.
	var bR, bG, bB float64
	if isPorterDuff(mode) {
		bR, bG, bB = csR, csG, csB
	} else if mode == ModeLuminosity {
		bR, bG, bB = setLum(rb, gb, bb, lum(csR, csG, csB))
	} else {
		bR = sepBlend(rb, csR, mode)
		bG = sepBlend(gb, csG, mode)
		bB = sepBlend(bb, csB, mode)
	}

	// Cs' = (1 - ab)*Cs + ab*B(Cb, Cs)
	sR := (1-ab)*csR + ab*bR
	sG := (1-ab)*csG + ab*bG
	sB := (1-ab)*csB + ab*bB

	ao := sa*fa + ab*fb
	if ao <= 0 {
		pix[di], pix[di+1], pix[di+2], pix[di+3] = 0, 0, 0, 0
		return
	}
	pix[di] = to8((sa*fa*sR + ab*fb*rb) / ao)
	pix[di+1] = to8((sa*fa*sG + ab*fb*gb) / ao)
	pix[di+2] = to8((sa*fa*sB + ab*fb*bb) / ao)
	pix[di+3] = to8(ao)
}

// sepBlend applies a separable blend function B(Cb, Cs) per channel, on [0,1].
func sepBlend(cb, cs float64, mode CompositionMode) float64 {
	switch mode {
	case ModeMultiply:
		return cb * cs
	case ModeScreen:
		return cb + cs - cb*cs
	case ModeOverlay:
		if cb <= 0.5 {
			return 2 * cs * cb
		}
		return 1 - 2*(1-cs)*(1-cb)
	case ModeDarken:
		return math.Min(cb, cs)
	case ModeLighten:
		return math.Max(cb, cs)
	case ModeColorDodge:
		if cb == 0 {
			return 0
		}
		if cs == 1 {
			return 1
		}
		return math.Min(1, cb/(1-cs))
	case ModeColorBurn:
		if cb == 1 {
			return 1
		}
		if cs == 0 {
			return 0
		}
		return 1 - math.Min(1, (1-cb)/cs)
	case ModeHardLight:
		if cs <= 0.5 {
			return 2 * cs * cb
		}
		return 1 - 2*(1-cs)*(1-cb)
	case ModeSoftLight:
		if cs <= 0.5 {
			return cb - (1-2*cs)*cb*(1-cb)
		}
		var d float64
		if cb <= 0.25 {
			d = ((16*cb-12)*cb + 4) * cb
		} else {
			d = math.Sqrt(cb)
		}
		return cb + (2*cs-1)*(d-cb)
	case ModeDifference:
		return math.Abs(cb - cs)
	case ModeExclusion:
		return cb + cs - 2*cb*cs
	default:
		return cs
	}
}

// Non-separable helpers (used by luminosity), per W3C Compositing spec.
func lum(r, g, b float64) float64 { return 0.3*r + 0.59*g + 0.11*b }

func clipColor(r, g, b float64) (float64, float64, float64) {
	l := lum(r, g, b)
	n := math.Min(r, math.Min(g, b))
	x := math.Max(r, math.Max(g, b))
	if n < 0 {
		r = l + (r-l)*l/(l-n)
		g = l + (g-l)*l/(l-n)
		b = l + (b-l)*l/(l-n)
	}
	if x > 1 {
		r = l + (r-l)*(1-l)/(x-l)
		g = l + (g-l)*(1-l)/(x-l)
		b = l + (b-l)*(1-l)/(x-l)
	}
	return r, g, b
}

func setLum(r, g, b, l float64) (float64, float64, float64) {
	d := l - lum(r, g, b)
	return clipColor(r+d, g+d, b+d)
}
