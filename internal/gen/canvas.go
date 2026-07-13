package gen

import (
	"image"
	"math"
)

// jsRound replicates JS Math.round: floor(x + 0.5) (round half toward +Inf).
func jsRound(x float64) int { return int(math.Floor(x + 0.5)) }

// Canvas is an in-memory straight-alpha RGBA buffer, matching the semantics of
// the browser's ImageData (non-premultiplied, 0-255 per channel).
type Canvas struct {
	W, H int
	Pix  []uint8 // len == W*H*4, R,G,B,A

	// lut memoizes the most recently built blend table (blendlut.go). Consecutive
	// FillRect calls within one drawing primitive share the same (gray, alpha,
	// mode), so this single-entry cache collapses the per-rectangle table build
	// into a one-time cost per primitive. It is scratch state, not part of the
	// image, and is intentionally not copied by Clone.
	lut      [256]uint8
	lutKey   blendLUTKey
	lutValid bool

	// fast selects the non-deterministic approximate sprite paths (composefast,
	// spritescale_fast). Like lut it is scratch render state, not image data,
	// and is not copied by Clone: clones are only post-processed and encoded,
	// which never consult the flag.
	fast bool
}

// NewCanvas allocates a fully transparent canvas.
func NewCanvas(w, h int) *Canvas {
	return &Canvas{W: w, H: h, Pix: make([]uint8, w*h*4)}
}

// Clone returns a deep copy of the canvas. Used to derive several post-processed
// outputs from a single generated height field without mutating the source.
func (c *Canvas) Clone() *Canvas {
	pix := make([]uint8, len(c.Pix))
	copy(pix, c.Pix)
	return &Canvas{W: c.W, H: c.H, Pix: pix}
}

// Fill sets every pixel to an opaque grayscale value (used for the background).
func (c *Canvas) Fill(gray uint8) {
	parallelBands(len(c.Pix)/4, parallelMinPixels, func(lo, hi int) {
		fillRun(c.Pix[lo*4:hi*4], gray)
	})
}

// FillRect blends a grayscale rectangle (gray in 0-255, alphaPct in 0-100) onto
// the canvas using the given composition mode. Coordinates may be negative; the
// rectangle is clipped to the canvas bounds.
func (c *Canvas) FillRect(x, y, w, h int, gray uint8, alphaPct int, mode CompositionMode) {
	if w <= 0 || h <= 0 {
		return
	}
	x0, y0 := x, y
	x1, y1 := x+w, y+h
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.W {
		x1 = c.W
	}
	if y1 > c.H {
		y1 = c.H
	}
	if x0 >= x1 || y0 >= y1 {
		return
	}
	g := float64(gray) / 255
	sa := float64(alphaPct) / 100

	// Fast path: composite through a 256-entry blend table, turning the per-pixel
	// float compositor into three table lookups (bit-identical to the scalar path;
	// see blendlut.go).
	if sa > 0 && lutableMode(mode) {
		if lut := c.blendLUTFor(gray, alphaPct, mode, (x1-x0)*(y1-y0)); lut != nil {
			c.blendRows(x0, y0, x1, y1, func(di, n int) {
				blendRunLUT(c.Pix, di, n, lut, g, sa, mode)
			})
			return
		}
	}

	c.blendRows(x0, y0, x1, y1, func(di, n int) {
		blendRun(c.Pix, di, n, g, sa, mode)
	})
}

// blendLUTFor returns the blend table for (gray, alphaPct, mode), reusing the
// cached one when it matches — successive rects in a grid / column / row share
// one key, so the table is built once per primitive. It returns nil when no
// table is cached and the rectangle is too small to amortise a fresh build.
func (c *Canvas) blendLUTFor(gray uint8, alphaPct int, mode CompositionMode, area int) *[256]uint8 {
	key := blendLUTKey{gray: gray, alpha: alphaPct, mode: mode}
	if c.lutValid && c.lutKey == key {
		return &c.lut
	}
	if area < blendLUTMinArea {
		return nil
	}
	buildBlendLUT(&c.lut, float64(gray)/255, float64(alphaPct)/100, mode)
	c.lutKey, c.lutValid = key, true
	return &c.lut
}

// blendRows runs blendRow over every row of the clipped rectangle, splitting
// the rows across goroutines when the area pays for it. Each row is a
// contiguous run with a constant source, and blendRow only touches its own
// pixels, so the split is bit-exact.
func (c *Canvas) blendRows(x0, y0, x1, y1 int, blendRow func(di, n int)) {
	rw := x1 - x0
	parallelBands(y1-y0, minRowsPerBand(rw), func(lo, hi int) {
		for yy := y0 + lo; yy < y0+hi; yy++ {
			blendRow(yy*c.W*4+x0*4, rw)
		}
	})
}

// DrawImage composites a straight-alpha source image at (dx, dy) using the given
// composition mode. The source is expected to already be at its target size.
func (c *Canvas) DrawImage(src *image.NRGBA, dx, dy int, mode CompositionMode) {
	b := src.Bounds()
	sx0, sy0, sx1, sy1 := clipToCanvas(b.Dx(), b.Dy(), dx, dy, c.W, c.H)
	if sx0 >= sx1 || sy0 >= sy1 {
		return
	}
	rw := sx1 - sx0
	blendImageRow := (*Canvas).drawImageRow
	if c.fast {
		blendImageRow = (*Canvas).drawImageRowFast
	}
	parallelBands(sy1-sy0, minRowsPerBand(rw), func(lo, hi int) {
		for sy := sy0 + lo; sy < sy0+hi; sy++ {
			srow := src.PixOffset(b.Min.X+sx0, b.Min.Y+sy)
			drow := (dy+sy)*c.W*4 + (dx+sx0)*4
			blendImageRow(c, src.Pix[srow:srow+rw*4], drow, mode)
		}
	})
}

// clipToCanvas intersects a sw×sh source placed at (dx, dy) with a w×h canvas
// and returns the surviving source-coordinate rectangle [sx0,sx1)×[sy0,sy1).
func clipToCanvas(sw, sh, dx, dy, w, h int) (sx0, sy0, sx1, sy1 int) {
	sx0, sy0, sx1, sy1 = 0, 0, sw, sh
	if dx < 0 {
		sx0 = -dx
	}
	if dy < 0 {
		sy0 = -dy
	}
	if dx+sw > w {
		sx1 = w - dx
	}
	if dy+sh > h {
		sy1 = h - dy
	}
	return
}

// drawImageRow composites one clipped source row (straight RGBA) onto the
// canvas at byte offset di. Fully transparent pixels are skipped; fully opaque
// source-over pixels copy straight through (blendInto with sa==1 reduces to
// the source bytes for any backdrop); everything else takes the exact
// compositor.
func (c *Canvas) drawImageRow(srow []uint8, di int, mode CompositionMode) {
	copyOpaque := mode == ModeSourceOver
	for o := 0; o+4 <= len(srow); o, di = o+4, di+4 {
		a := srow[o+3]
		if a == 0 {
			continue
		}
		if a == 255 && copyOpaque {
			c.Pix[di], c.Pix[di+1], c.Pix[di+2], c.Pix[di+3] = srow[o], srow[o+1], srow[o+2], 255
			continue
		}
		blendInto(c.Pix, di,
			float64(srow[o])/255,
			float64(srow[o+1])/255,
			float64(srow[o+2])/255,
			float64(a)/255, mode)
	}
}

// NRGBA exposes the canvas as a standard image for encoding. The buffer is
// shared (no copy); straight alpha matches image.NRGBA semantics.
func (c *Canvas) NRGBA() *image.NRGBA {
	return &image.NRGBA{
		Pix:    c.Pix,
		Stride: c.W * 4,
		Rect:   image.Rect(0, 0, c.W, c.H),
	}
}
