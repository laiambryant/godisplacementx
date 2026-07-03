package gen

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"

	xdraw "golang.org/x/image/draw"
)

// Sprites are pre-rasterized from the original SVGs into PNGs at spriteBaseSize
// (see tools/rasterize-sprites). This keeps the runtime pure Go and avoids the
// SVG rasterizer mishandling complex sprites (clipPath/use) or being slow.
//
//go:embed assets/sprites_png
var spritesFS embed.FS

// spriteBaseSize is the resolution of the embedded sprite PNGs. Draws scale this
// raster to the requested size.
const spriteBaseSize = 512

// SpriteSet is the flattened, ordered collection of sprite base images for the
// selected packs. The order matches getSprites() in store.ts: packs are added in
// the fixed order classic, bigdata, aggromaxx, crappack, regardless of selection
// order, each numbered 1..N.
type SpriteSet struct {
	base []*image.NRGBA
}

// LoadSprites decodes the embedded sprite PNGs for the selected packs.
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
		count := spritePackCounts[pack]
		for i := 1; i <= count; i++ {
			path := fmt.Sprintf("assets/sprites_png/%s/%d.png", pack, i)
			data, err := spritesFS.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read sprite %s: %w", path, err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, fmt.Errorf("decode sprite %s: %w", path, err)
			}
			s.base = append(s.base, toNRGBA(img))
		}
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
// 90). It scales the base raster rather than re-rasterizing.
func (s *SpriteSet) Render(index, size, angleDeg int) *image.NRGBA {
	if index < 0 || index >= len(s.base) || size <= 0 {
		return nil
	}
	base := s.base[index]

	var scaled *image.NRGBA
	if size == base.Bounds().Dx() {
		scaled = base
	} else {
		scaled = image.NewNRGBA(image.Rect(0, 0, size, size))
		xdraw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), base, base.Bounds(), xdraw.Src, nil)
	}
	return rotateNRGBA(scaled, angleDeg)
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
