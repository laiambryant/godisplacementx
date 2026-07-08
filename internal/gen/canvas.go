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
	fillRun(c.Pix, gray)
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
	for yy := y0; yy < y1; yy++ {
		row := yy * c.W * 4
		// Each row is a contiguous run with a constant grayscale source,
		// alpha and mode: the batch kernel blends the whole run at once.
		blendRun(c.Pix, row+x0*4, x1-x0, g, sa, mode)
	}
}

// DrawImage composites a straight-alpha source image at (dx, dy) using the given
// composition mode. The source is expected to already be at its target size.
func (c *Canvas) DrawImage(src *image.NRGBA, dx, dy int, mode CompositionMode) {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	for sy := 0; sy < sh; sy++ {
		ty := dy + sy
		if ty < 0 || ty >= c.H {
			continue
		}
		srow := src.PixOffset(b.Min.X, b.Min.Y+sy)
		drow := ty * c.W * 4
		for sx := 0; sx < sw; sx++ {
			tx := dx + sx
			if tx < 0 || tx >= c.W {
				continue
			}
			si := srow + sx*4
			sa := float64(src.Pix[si+3]) / 255
			if sa <= 0 {
				continue
			}
			blendInto(c.Pix, drow+tx*4,
				float64(src.Pix[si])/255,
				float64(src.Pix[si+1])/255,
				float64(src.Pix[si+2])/255,
				sa, mode)
		}
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
