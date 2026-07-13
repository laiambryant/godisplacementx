package cli

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func runCLI(t *testing.T, args ...string) error {
	t.Helper()
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

func TestExecuteVersion(t *testing.T) {
	old := os.Args
	os.Args = []string{"godisplacementx", "version"}
	t.Cleanup(func() { os.Args = old })
	if err := Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestRandomizeToStdout(t *testing.T) {
	if err := runCLI(t, "randomize", "--seed", "7"); err != nil {
		t.Fatal(err)
	}
}

func TestRandomizeToFile(t *testing.T) {
	out := filepath.Join(t.TempDir(), "params.json")
	if err := runCLI(t, "randomize", "--seed", "7", "-o", out); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil || len(data) == 0 {
		t.Fatalf("config not written: %v", err)
	}
}

func TestRandomizeWriteError(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "no-dir", "params.json")
	if err := runCLI(t, "randomize", "-o", bad); err == nil {
		t.Fatal("want write error")
	}
}

func TestGenerateWithEveryOverride(t *testing.T) {
	out := filepath.Join(t.TempDir(), "map.png")
	err := runCLI(t, "generate",
		"-o", out, "--width", "16", "--height", "16", "--seed", "5",
		"--iterations", "15", "--background", "10",
		"--rect=false", "--grid=true", "--cols=false", "--rows=true",
		"--lines=true", "--sprites=false", "--sprites-rotation=false",
		"--seamless", "--sprite-packs", "classic,bigdata,",
		"--composition-modes", "multiply,screen,", "--mode", "grayscale", "--invert")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateDefaultOutName(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := runCLI(t, "generate", "--width", "16", "--height", "16", "--seed", "1"); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob("DisplacementX_16x16_*.png")
	if err != nil || len(matches) != 1 {
		t.Fatalf("default-named output missing: %v %v", matches, err)
	}
}

func TestGenerateRandomized(t *testing.T) {
	out := filepath.Join(t.TempDir(), "rand.png")
	if err := runCLI(t, "generate", "-o", out, "--width", "16", "--height", "16", "--seed", "3", "--randomize", "--iterations", "12"); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(cfg, []byte(`{"iterations":12}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "map.png")
	if err := runCLI(t, "generate", "--config", cfg, "-o", out, "--width", "16", "--height", "16", "--seed", "2"); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateErrors(t *testing.T) {
	dir := t.TempDir()
	badJSON := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(badJSON, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := [][]string{
		{"generate", "--config", filepath.Join(dir, "missing.json")},
		{"generate", "--config", badJSON},
		{"generate", "--gradient", "#zzzzzz", "--width", "16", "--height", "16"},
		{"generate", "--mode", "bogus", "--width", "16", "--height", "16", "--seed", "1"},
		{"generate", "--resolution", "0", "--seed", "1"},
		{"generate", "--sprite-packs", "bogus", "--width", "16", "--height", "16"},
		{"generate", "--composition-modes", "bogus", "--width", "16", "--height", "16"},
		{"generate", "-o", filepath.Join(dir, "no-dir", "x.png"), "--width", "16", "--height", "16", "--seed", "1"},
	}
	for _, args := range cases {
		if err := runCLI(t, args...); err == nil {
			t.Fatalf("want error for %v", args)
		}
	}
}

func TestGenerateColorWithGradient(t *testing.T) {
	out := filepath.Join(t.TempDir(), "c.png")
	if err := runCLI(t, "generate", "-o", out, "--width", "16", "--height", "16", "--seed", "4",
		"--mode", "color", "--gradient", "#0f0, #9500ff ,#ffe500"); err != nil {
		t.Fatal(err)
	}
}

func TestBundleHappyPath(t *testing.T) {
	dir := t.TempDir()
	err := runCLI(t, "bundle",
		"--width", "16", "--height", "16", "--seed", "9", "--invert",
		"--gradient", "#000000,#ffffff",
		"--emit", "grayscale:"+filepath.Join(dir, "h.gdxraw"),
		"--emit", "grayscale:123:"+filepath.Join(dir, "r.png"),
		"--emit", "color:"+filepath.Join(dir, "albedo.png"),
		"--emit", "normal:"+filepath.Join(dir, "normal.png"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"h.gdxraw", "r.png", "albedo.png", "normal.png"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestBundleFromConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "cfg.json")
	if err := os.WriteFile(cfg, []byte(`{"iterations":12}`), 0o644); err != nil {
		t.Fatal(err)
	}
	err := runCLI(t, "bundle", "--config", cfg, "--width", "16", "--height", "16",
		"--emit", "grayscale:"+filepath.Join(dir, "h.png"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestBundleErrors(t *testing.T) {
	dir := t.TempDir()
	cases := [][]string{
		{"bundle", "--width", "16", "--height", "16"},
		{"bundle", "--config", filepath.Join(dir, "missing.json"), "--emit", "grayscale:x.png"},
		{"bundle", "--gradient", "#nope", "--emit", "grayscale:x.png"},
		{"bundle", "--width", "16", "--height", "16", "--emit", "grayscale"},
		{"bundle", "--width", "16", "--height", "16", "--emit", "bogus:x.png"},
		{"bundle", "--width", "16", "--height", "16", "--emit", "grayscale:"},
		{"bundle", "--width", "16", "--height", "16", "--emit", "grayscale:123:"},
	}
	for _, args := range cases {
		if err := runCLI(t, args...); err == nil {
			t.Fatalf("want error for %v", args)
		}
	}
}

func TestParseEmitWindowsPathKeepsDefaultSeed(t *testing.T) {
	e, err := parseEmit(`grayscale:C:\maps\out.png`, 42)
	if err != nil {
		t.Fatal(err)
	}
	if e.Seed != 42 || e.Path != `C:\maps\out.png` {
		t.Fatalf("emit = %+v", e)
	}
}

func TestParseHexColorForms(t *testing.T) {
	if _, err := parseHexColor("#ffff"); err == nil {
		t.Fatal("want error for 4-digit colour")
	}
	c, err := parseHexColor("0f0")
	if err != nil || c.G != 255 {
		t.Fatalf("short form = %+v, %v", c, err)
	}
}
