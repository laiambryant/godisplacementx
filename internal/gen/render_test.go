package gen

import (
	"bytes"
	"image"
	_ "image/png" // register the PNG decoder for image.Decode
	"os"
	"path/filepath"
	"testing"
)

// decodePNG loads a PNG file and returns its bounds, failing the test on error.
func decodePNG(t *testing.T, path string) image.Image {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return img
}

func TestRenderBundleWritesAllOutputs(t *testing.T) {
	dir := t.TempDir()
	gray := filepath.Join(dir, "height.png")
	color := filepath.Join(dir, "albedo.png")
	normal := filepath.Join(dir, "normal.png")

	const seed = uint64(12345)
	emits := []EmitSpec{
		{Mode: OutputGrayscale, Path: gray, Seed: seed},
		{Mode: OutputColor, Path: color, Seed: seed},
		{Mode: OutputNormal, Path: normal, Seed: seed},
	}
	if err := RenderBundle(Default(), 48, 48, false, DefaultGradient(), emits); err != nil {
		t.Fatalf("RenderBundle: %v", err)
	}

	for _, p := range []string{gray, color, normal} {
		img := decodePNG(t, p)
		if img.Bounds().Dx() != 48 || img.Bounds().Dy() != 48 {
			t.Errorf("%s has bounds %v, want 48x48", p, img.Bounds())
		}
	}

	// Color post-processing must change pixels relative to the raw grayscale field.
	if bytes.Equal(readFile(t, gray), readFile(t, color)) {
		t.Errorf("color output is identical to grayscale; post-processing not applied")
	}
}

func TestRenderBundleSeedReuseAndIndependence(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.png")
	c := filepath.Join(dir, "c.png")

	// a and b share a seed (must be identical); c uses a different seed.
	emits := []EmitSpec{
		{Mode: OutputGrayscale, Path: a, Seed: 7},
		{Mode: OutputGrayscale, Path: b, Seed: 7},
		{Mode: OutputGrayscale, Path: c, Seed: 8},
	}
	if err := RenderBundle(Default(), 48, 48, false, nil, emits); err != nil {
		t.Fatalf("RenderBundle: %v", err)
	}

	if !bytes.Equal(readFile(t, a), readFile(t, b)) {
		t.Errorf("same-seed grayscale outputs differ; the height field was not reused deterministically")
	}
	if bytes.Equal(readFile(t, a), readFile(t, c)) {
		t.Errorf("different-seed grayscale outputs are identical; seeds are not independent")
	}
}

func TestRenderBundleRejectsBadInput(t *testing.T) {
	if err := RenderBundle(Default(), 0, 48, false, nil, []EmitSpec{{Mode: OutputGrayscale, Path: "x.png"}}); err == nil {
		t.Errorf("expected error for invalid size")
	}
	if err := RenderBundle(Default(), 48, 48, false, nil, nil); err == nil {
		t.Errorf("expected error for empty emits")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
