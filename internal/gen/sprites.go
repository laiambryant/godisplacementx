package gen

import (
	"embed"
	"image"
	"image/draw"
	"io/fs"
)

// Sprites are pre-rasterized from the original SVGs into PNGs at spriteBaseSize
// (see tools/rasterize-sprites). This keeps the runtime pure Go and avoids the
// SVG rasterizer mishandling complex sprites (clipPath/use) or being slow.
//
//go:embed assets/sprites_png
var embeddedSprites embed.FS

// spriteFS is the sprite source; a var so tests can inject a broken filesystem
// to exercise the load error paths.
var spriteFS fs.FS = embeddedSprites

// spriteBaseSize is the resolution of the embedded sprite PNGs. Draws scale this
// raster to the requested size.
const spriteBaseSize = 512

// SpriteSet is the flattened, ordered collection of sprite base images for the
// selected packs. The order matches getSprites() in store.ts: packs are added in
// the fixed order classic, bigdata, aggromaxx, crappack, regardless of selection
// order, each numbered 1..N. The base images are shared with the process-wide
// decode cache and must never be mutated.
type SpriteSet struct {
	base []*image.NRGBA
}

// LoadSprites assembles the sprite set for the selected packs from the
// process-wide decode cache (spritecache.go); packs keep the fixed canonical
// order regardless of selection order.
func LoadSprites(packs []SpritesPack) (*SpriteSet, error) {
	selected := map[SpritesPack]bool{}
	for _, p := range packs {
		selected[p] = true
	}
	s := &SpriteSet{}
	for _, pack := range spritePackOrder {
		if !selected[pack] {
			continue
		}
		base, err := loadSpritePack(pack)
		if err != nil {
			return nil, err
		}
		s.base = append(s.base, base...)
	}
	return s, nil
}

func toNRGBA(img image.Image) *image.NRGBA {
	if n, ok := img.(*image.NRGBA); ok {
		return n
	}
	n := image.NewNRGBA(img.Bounds())
	draw.Draw(n, n.Bounds(), img, img.Bounds().Min, draw.Src)
	return n
}

// Len returns the number of sprites available.
func (s *SpriteSet) Len() int { return len(s.base) }

// Render produces sprite index at size×size, rotated by angleDeg (a multiple of
// 90). It scales the base raster rather than re-rasterizing; scaling and
// rotation run as one fused pass (spritescale.go, or the fixed-point
// spritescale_fast.go variant when fast is set).
func (s *SpriteSet) Render(index, size, angleDeg int, fast bool) *image.NRGBA {
	if index < 0 || index >= len(s.base) || size <= 0 {
		return nil
	}
	base := s.base[index]
	if size == base.Bounds().Dx() {
		return rotateNRGBA(base, angleDeg)
	}
	rot := ((angleDeg % 360) + 360) % 360
	if fast {
		return scaleRotateNRGBAFast(base, size, rot)
	}
	return scaleRotateNRGBA(base, size, rot)
}

// rotateNRGBA rotates a square image clockwise by a multiple of 90 degrees.
func rotateNRGBA(src *image.NRGBA, angleDeg int) *image.NRGBA {
	a := ((angleDeg % 360) + 360) % 360
	if a == 0 {
		return src
	}
	n := src.Bounds().Dx()
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	for oy := 0; oy < n; oy++ {
		for ox := 0; ox < n; ox++ {
			var sx, sy int
			switch a {
			case 90: // clockwise
				sx, sy = oy, n-1-ox
			case 180:
				sx, sy = n-1-ox, n-1-oy
			case 270:
				sx, sy = n-1-oy, ox
			default:
				sx, sy = ox, oy
			}
			si := src.PixOffset(sx, sy)
			di := dst.PixOffset(ox, oy)
			copy(dst.Pix[di:di+4], src.Pix[si:si+4])
		}
	}
	return dst
}
