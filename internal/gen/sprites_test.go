package gen

import (
	"image"
	"testing"
	"testing/fstest"
)

func resetSpriteCache() {
	spriteCache.mu.Lock()
	spriteCache.packs = map[SpritesPack][]*image.NRGBA{}
	spriteCache.mu.Unlock()
}

func withSpriteFS(t *testing.T, fsys fstest.MapFS) {
	t.Helper()
	old := spriteFS
	spriteFS = fsys
	resetSpriteCache()
	t.Cleanup(func() {
		spriteFS = old
		resetSpriteCache()
	})
}

func TestLoadSpritesAllPacks(t *testing.T) {
	s, err := LoadSprites(AllSpritePacks())
	if err != nil {
		t.Fatal(err)
	}
	want := 0
	for _, pack := range spritePackOrder {
		want += spritePackCounts[pack]
	}
	if s.Len() != want {
		t.Fatalf("Len = %d, want %d", s.Len(), want)
	}
}

func TestLoadSpritesReadError(t *testing.T) {
	withSpriteFS(t, fstest.MapFS{})
	if _, err := LoadSprites([]SpritesPack{PackClassic}); err == nil {
		t.Fatal("want read error for empty sprite FS")
	}
}

func TestLoadSpritesDecodeError(t *testing.T) {
	broken := fstest.MapFS{
		"assets/sprites_png/classic/1.png": &fstest.MapFile{Data: []byte("not a png")},
	}
	withSpriteFS(t, broken)
	if _, err := LoadSprites([]SpritesPack{PackClassic}); err == nil {
		t.Fatal("want decode error for corrupt sprite data")
	}
}

func TestSpriteSetRenderBounds(t *testing.T) {
	s, err := LoadSprites([]SpritesPack{PackClassic})
	if err != nil {
		t.Fatal(err)
	}
	if img := s.Render(-1, 64, 0, false); img != nil {
		t.Fatal("negative index must render nil")
	}
	if img := s.Render(s.Len(), 64, 0, false); img != nil {
		t.Fatal("out-of-range index must render nil")
	}
	if img := s.Render(0, 0, 0, false); img != nil {
		t.Fatal("zero size must render nil")
	}
}

func TestSpriteSetRenderVariants(t *testing.T) {
	s, err := LoadSprites([]SpritesPack{PackClassic})
	if err != nil {
		t.Fatal(err)
	}
	base := s.Render(0, spriteBaseSize, 0, false)
	if base == nil || base.Bounds().Dx() != spriteBaseSize {
		t.Fatal("base-size render must return the unscaled raster")
	}
	for _, angle := range []int{90, 180, 270, -90} {
		if img := s.Render(0, spriteBaseSize, angle, false); img == nil {
			t.Fatalf("rotation %d rendered nil", angle)
		}
	}
	if img := s.Render(0, 32, 90, false); img == nil || img.Bounds().Dx() != 32 {
		t.Fatal("exact scale render failed")
	}
	if img := s.Render(0, 32, 90, true); img == nil || img.Bounds().Dx() != 32 {
		t.Fatal("fast scale render failed")
	}
}

func TestRotateNRGBAUnhandledAngleCopies(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	src.Pix[0] = 7
	out := rotateNRGBA(src, 45)
	if out.Pix[0] != 7 {
		t.Fatal("non-multiple-of-90 angle must keep pixels in place")
	}
}

func TestToNRGBA(t *testing.T) {
	n := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	if toNRGBA(n) != n {
		t.Fatal("NRGBA input must pass through")
	}
	rgba := image.NewRGBA(image.Rect(0, 0, 1, 1))
	rgba.Pix[0], rgba.Pix[3] = 9, 255
	conv := toNRGBA(rgba)
	if conv.Pix[0] != 9 {
		t.Fatal("RGBA input must convert")
	}
}
