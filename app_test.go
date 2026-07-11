//go:build bindings

// These tests exercise the GUI's htmx HTTP layer (gui_handler.go). They are
// tagged `bindings` because that build tag pulls in the desktop-only handler.
package main

import (
	"bytes"
	"encoding/base64"
	"html"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func get(t *testing.T, h http.Handler, target string) string {
	t.Helper()
	return doForm(t, h, "GET", target, nil)
}

func doForm(t *testing.T, h http.Handler, method, target string, form url.Values) string {
	t.Helper()
	var body *strings.Reader
	req := httptest.NewRequest(method, target, nil)
	if form != nil {
		body = strings.NewReader(form.Encode())
		req = httptest.NewRequest(method, target, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%s %s => %d: %s", method, target, rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

func mustContain(t *testing.T, body, sub, label string) {
	t.Helper()
	if !strings.Contains(body, sub) {
		t.Fatalf("%s: expected to contain %q\n---\n%s", label, sub, body)
	}
}

// extractDataURL pulls the first data:image/png;base64 URL out of an HTML body.
// html/template entity-escapes '+' inside attributes (a browser un-escapes it
// when parsing), so the attribute text must be unescaped before decoding.
func extractDataURL(t *testing.T, body string) []byte {
	t.Helper()
	const prefix = "data:image/png;base64,"
	i := strings.Index(body, prefix)
	if i < 0 {
		t.Fatalf("no PNG data URL in body:\n%s", body)
	}
	rest := body[i+len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		t.Fatal("unterminated data URL")
	}
	raw, err := base64.StdEncoding.DecodeString(html.UnescapeString(rest[:end]))
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	return raw
}

// rectOnly is a fast render: one cheap layer at the smallest resolution.
func rectOnly(action, seed, mode string) url.Values {
	return url.Values{
		"action":      {action},
		"seed":        {seed},
		"mode":        {mode},
		"invert":      {"false"},
		"resolution":  {"1024"},
		"iterations":  {"40"},
		"rectEnabled": {"on"},
	}
}

func TestControlsRenderAllStrips(t *testing.T) {
	h := (&App{}).routes()
	body := get(t, h, "/api/controls")
	for _, want := range []string{
		`id="strip-basics"`, `id="strip-rect"`, `id="strip-grid"`,
		`id="strip-cols"`, `id="strip-rows"`, `id="strip-lines"`,
		`id="strip-sprites"`, `id="strip-other"`,
		"Iterations", `name="rectBrightnessMin"`, `name="rectBrightnessMax"`,
		"source-over",
	} {
		mustContain(t, body, want, "controls")
	}
}

func TestVersionEndpoint(t *testing.T) {
	mustContain(t, get(t, (&App{}).routes(), "/api/version"), "v0.1.0", "version")
}

func TestMonitorEmptyState(t *testing.T) {
	body := get(t, (&App{}).routes(), "/api/monitor")
	mustContain(t, body, "No field acquired", "empty monitor")
	mustContain(t, body, `value="2048" checked`, "default resolution")
}

func TestRenderProducesPNGOfRequestedSize(t *testing.T) {
	h := (&App{}).routes()
	body := doForm(t, h, "POST", "/api/render", rectOnly("render", "", "grayscale"))
	mustContain(t, body, `class="v seed"`, "seed readout")
	img, err := png.Decode(bytes.NewReader(extractDataURL(t, body)))
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1024 || b.Dy() != 1024 {
		t.Errorf("image is %dx%d, want 1024x1024", b.Dx(), b.Dy())
	}
}

func TestRenderActionAssignsRandomSeed(t *testing.T) {
	h := (&App{}).routes()
	body := doForm(t, h, "POST", "/api/render", rectOnly("render", "", "grayscale"))
	if strings.Contains(body, `name="seed" value=""`) {
		t.Error("render action did not assign a seed")
	}
}

func TestSameSeedIsReproducible(t *testing.T) {
	h := (&App{}).routes()
	// action=invert reuses the posted seed instead of rerolling it.
	form := rectOnly("invert", "7", "grayscale")
	a := doForm(t, h, "POST", "/api/render", form)
	b := doForm(t, h, "POST", "/api/render", form)
	mustContain(t, a, `name="seed" value="7"`, "seed passthrough")
	// Compare the rendered PNGs, not the whole body: the monitor also reports
	// the wall-clock render duration, which legitimately varies between runs.
	if !bytes.Equal(extractDataURL(t, a), extractDataURL(t, b)) {
		t.Error("same seed produced different output")
	}
}

func TestRenderModes(t *testing.T) {
	h := (&App{}).routes()
	cases := []struct{ action, fromMode string }{
		{"normal", "grayscale"},
		{"color", "grayscale"},
	}
	for _, c := range cases {
		form := rectOnly(c.action, "3", c.fromMode)
		form["gradient"] = []string{"#00ffff", "#9500ff", "#ffe500"}
		extractDataURL(t, doForm(t, h, "POST", "/api/render", form)) // must be a valid PNG
	}
}

func TestRandomizeAllAndPerLayer(t *testing.T) {
	h := (&App{}).routes()
	mustContain(t, doForm(t, h, "POST", "/api/randomize", nil), `id="strip-rect"`, "randomize all")
	one := doForm(t, h, "POST", "/api/randomize?layer=rect", url.Values{"rectEnabled": {"on"}})
	mustContain(t, one, `id="strip-rect"`, "randomize layer")
	mustContain(t, one, "checked", "per-layer randomize keeps enabled state")
}

func TestGradientAddAndRemove(t *testing.T) {
	h := (&App{}).routes()
	add := doForm(t, h, "POST", "/api/gradient?op=add", url.Values{"gradient": {"#00ffff", "#9500ff"}})
	if n := strings.Count(add, `type="color"`); n != 3 {
		t.Fatalf("after add: want 3 stops, got %d", n)
	}
	rm := doForm(t, h, "POST", "/api/gradient?op=remove&i=0", url.Values{"gradient": {"#00ffff", "#9500ff", "#ffe500"}})
	if n := strings.Count(rm, `type="color"`); n != 2 {
		t.Fatalf("after remove: want 2 stops, got %d", n)
	}
}
