package gen

import (
	"bytes"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

type failAfterWriter struct {
	remaining int
}

func (f *failAfterWriter) Write(p []byte) (int, error) {
	if f.remaining <= 0 {
		return 0, errors.New("injected write failure")
	}
	f.remaining--
	return len(p), nil
}

func gradientCanvas(w, h int, opaque bool) *Canvas {
	c := NewCanvas(w, h)
	for i := 0; i < w*h; i++ {
		c.Pix[i*4+0] = uint8(i)
		c.Pix[i*4+1] = uint8(i * 3)
		c.Pix[i*4+2] = uint8(i * 7)
		c.Pix[i*4+3] = 255
	}
	if !opaque {
		c.Pix[3] = 128
	}
	return c
}

func TestEncodeRawRoundTrips(t *testing.T) {
	cases := []struct {
		name   string
		mode   OutputMode
		opaque bool
		format uint8
		bpp    int
	}{
		{"grayscale-l8", OutputGrayscale, true, rawFormatL8, 1},
		{"normal-rgb8", OutputNormal, true, rawFormatRGB8, 3},
		{"color-rgba8", OutputColor, false, rawFormatRGBA8, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := gradientCanvas(5, 3, tc.opaque)
			var buf bytes.Buffer
			if err := EncodeRaw(&buf, c, tc.mode); err != nil {
				t.Fatal(err)
			}
			w, h, format, payload, err := readRaw(&buf)
			if err != nil {
				t.Fatal(err)
			}
			if w != 5 || h != 3 || format != tc.format || len(payload) != 5*3*tc.bpp {
				t.Fatalf("round trip = %dx%d format %d len %d", w, h, format, len(payload))
			}
			switch tc.format {
			case rawFormatL8:
				if payload[6] != c.Pix[6*4] {
					t.Fatal("L8 payload must carry the red channel")
				}
			case rawFormatRGB8:
				if payload[6*3] != c.Pix[6*4] || payload[6*3+2] != c.Pix[6*4+2] {
					t.Fatal("RGB8 payload must carry RGB")
				}
			case rawFormatRGBA8:
				if payload[3] != 128 {
					t.Fatal("RGBA8 payload must carry alpha")
				}
			}
		})
	}
}

func TestEncodeRawInvalidCanvas(t *testing.T) {
	if err := EncodeRaw(&bytes.Buffer{}, NewCanvas(0, 0), OutputGrayscale); err == nil {
		t.Fatal("want error for empty canvas")
	}
}

func TestEncodeRawWriteErrors(t *testing.T) {
	c := gradientCanvas(4, 4, true)
	for _, allowed := range []int{0, 1} {
		if err := EncodeRaw(&failAfterWriter{remaining: allowed}, c, OutputGrayscale); err == nil {
			t.Fatalf("want error with %d writes allowed", allowed)
		}
	}
}

func TestReadRawErrors(t *testing.T) {
	if _, _, _, _, err := readRaw(bytes.NewReader(nil)); err == nil {
		t.Fatal("want error for empty stream")
	}
	bad := append([]byte("NOPE"), make([]byte, 12)...)
	if _, _, _, _, err := readRaw(bytes.NewReader(bad)); err == nil {
		t.Fatal("want error for bad magic")
	}
	badVersion := append([]byte(rawMagic), make([]byte, 12)...)
	badVersion[4] = 9
	if _, _, _, _, err := readRaw(bytes.NewReader(badVersion)); err == nil {
		t.Fatal("want error for bad version")
	}
	badFormat := append([]byte(rawMagic), make([]byte, 12)...)
	badFormat[4] = rawVersion
	badFormat[5] = 9
	if _, _, _, _, err := readRaw(bytes.NewReader(badFormat)); err == nil {
		t.Fatal("want error for bad format")
	}
}

func TestWriteRawFileCreateError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "no-such-dir", "x.gdxraw")
	if err := WriteRawFile(bad, gradientCanvas(2, 2, true), OutputGrayscale); err == nil {
		t.Fatal("want create error")
	}
}

func TestWriteMapFileDispatch(t *testing.T) {
	dir := t.TempDir()
	c := gradientCanvas(4, 4, true)

	raw := filepath.Join(dir, "OUT.GDXRAW")
	if err := WriteMapFile(raw, c, OutputGrayscale, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(raw)
	if err != nil || string(data[:4]) != rawMagic {
		t.Fatalf("raw dispatch failed: %v", err)
	}

	fastPNG := filepath.Join(dir, "fast.png")
	if err := WriteMapFile(fastPNG, c, OutputGrayscale, true); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(fastPNG)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 4 {
		t.Fatal("fast png must decode to the canvas size")
	}
}
