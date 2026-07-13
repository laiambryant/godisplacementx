package gen

import (
	"bytes"
	"fmt"
	"image"
	"testing"

	xdraw "golang.org/x/image/draw"
)

// spriteSource builds an n×n straight-alpha image with hard shapes, AA-like
// soft edges, fully transparent regions and varied colour, mimicking the
// embedded sprite rasters.
func spriteSource(n int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			i := img.PixOffset(x, y)
			switch {
			case (x/8+y/8)%3 == 0:
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 255, 255, 255, 255
			case (x/8+y/8)%3 == 1:
				img.Pix[i] = uint8((x * 11) % 256)
				img.Pix[i+1] = uint8((y * 17) % 256)
				img.Pix[i+2] = uint8((x ^ y) % 256)
				img.Pix[i+3] = uint8((x*y)%254 + 1)
			default:
				// fully transparent with junk RGB, as PNG decoding can produce
			}
		}
	}
	return img
}

// renderReference is the pre-fusion implementation: x/image/draw bilinear
// scale into an NRGBA destination, then the standalone rotation pass.
func renderReference(base *image.NRGBA, size, angleDeg int) *image.NRGBA {
	scaled := image.NewNRGBA(image.Rect(0, 0, size, size))
	xdraw.ApproxBiLinear.Scale(scaled, scaled.Bounds(), base, base.Bounds(), xdraw.Src, nil)
	return rotateNRGBA(scaled, angleDeg)
}

// TestScaleRotateNRGBAMatchesXDraw pins the fused scaler against the x/image
// generic path byte for byte, across down/up-scales and all rotations.
func TestScaleRotateNRGBAMatchesXDraw(t *testing.T) {
	base := spriteSource(96)
	for _, size := range []int{17, 48, 95, 97, 200, 331} {
		for _, rot := range []int{0, 90, 180, 270} {
			got := scaleRotateNRGBA(base, size, rot)
			want := renderReference(base, size, rot)
			if !bytes.Equal(got.Pix, want.Pix) {
				for i := range got.Pix {
					if got.Pix[i] != want.Pix[i] {
						t.Fatalf("size=%d rot=%d: byte %d got=%d want=%d", size, rot, i, got.Pix[i], want.Pix[i])
					}
				}
			}
		}
	}
}

func BenchmarkSpriteRender(b *testing.B) {
	base := spriteSource(512)
	for _, size := range []int{256, 1024, 4096} {
		b.Run(fmt.Sprintf("%d", size), func(b *testing.B) {
			for b.Loop() {
				scaleRotateNRGBA(base, size, 90)
			}
		})
	}
}
