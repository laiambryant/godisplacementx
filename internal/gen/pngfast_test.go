package gen

import (
	"bytes"
	"fmt"
	"hash/adler32"
	"image"
	"image/png"
	"io"
	"math/rand/v2"
	"testing"
)

// TestAdlerCombineMatchesFullChecksum pins the stitched checksum against
// hash/adler32 over the concatenation, for assorted split points and sizes.
func TestAdlerCombineMatchesFullChecksum(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 11))
	for _, total := range []int{0, 1, 2, 5521, 65520, 65521, 400000} {
		data := make([]byte, total)
		for i := range data {
			data[i] = byte(rng.UintN(256))
		}
		want := adler32.Checksum(data)
		for _, cut := range []int{0, total / 3, total / 2, total} {
			a := adler32.Checksum(data[:cut])
			b := adler32.Checksum(data[cut:])
			if got := adlerCombine(a, b, total-cut); got != want {
				t.Fatalf("total=%d cut=%d: got %08x want %08x", total, cut, got, want)
			}
		}
	}
}

// canvasForRoundTrip builds a canvas with varied content; when translucent,
// a third of the pixels get alpha below 255 so the RGBA path is taken.
func canvasForRoundTrip(w, h int, translucent bool) *Canvas {
	c := NewCanvas(w, h)
	for i := 0; i < w*h; i++ {
		c.Pix[i*4] = uint8(i % 256)
		c.Pix[i*4+1] = uint8((i * 7) % 256)
		c.Pix[i*4+2] = uint8((i * 13) % 256)
		a := uint8(255)
		if translucent && i%3 == 0 {
			a = uint8((i*31)%255 + 0)
		}
		c.Pix[i*4+3] = a
	}
	return c
}

// TestEncodePNGRoundTrip decodes the parallel encoder's output with the
// stdlib decoder and requires pixel-exact equality with the canvas, across
// odd sizes, band boundaries, and both colour types.
func TestEncodePNGRoundTrip(t *testing.T) {
	sizes := [][2]int{{1, 1}, {3, 2}, {40, 40}, {257, 129}, {64, pngBandRows}, {33, pngBandRows + 1}, {100, pngBandRows*3 + 17}}
	for _, translucent := range []bool{false, true} {
		for _, size := range sizes {
			w, h := size[0], size[1]
			c := canvasForRoundTrip(w, h, translucent)

			var buf bytes.Buffer
			if err := EncodePNG(&buf, c); err != nil {
				t.Fatalf("%dx%d translucent=%v: encode: %v", w, h, translucent, err)
			}
			img, err := png.Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("%dx%d translucent=%v: decode: %v", w, h, translucent, err)
			}

			// The decoder yields straight-alpha *image.NRGBA for colour type
			// RGBA and *image.RGBA for opaque RGB; compare raw channel bytes
			// so low-alpha pixels are not laundered through premultiplication.
			for y := range h {
				for x := range w {
					i := (y*w + x) * 4
					var gotR, gotG, gotB, gotA uint8
					switch d := img.(type) {
					case *image.NRGBA:
						o := d.PixOffset(x, y)
						gotR, gotG, gotB, gotA = d.Pix[o], d.Pix[o+1], d.Pix[o+2], d.Pix[o+3]
					case *image.RGBA:
						o := d.PixOffset(x, y)
						gotR, gotG, gotB, gotA = d.Pix[o], d.Pix[o+1], d.Pix[o+2], d.Pix[o+3]
					default:
						t.Fatalf("unexpected decoded type %T", img)
					}
					if gotR != c.Pix[i] || gotG != c.Pix[i+1] || gotB != c.Pix[i+2] || gotA != c.Pix[i+3] {
						t.Fatalf("%dx%d translucent=%v (%d,%d): got %d,%d,%d,%d want %d,%d,%d,%d",
							w, h, translucent, x, y, gotR, gotG, gotB, gotA, c.Pix[i], c.Pix[i+1], c.Pix[i+2], c.Pix[i+3])
					}
				}
			}
		}
	}
}

// TestEncodePNGGeneratedImage round-trips a real generated map, whose byte
// patterns (flat runs, gradients, sprite edges) differ from synthetic ramps.
func TestEncodePNGGeneratedImage(t *testing.T) {
	c, err := Generate(Default(), 320, 320, NewRNG(4242))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := EncodePNG(&buf, c); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("expected opaque RGB decode to *image.RGBA, got %T", img)
	}
	for i := 0; i < 320*320; i++ {
		if rgba.Pix[i*4] != c.Pix[i*4] || rgba.Pix[i*4+1] != c.Pix[i*4+1] || rgba.Pix[i*4+2] != c.Pix[i*4+2] {
			t.Fatalf("pixel %d differs", i)
		}
	}
}

func BenchmarkEncodePNGParallel(b *testing.B) {
	for _, res := range []int{2048, 8192} {
		c, err := Generate(Default(), res, res, NewRNG(12345))
		if err != nil {
			b.Fatal(err)
		}
		b.Run(fmt.Sprintf("%d", res), func(b *testing.B) {
			b.SetBytes(int64(len(c.Pix)))
			for b.Loop() {
				if err := EncodePNG(io.Discard, c); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
