package gen

import (
	"bytes"
	"image/png"
	"testing"
)

func TestEncodePNGFastRoundTrip(t *testing.T) {
	for _, opaque := range []bool{true, false} {
		c := gradientCanvas(9, 5, opaque)
		var buf bytes.Buffer
		if err := EncodePNGFast(&buf, c); err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(&buf)
		if err != nil {
			t.Fatal(err)
		}
		n := toNRGBA(img)
		for i := 0; i < 9*5; i++ {
			for ch := 0; ch < 4; ch++ {
				if n.Pix[i*4+ch] != c.Pix[i*4+ch] {
					t.Fatalf("opaque=%v pixel %d ch %d: %d != %d", opaque, i, ch, n.Pix[i*4+ch], c.Pix[i*4+ch])
				}
			}
		}
	}
}

func TestEncodePNGMultiBand(t *testing.T) {
	c := gradientCanvas(8, pngBandRows*2+5, true)
	for _, fast := range []bool{false, true} {
		var buf bytes.Buffer
		var err error
		if fast {
			err = EncodePNGFast(&buf, c)
		} else {
			err = EncodePNG(&buf, c)
		}
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(&buf)
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dy() != pngBandRows*2+5 {
			t.Fatal("multi-band height mismatch")
		}
	}
}

func TestEncodePNGInvalidCanvas(t *testing.T) {
	if err := EncodePNG(&bytes.Buffer{}, NewCanvas(0, 0)); err == nil {
		t.Fatal("want error for empty canvas")
	}
}

func TestEncodePNGWriteErrors(t *testing.T) {
	c := gradientCanvas(4, 4, true)
	var full bytes.Buffer
	if err := EncodePNG(&full, c); err != nil {
		t.Fatal(err)
	}
	sawError := false
	for allowed := 0; allowed < 16; allowed++ {
		err := EncodePNG(&failAfterWriter{remaining: allowed}, c)
		if err == nil {
			if !sawError {
				t.Fatal("never hit a write error before success")
			}
			return
		}
		sawError = true
	}
	t.Fatal("encoder never completed within the write budget")
}
