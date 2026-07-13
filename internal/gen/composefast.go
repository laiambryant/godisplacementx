package gen

import "sync"

// Fast-mode sprite compositor. On an opaque backdrop the W3C compositor
// reduces to out = to8(sa·B(Cb,Cs) + (1−sa)·Cb): the blended colour B lives in
// a per-mode 256×256 table built from the exact float sepBlend, and the outer
// mix becomes an integer lerp — error ≤ ±1 per channel versus blendInto.
// Luminosity (non-separable), xor (turns the backdrop translucent) and lighter
// (not a lerp) keep the exact compositor, as does any non-opaque backdrop
// pixel, so fast mode only ever approximates the separable opaque case.

var fastBlendLUTs = struct {
	mu   sync.Mutex
	luts map[CompositionMode]*[256][256]uint8
}{luts: map[CompositionMode]*[256][256]uint8{}}

// fastBlendLUT returns the blended-colour table for a separable mode:
// lut[cb][cs] = to8(B(cb/255, cs/255)). Built once per mode per process.
func fastBlendLUT(mode CompositionMode) *[256][256]uint8 {
	fastBlendLUTs.mu.Lock()
	defer fastBlendLUTs.mu.Unlock()
	if lut, ok := fastBlendLUTs.luts[mode]; ok {
		return lut
	}
	lut := &[256][256]uint8{}
	for cb := 0; cb < 256; cb++ {
		for cs := 0; cs < 256; cs++ {
			lut[cb][cs] = to8(sepBlend(float64(cb)/255, float64(cs)/255, mode))
		}
	}
	fastBlendLUTs.luts[mode] = lut
	return lut
}

// fastCompositesAsSourceOver reports whether B(Cb,Cs) = Cs for the mode on an
// opaque backdrop, so no blend table is needed.
func fastCompositesAsSourceOver(mode CompositionMode) bool {
	return mode == ModeSourceOver || mode == ModeSourceAtop
}

// fastRowSupported reports whether drawImageRowFast handles the mode at all.
func fastRowSupported(mode CompositionMode) bool {
	switch mode {
	case ModeLuminosity, ModeXor, ModeLighter:
		return false
	}
	return true
}

// lerp255 mixes b over cb with weight a/255, rounding to nearest.
func lerp255(a, b, cb uint32) uint8 {
	return uint8((a*b + (255-a)*cb + 127) / 255)
}

// drawImageRowFast composites one clipped source row like drawImageRow, but
// through the integer opaque-backdrop approximation. Pixels whose backdrop is
// not fully opaque fall back to the exact compositor.
func (c *Canvas) drawImageRowFast(srow []uint8, di int, mode CompositionMode) {
	if !fastRowSupported(mode) {
		c.drawImageRow(srow, di, mode)
		return
	}
	sourceLike := fastCompositesAsSourceOver(mode)
	var lut *[256][256]uint8
	if !sourceLike {
		lut = fastBlendLUT(mode)
	}
	for o := 0; o+4 <= len(srow); o, di = o+4, di+4 {
		a := uint32(srow[o+3])
		if a == 0 {
			continue
		}
		if c.Pix[di+3] != 255 {
			blendInto(c.Pix, di,
				float64(srow[o])/255,
				float64(srow[o+1])/255,
				float64(srow[o+2])/255,
				float64(a)/255, mode)
			continue
		}
		if a == 255 && sourceLike {
			c.Pix[di], c.Pix[di+1], c.Pix[di+2] = srow[o], srow[o+1], srow[o+2]
			continue
		}
		var bR, bG, bB uint32
		if sourceLike {
			bR, bG, bB = uint32(srow[o]), uint32(srow[o+1]), uint32(srow[o+2])
		} else {
			bR = uint32(lut[c.Pix[di]][srow[o]])
			bG = uint32(lut[c.Pix[di+1]][srow[o+1]])
			bB = uint32(lut[c.Pix[di+2]][srow[o+2]])
		}
		c.Pix[di] = lerp255(a, bR, uint32(c.Pix[di]))
		c.Pix[di+1] = lerp255(a, bG, uint32(c.Pix[di+1]))
		c.Pix[di+2] = lerp255(a, bB, uint32(c.Pix[di+2]))
	}
}
