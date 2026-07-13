package gen

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func tinyParams() Params {
	p := Default()
	p.Iterations = 20
	return p
}

func TestRenderInvalidSize(t *testing.T) {
	if _, err := Render(RenderRequest{Params: tinyParams(), Width: 0, Height: 16}); err == nil {
		t.Fatal("want error for zero width")
	}
}

func TestRenderPicksRandomSeed(t *testing.T) {
	res, err := Render(RenderRequest{Params: tinyParams(), Width: 16, Height: 16})
	if err != nil {
		t.Fatal(err)
	}
	if res.Canvas == nil {
		t.Fatal("nil canvas")
	}
}

func TestRenderModesAndInvert(t *testing.T) {
	for _, mode := range []OutputMode{OutputGrayscale, OutputNormal, OutputColor, ""} {
		res, err := Render(RenderRequest{
			Params: tinyParams(), Width: 16, Height: 16,
			Seed: 7, HasSeed: true, Mode: mode, Invert: true,
			Gradient: []ColorRGB{{R: 0}, {R: 255}},
		})
		if err != nil {
			t.Fatalf("mode %q: %v", mode, err)
		}
		if res.Seed != 7 {
			t.Fatalf("mode %q: seed = %d", mode, res.Seed)
		}
	}
}

func TestRenderColorDefaultGradient(t *testing.T) {
	if _, err := Render(RenderRequest{Params: tinyParams(), Width: 16, Height: 16, Seed: 1, HasSeed: true, Mode: OutputColor}); err != nil {
		t.Fatal(err)
	}
}

func TestRenderUnknownModeFails(t *testing.T) {
	if _, err := Render(RenderRequest{Params: tinyParams(), Width: 16, Height: 16, Seed: 1, HasSeed: true, Mode: "bogus"}); err == nil {
		t.Fatal("want error for unknown mode")
	}
}

func TestRenderGenerateError(t *testing.T) {
	withSpriteFS(t, fstest.MapFS{})
	p := tinyParams()
	p.SpritesEnabled = true
	if _, err := Render(RenderRequest{Params: p, Width: 16, Height: 16, Seed: 1, HasSeed: true}); err == nil {
		t.Fatal("want sprite load error")
	}
}

func TestRenderBundleInvalid(t *testing.T) {
	if err := RenderBundle(tinyParams(), 0, 16, false, nil, []EmitSpec{{Mode: OutputGrayscale, Path: "x.png"}}, false); err == nil {
		t.Fatal("want error for zero width")
	}
	if err := RenderBundle(tinyParams(), 16, 16, false, nil, nil, false); err == nil {
		t.Fatal("want error for no emits")
	}
}

func TestRenderBundleWritesAllModes(t *testing.T) {
	dir := t.TempDir()
	emits := []EmitSpec{
		{Mode: OutputGrayscale, Path: filepath.Join(dir, "h.png"), Seed: 1},
		{Mode: OutputGrayscale, Path: filepath.Join(dir, "h.gdxraw"), Seed: 1},
		{Mode: OutputColor, Path: filepath.Join(dir, "c.png"), Seed: 1},
		{Mode: OutputNormal, Path: filepath.Join(dir, "n.png"), Seed: 2},
	}
	if err := RenderBundle(tinyParams(), 16, 16, false, []ColorRGB{{R: 1}, {G: 2}}, emits, false); err != nil {
		t.Fatal(err)
	}
	for _, e := range emits {
		if _, err := os.Stat(e.Path); err != nil {
			t.Fatalf("missing output %s: %v", e.Path, err)
		}
	}
}

func TestRenderBundleInvertClonesGrayscale(t *testing.T) {
	dir := t.TempDir()
	emits := []EmitSpec{{Mode: OutputGrayscale, Path: filepath.Join(dir, "i.png"), Seed: 3}}
	if err := RenderBundle(tinyParams(), 16, 16, true, nil, emits, false); err != nil {
		t.Fatal(err)
	}
}

func TestRenderBundleGenerateError(t *testing.T) {
	withSpriteFS(t, fstest.MapFS{})
	p := tinyParams()
	p.SpritesEnabled = true
	err := RenderBundle(p, 16, 16, false, nil, []EmitSpec{{Mode: OutputGrayscale, Path: filepath.Join(t.TempDir(), "x.png")}}, false)
	if err == nil {
		t.Fatal("want sprite load error")
	}
}

func TestRenderBundleWriteError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "missing-dir", "x.png")
	err := RenderBundle(tinyParams(), 16, 16, false, nil, []EmitSpec{{Mode: OutputGrayscale, Path: bad}}, false)
	if err == nil {
		t.Fatal("want write error")
	}
}

func TestWriteEmitBadMode(t *testing.T) {
	c := NewCanvas(4, 4)
	if err := writeEmit(c, EmitSpec{Mode: "bogus", Path: "x.png"}, true, nil, false); err == nil {
		t.Fatal("want error for unknown mode")
	}
}

func TestApplyModeUnknown(t *testing.T) {
	if err := applyMode(NewCanvas(2, 2), "nope", nil); err == nil {
		t.Fatal("want error")
	}
}

func TestRandomSeedVaries(t *testing.T) {
	a, b, c := RandomSeed(), RandomSeed(), RandomSeed()
	if a == b && b == c {
		t.Fatal("three identical random seeds in a row")
	}
}

func TestNewRandomRNGVaries(t *testing.T) {
	a := NewRandomRNG().Float()
	b := NewRandomRNG().Float()
	c := NewRandomRNG().Float()
	if a == b && b == c {
		t.Fatal("three identical random streams in a row")
	}
}

type failingWriteCloser struct {
	writeErr error
	closeErr error
}

func (f *failingWriteCloser) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func (f *failingWriteCloser) Close() error { return f.closeErr }

func TestWritePNGFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.png")
	if err := WritePNGFile(path, NewCanvas(8, 8)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWritePNGFileCreateError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "no-such-dir", "out.png")
	if err := WritePNGFile(bad, NewCanvas(4, 4)); err == nil {
		t.Fatal("want create error")
	}
}

func TestEncodeBufferedEncodeError(t *testing.T) {
	boom := errors.New("boom")
	err := encodeBuffered(&failingWriteCloser{}, NewCanvas(2, 2), func(io.Writer, *Canvas) error { return boom })
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}

func TestEncodeBufferedFlushError(t *testing.T) {
	sink := &failingWriteCloser{writeErr: errors.New("disk full")}
	err := encodeBuffered(sink, NewCanvas(2, 2), func(w io.Writer, c *Canvas) error {
		_, _ = w.Write([]byte("data"))
		return nil
	})
	if err == nil {
		t.Fatal("want flush error")
	}
}

func TestEncodeBufferedCloseError(t *testing.T) {
	sink := &failingWriteCloser{closeErr: errors.New("close failed")}
	err := encodeBuffered(sink, NewCanvas(2, 2), func(io.Writer, *Canvas) error { return nil })
	if err == nil {
		t.Fatal("want close error")
	}
}
