package gen

import (
	"image"
	"testing"
)

func spanImage(alphas []uint8) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, len(alphas), 1))
	for i, a := range alphas {
		img.Pix[i*4+0] = 200
		img.Pix[i*4+1] = 100
		img.Pix[i*4+2] = 50
		img.Pix[i*4+3] = a
	}
	return img
}

func fastCanvas(w, h int) *Canvas {
	c := NewCanvas(w, h)
	c.fast = true
	c.Fill(64)
	return c
}

func TestDrawImageFastSourceOver(t *testing.T) {
	c := fastCanvas(8, 1)
	c.DrawImage(spanImage([]uint8{0, 255, 128}), 0, 0, ModeSourceOver)
	if c.Pix[0] != 64 {
		t.Fatal("alpha-0 pixel must be skipped")
	}
	if c.Pix[4] != 200 {
		t.Fatal("opaque source-over pixel must copy through")
	}
	if c.Pix[8] == 64 || c.Pix[8] == 200 {
		t.Fatal("half-alpha pixel must lerp")
	}
}

func TestDrawImageFastSeparableLUT(t *testing.T) {
	exact := NewCanvas(8, 1)
	exact.Fill(64)
	fast := fastCanvas(8, 1)
	src := spanImage([]uint8{255, 128, 30})
	exact.DrawImage(src, 0, 0, ModeMultiply)
	fast.DrawImage(src, 0, 0, ModeMultiply)
	for i := 0; i < 8*4; i++ {
		d := int(exact.Pix[i]) - int(fast.Pix[i])
		if d < -1 || d > 1 {
			t.Fatalf("fast multiply deviates by %d at %d", d, i)
		}
	}
}

func TestDrawImageFastUnsupportedModeFallsBack(t *testing.T) {
	exact := NewCanvas(4, 1)
	exact.Fill(64)
	fast := fastCanvas(4, 1)
	src := spanImage([]uint8{255, 128})
	exact.DrawImage(src, 0, 0, ModeXor)
	fast.DrawImage(src, 0, 0, ModeXor)
	for i := range exact.Pix {
		if exact.Pix[i] != fast.Pix[i] {
			t.Fatal("unsupported mode must use the exact compositor")
		}
	}
}

func TestDrawImageFastNonOpaqueBackdrop(t *testing.T) {
	c := fastCanvas(4, 1)
	c.Pix[3] = 100
	c.DrawImage(spanImage([]uint8{128}), 0, 0, ModeMultiply)
	if c.Pix[3] == 100 && c.Pix[0] == 64 {
		t.Fatal("non-opaque backdrop must go through the exact compositor")
	}
}

func TestFastCompositesAsSourceOver(t *testing.T) {
	if !fastCompositesAsSourceOver(ModeSourceAtop) {
		t.Fatal("source-atop reduces to source-over on opaque backdrops")
	}
	if fastCompositesAsSourceOver(ModeMultiply) {
		t.Fatal("multiply needs a blend table")
	}
}

func TestScaleRotateEmptySource(t *testing.T) {
	empty := image.NewNRGBA(image.Rect(0, 0, 0, 0))
	if out := scaleRotateNRGBA(empty, 4, 0); out.Bounds().Dx() != 4 {
		t.Fatal("exact scaler must return blank dst for empty src")
	}
	if out := scaleRotateNRGBAFast(empty, 4, 0); out.Bounds().Dx() != 4 {
		t.Fatal("fast scaler must return blank dst for empty src")
	}
}

func TestRenderFastMode(t *testing.T) {
	p := tinyParams()
	p.SpritesEnabled = true
	p.CompositionModes = []CompositionMode{ModeMultiply, ModeSourceOver, ModeXor}
	p.Iterations = 60
	res, err := Render(RenderRequest{Params: p, Width: 32, Height: 32, Seed: 5, HasSeed: true, Fast: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Canvas == nil {
		t.Fatal("nil canvas")
	}
}

func TestCanvasNRGBASharesBuffer(t *testing.T) {
	c := NewCanvas(2, 2)
	c.Fill(9)
	img := c.NRGBA()
	if img.Pix[0] != 9 || img.Stride != 8 {
		t.Fatal("NRGBA view must share the pixel buffer")
	}
	img.Pix[0] = 77
	if c.Pix[0] != 77 {
		t.Fatal("NRGBA view must not copy")
	}
}
