package gen

// LUT-accelerated compositing for the common FillRect case: a constant grayscale
// source blended onto an (almost always) opaque backdrop with a fixed alpha and
// composition mode.
//
// For every supported blend mode the compositor treats the R,G,B channels
// independently, and on an opaque backdrop each output channel is a pure function
// of that channel's backdrop byte alone. Since the source is grayscale (R=G=B), a
// single 256-entry table maps every possible backdrop byte to its composited
// result for all three channels. Building the table once per rectangle and looking
// up per pixel replaces the per-pixel float compositor (blendInto) with three
// L1-resident byte loads — a large win for the heavy separable modes and a solid
// one even for plain source-over.
//
// The table is bit-exact by construction: each entry is produced by running the
// reference compositor (blendInto) on a synthetic opaque grayscale pixel, so the
// LUT path yields byte-identical output to blendRunScalar. Any pixel whose backdrop
// is not fully opaque (only reachable after an xor draw) falls back to blendInto,
// preserving exactness there too.

// blendLUTKey identifies a blend table by the inputs that determine it: the
// grayscale source byte, the alpha percentage and the composition mode. It is the
// cache key for Canvas.lut.
type blendLUTKey struct {
	gray  uint8
	alpha int
	mode  CompositionMode
}

// blendLUTMinArea is the rectangle area (in pixels) below which building a
// 256-entry table does not pay for itself and the scalar compositor is used
// instead. Chosen so the ~256-entry build amortises across the rectangle's pixels.
const blendLUTMinArea = 512

// lutableMode reports whether mode composites as a per-channel function that
// leaves an opaque backdrop opaque, so a 256-entry LUT reproduces blendInto
// exactly on opaque pixels. Luminosity is non-separable (a channel's output
// depends on all three backdrop channels), and xor turns an opaque backdrop
// translucent (alpha becomes 1-sa); both are handled by the scalar compositor.
func lutableMode(mode CompositionMode) bool {
	switch mode {
	case ModeLuminosity, ModeXor:
		return false
	}
	return true
}

// buildBlendLUT fills lut so that lut[v] is exactly what blendInto writes for a
// backdrop channel value v on an opaque pixel, given a constant grayscale source g
// in [0,1], straight alpha sa in [0,1] and composition mode. Bit-exact by
// construction (it runs the reference compositor per entry).
func buildBlendLUT(lut *[256]uint8, g, sa float64, mode CompositionMode) {
	var px [4]uint8
	for v := 0; v < 256; v++ {
		px[0], px[1], px[2], px[3] = uint8(v), uint8(v), uint8(v), 255
		blendInto(px[:], 0, g, g, g, sa, mode)
		lut[v] = px[0]
	}
}

// blendRunLUT composites a constant grayscale source onto n consecutive pixels at
// byte offset di using a prebuilt LUT for the opaque-backdrop case. Any pixel whose
// backdrop is not fully opaque falls back to the exact scalar compositor, so the
// result is byte-identical to blendRunScalar.
func blendRunLUT(pix []uint8, di, n int, lut *[256]uint8, g, sa float64, mode CompositionMode) {
	for k := 0; k < n; k++ {
		o := di + k*4
		if pix[o+3] == 255 {
			pix[o] = lut[pix[o]]
			pix[o+1] = lut[pix[o+1]]
			pix[o+2] = lut[pix[o+2]]
		} else {
			blendInto(pix, o, g, g, g, sa, mode)
		}
	}
}
