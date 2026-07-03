//go:build desktop || bindings

package main

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"godisplacementx/internal/cli"
	"godisplacementx/internal/gen"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed frontend/templates/templates.html
var templatesSrc string

var tpl = template.Must(template.New("gui").Parse(templatesSrc))

var resolutions = []int{1024, 2048, 4096, 8192}

// routes wires the htmx endpoints. Wails serves embedded static files first and
// falls through to this handler for anything it can't find (and for every POST).
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/version", a.handleVersion)
	mux.HandleFunc("/api/controls", a.handleControls)
	mux.HandleFunc("/api/monitor", a.handleMonitor)
	mux.HandleFunc("/api/render", a.handleRender)
	mux.HandleFunc("/api/randomize", a.handleRandomize)
	mux.HandleFunc("/api/gradient", a.handleGradient)
	mux.HandleFunc("/api/save", a.handleSave)
	return mux
}

// ---- view models -------------------------------------------------------------

// Fader is one parameter control: a single slider (Val) or a dual-range slider
// (Lo/Hi). Name is the form field; dual sliders post NameMin and NameMax.
type Fader struct {
	Label          string
	Name           string
	Dual           bool
	Min, Max, Step int
	Val            int
	Lo, Hi         int
}

// ChipItem is one toggle in a checkbox group (sprite pack / composition mode).
type ChipItem struct {
	Label, Value string
	Checked      bool
}

// Strip is one channel strip in the rack.
type Strip struct {
	ID         string
	Title      string
	Kind       string // faders | basics | sprites | other
	HasToggle  bool
	EnableName string
	Enabled    bool
	RandLayer  string // "" hides the per-strip randomize button
	Faders     []Fader
	SeamlessOn bool
	Packs      []ChipItem
	RotateOn   bool
	Modes      []ChipItem
}

// RackView is the data for the whole settings rack.
type RackView struct{ Strips []Strip }

// GradientView drives the color-gradient editor.
type GradientView struct {
	Stops     []string
	BarCSS    template.CSS
	CanRemove bool
	CanAdd    bool
}

// MonitorView drives the output monitor fragment.
type MonitorView struct {
	HasImage    bool
	PNG         template.URL
	SeedDec     string
	SeedHex     string
	DurationMS  int64
	Width       int
	Height      int
	Mode        string
	Invert      bool
	Resolution  int
	Resolutions []int
	Err         string
	Gradient    GradientView
}

// ---- handlers ----------------------------------------------------------------

func (a *App) handleVersion(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprint(w, "v"+cli.Version)
}

func (a *App) handleControls(w http.ResponseWriter, _ *http.Request) {
	renderTemplate(w, "rack", buildRack(gen.Default()))
}

func (a *App) handleMonitor(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	renderTemplate(w, "monitor", emptyMonitor(resolutionOf(r.Form), gradientView(parseGradient(r.Form))))
}

func (a *App) handleRender(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	form := r.Form

	params := parseParams(form)
	res := resolutionOf(form)
	stops := parseGradient(form)
	gv := gradientView(stops)

	seedStr, mode, invert := transition(form)

	var seed uint64
	hasSeed := false
	if seedStr != "" {
		if v, err := strconv.ParseUint(seedStr, 10, 64); err == nil {
			seed, hasSeed = v, true
		}
	}

	result, err := gen.Render(gen.RenderRequest{
		Params:   params,
		Width:    res,
		Height:   res,
		Seed:     seed,
		HasSeed:  hasSeed,
		Mode:     gen.OutputMode(mode),
		Invert:   invert,
		Gradient: stops,
	})
	if err != nil {
		renderTemplate(w, "monitor", MonitorView{
			Mode: mode, Invert: invert, SeedDec: seedStr,
			Resolution: res, Resolutions: resolutions,
			Err: err.Error(), Gradient: gv,
		})
		return
	}

	var buf bytes.Buffer
	if err := gen.EncodePNG(&buf, result.Canvas); err != nil {
		renderTemplate(w, "monitor", MonitorView{
			Mode: mode, Invert: invert, Resolution: res, Resolutions: resolutions,
			Err: err.Error(), Gradient: gv,
		})
		return
	}

	renderTemplate(w, "monitor", MonitorView{
		HasImage:    true,
		PNG:         template.URL("data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())),
		SeedDec:     strconv.FormatUint(result.Seed, 10),
		SeedHex:     fmt.Sprintf("0x%X", result.Seed),
		DurationMS:  result.DurationMS,
		Width:       res,
		Height:      res,
		Mode:        mode,
		Invert:      invert,
		Resolution:  res,
		Resolutions: resolutions,
		Gradient:    gv,
	})
}

func (a *App) handleRandomize(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	layer := r.URL.Query().Get("layer")
	g := gen.NewRandomRNG()

	if layer == "" { // randomize everything
		p := gen.Default()
		p.Randomize(g)
		renderTemplate(w, "rack", buildRack(p))
		return
	}

	// Per-layer: keep every other control's posted value, reroll just this one.
	p := parseParams(r.Form)
	randomizeLayer(&p, layer, g)
	renderTemplate(w, "strip", buildStrip(p, layer))
}

func (a *App) handleGradient(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	stops := parseGradient(r.Form)
	if len(stops) < 2 {
		stops = gen.DefaultGradient()
	}

	switch r.URL.Query().Get("op") {
	case "add":
		if len(stops) < 20 {
			stops = append(stops, gen.ColorRGB{})
		}
	case "remove":
		if i, err := strconv.Atoi(r.URL.Query().Get("i")); err == nil && len(stops) > 2 && i >= 0 && i < len(stops) {
			stops = append(stops[:i], stops[i+1:]...)
		}
	case "randomize":
		g := gen.NewRandomRNG()
		for i := range stops {
			stops[i] = randomColor(g)
		}
	}

	renderTemplate(w, "gradient", gradientView(stops))
}

func (a *App) handleSave(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	dataURL := r.Form.Get("pngData")
	b64 := dataURL
	if i := strings.Index(b64, ","); i >= 0 {
		b64 = b64[i+1:]
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(data) == 0 {
		fmt.Fprint(w, `<span class="err">Nothing to save yet</span>`)
		return
	}

	res := resolutionOf(r.Form)
	name := fmt.Sprintf("godisplacementx_%dx%d_%s", res, res, time.Now().Format("2006-01-02-150405"))

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename:      name,
		CanCreateDirectories: true,
		Filters:              []runtime.FileFilter{{DisplayName: "PNG Image (*.png)", Pattern: "*.png"}},
	})
	if err != nil {
		fmt.Fprintf(w, `<span class="err">%s</span>`, template.HTMLEscapeString(err.Error()))
		return
	}
	if path == "" {
		fmt.Fprint(w, `<span class="muted">save cancelled</span>`)
		return
	}
	if !strings.HasSuffix(strings.ToLower(path), ".png") {
		path += ".png"
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fmt.Fprintf(w, `<span class="err">%s</span>`, template.HTMLEscapeString(err.Error()))
		return
	}
	fmt.Fprint(w, `<span class="v">saved &#10003;</span>`)
}

// ---- view builders -----------------------------------------------------------

func buildRack(p gen.Params) RackView {
	return RackView{Strips: []Strip{
		buildStrip(p, "basics"),
		buildStrip(p, "rect"),
		buildStrip(p, "grid"),
		buildStrip(p, "cols"),
		buildStrip(p, "rows"),
		buildStrip(p, "lines"),
		buildStrip(p, "sprites"),
		buildStrip(p, "modes"),
	}}
}

func buildStrip(p gen.Params, key string) Strip {
	switch key {
	case "basics":
		return Strip{
			ID: "strip-basics", Title: "Basics", Kind: "basics",
			SeamlessOn: p.SeamlessTextureEnabled,
			Faders: []Fader{
				singleF("Iterations", "iterations", 10, 2000, 1, p.Iterations),
				singleF("Background brightness", "backgroundBrightness", 0, 255, 1, p.BackgroundBrightness),
			},
		}
	case "rect":
		return Strip{
			ID: "strip-rect", Title: "Rect", Kind: "faders", HasToggle: true,
			EnableName: "rectEnabled", Enabled: p.RectEnabled, RandLayer: "rect",
			Faders: []Fader{
				dualF("Brightness", "rectBrightness", 0, 255, 1, p.RectBrightness),
				dualF("Alpha", "rectAlpha", 0, 100, 1, p.RectAlpha),
				singleF("Scale", "rectScale", 20, 200, 1, p.RectScale),
			},
		}
	case "grid":
		return layerStrip("strip-grid", "Grid", "grid", p.GridEnabled,
			p.GridBrightness, p.GridAlpha, p.GridScale, p.GridAmount, p.GridGap)
	case "cols":
		return layerStrip("strip-cols", "Cols", "cols", p.ColsEnabled,
			p.ColsBrightness, p.ColsAlpha, p.ColsScale, p.ColsAmount, p.ColsGap)
	case "rows":
		return layerStrip("strip-rows", "Rows", "rows", p.RowsEnabled,
			p.RowsBrightness, p.RowsAlpha, p.RowsScale, p.RowsAmount, p.RowsGap)
	case "lines":
		return Strip{
			ID: "strip-lines", Title: "Lines", Kind: "faders", HasToggle: true,
			EnableName: "linesEnabled", Enabled: p.LinesEnabled, RandLayer: "lines",
			Faders: []Fader{
				dualF("Brightness", "linesBrightness", 0, 255, 1, p.LinesBrightness),
				dualF("Alpha", "linesAlpha", 0, 100, 1, p.LinesAlpha),
				dualF("Width", "linesWidth", 1, 50, 1, p.LinesWidth),
			},
		}
	case "sprites":
		return Strip{
			ID: "strip-sprites", Title: "Sprites", Kind: "sprites", HasToggle: true,
			EnableName: "spritesEnabled", Enabled: p.SpritesEnabled, RandLayer: "sprites",
			RotateOn: p.SpritesRotationEnabled,
			Packs: []ChipItem{
				{Label: "Classic", Value: "classic", Checked: hasPack(p.SpritesPacks, "classic")},
				{Label: "Big data", Value: "bigdata", Checked: hasPack(p.SpritesPacks, "bigdata")},
				{Label: "Aggromaxx", Value: "aggromaxx", Checked: hasPack(p.SpritesPacks, "aggromaxx")},
				{Label: "Crap pack", Value: "crappack", Checked: hasPack(p.SpritesPacks, "crappack")},
			},
		}
	default: // "modes"
		modes := make([]ChipItem, 0, len(gen.AllCompositionModes()))
		for _, m := range gen.AllCompositionModes() {
			modes = append(modes, ChipItem{Label: string(m), Value: string(m), Checked: hasMode(p.CompositionModes, m)})
		}
		return Strip{ID: "strip-other", Title: "Other", Kind: "other", RandLayer: "modes", Modes: modes}
	}
}

func layerStrip(id, title, key string, enabled bool, bright, alpha gen.Dual, scale int, amount gen.Dual, gap int) Strip {
	return Strip{
		ID: id, Title: title, Kind: "faders", HasToggle: true,
		EnableName: key + "Enabled", Enabled: enabled, RandLayer: key,
		Faders: []Fader{
			dualF("Brightness", key+"Brightness", 0, 255, 1, bright),
			dualF("Alpha", key+"Alpha", 0, 100, 1, alpha),
			singleF("Scale", key+"Scale", 20, 200, 1, scale),
			dualF("Amount", key+"Amount", 2, 10, 1, amount),
			singleF("Gap", key+"Gap", 10, 1000, 10, gap),
		},
	}
}

func singleF(label, name string, min, max, step, val int) Fader {
	return Fader{Label: label, Name: name, Min: min, Max: max, Step: step, Val: val}
}

func dualF(label, name string, min, max, step int, d gen.Dual) Fader {
	lo, hi := d[0], d[1]
	if lo > hi {
		lo, hi = hi, lo
	}
	return Fader{Label: label, Name: name, Dual: true, Min: min, Max: max, Step: step, Lo: lo, Hi: hi}
}

func emptyMonitor(res int, gv GradientView) MonitorView {
	return MonitorView{
		Mode: string(gen.OutputGrayscale), Resolution: res,
		Resolutions: resolutions, Gradient: gv,
	}
}

func gradientView(stops []gen.ColorRGB) GradientView {
	if len(stops) < 2 {
		stops = gen.DefaultGradient()
	}
	hexes := make([]string, len(stops))
	for i, c := range stops {
		hexes[i] = toHex(c)
	}
	bar := template.CSS(hexes[0])
	if len(hexes) > 1 {
		bar = template.CSS("linear-gradient(90deg, " + strings.Join(hexes, ", ") + ")")
	}
	return GradientView{Stops: hexes, BarCSS: bar, CanRemove: len(stops) > 2, CanAdd: len(stops) < 20}
}

// ---- request parsing ---------------------------------------------------------

// transition resolves the seed/mode/invert for a render from the requested
// action and the monitor's current (posted) state, mirroring the original
// preview toggles: a plain render rerolls everything; the toggles reuse the
// current seed.
func transition(form url.Values) (seed, mode string, invert bool) {
	curSeed := form.Get("seed")
	curMode := form.Get("mode")
	if curMode == "" {
		curMode = string(gen.OutputGrayscale)
	}
	curInvert := form.Get("invert") == "true"

	switch form.Get("action") {
	case "invert":
		return curSeed, curMode, !curInvert
	case "normal":
		if curMode == string(gen.OutputNormal) {
			return curSeed, string(gen.OutputGrayscale), curInvert
		}
		return curSeed, string(gen.OutputNormal), curInvert
	case "color":
		if curMode == string(gen.OutputColor) {
			return curSeed, string(gen.OutputGrayscale), curInvert
		}
		return curSeed, string(gen.OutputColor), curInvert
	default: // "render": fresh seed, plain grayscale
		return "", string(gen.OutputGrayscale), false
	}
}

func parseParams(f url.Values) gen.Params {
	p := gen.Default()

	p.Iterations = clamp(atoi(f.Get("iterations"), p.Iterations), 10, 2000)
	p.BackgroundBrightness = clamp(atoi(f.Get("backgroundBrightness"), p.BackgroundBrightness), 0, 255)
	p.SeamlessTextureEnabled = checked(f, "seamlessTextureEnabled")

	p.RectEnabled = checked(f, "rectEnabled")
	p.RectBrightness = dualOf(f, "rectBrightness", 0, 255)
	p.RectAlpha = dualOf(f, "rectAlpha", 0, 100)
	p.RectScale = clamp(atoi(f.Get("rectScale"), p.RectScale), 20, 200)

	p.GridEnabled = checked(f, "gridEnabled")
	p.GridBrightness = dualOf(f, "gridBrightness", 0, 255)
	p.GridAlpha = dualOf(f, "gridAlpha", 0, 100)
	p.GridScale = clamp(atoi(f.Get("gridScale"), p.GridScale), 20, 200)
	p.GridAmount = dualOf(f, "gridAmount", 2, 10)
	p.GridGap = clamp(atoi(f.Get("gridGap"), p.GridGap), 10, 1000)

	p.ColsEnabled = checked(f, "colsEnabled")
	p.ColsBrightness = dualOf(f, "colsBrightness", 0, 255)
	p.ColsAlpha = dualOf(f, "colsAlpha", 0, 100)
	p.ColsScale = clamp(atoi(f.Get("colsScale"), p.ColsScale), 20, 200)
	p.ColsAmount = dualOf(f, "colsAmount", 2, 10)
	p.ColsGap = clamp(atoi(f.Get("colsGap"), p.ColsGap), 10, 1000)

	p.RowsEnabled = checked(f, "rowsEnabled")
	p.RowsBrightness = dualOf(f, "rowsBrightness", 0, 255)
	p.RowsAlpha = dualOf(f, "rowsAlpha", 0, 100)
	p.RowsScale = clamp(atoi(f.Get("rowsScale"), p.RowsScale), 20, 200)
	p.RowsAmount = dualOf(f, "rowsAmount", 2, 10)
	p.RowsGap = clamp(atoi(f.Get("rowsGap"), p.RowsGap), 10, 1000)

	p.LinesEnabled = checked(f, "linesEnabled")
	p.LinesBrightness = dualOf(f, "linesBrightness", 0, 255)
	p.LinesAlpha = dualOf(f, "linesAlpha", 0, 100)
	p.LinesWidth = dualOf(f, "linesWidth", 1, 50)

	p.SpritesEnabled = checked(f, "spritesEnabled")
	p.SpritesPacks = parsePacks(f["spritesPacks"])
	p.SpritesRotationEnabled = checked(f, "spritesRotationEnabled")

	p.CompositionModes = parseModes(f["compositionModes"])
	return p
}

func parsePacks(values []string) []gen.SpritesPack {
	out := []gen.SpritesPack{}
	for _, p := range gen.AllSpritePacks() {
		for _, v := range values {
			if string(p) == v {
				out = append(out, p)
				break
			}
		}
	}
	return out
}

func parseModes(values []string) []gen.CompositionMode {
	out := []gen.CompositionMode{}
	for _, v := range values {
		if gen.IsValidCompositionMode(v) {
			out = append(out, gen.CompositionMode(v))
		}
	}
	return out
}

func parseGradient(f url.Values) []gen.ColorRGB {
	stops := []gen.ColorRGB{}
	for _, h := range f["gradient"] {
		if c, ok := parseHex(h); ok {
			stops = append(stops, c)
		}
	}
	return stops
}

func resolutionOf(f url.Values) int {
	r := atoi(f.Get("resolution"), 2048)
	for _, allowed := range resolutions {
		if r == allowed {
			return r
		}
	}
	return 2048
}

// ---- randomization (per layer) -----------------------------------------------

func randomizeLayer(p *gen.Params, layer string, g *gen.RNG) {
	switch layer {
	case "rect":
		p.RectBrightness = dualRand(g, 0, 255)
		p.RectAlpha = dualRand(g, 0, 100)
		p.RectScale = g.Integer(20, 200)
	case "grid":
		p.GridBrightness = dualRand(g, 0, 255)
		p.GridAlpha = dualRand(g, 0, 100)
		p.GridScale = g.Integer(20, 200)
		p.GridAmount = dualRand(g, 2, 10)
		p.GridGap = g.Integer(10, 1000)
	case "cols":
		p.ColsBrightness = dualRand(g, 0, 255)
		p.ColsAlpha = dualRand(g, 0, 100)
		p.ColsScale = g.Integer(20, 200)
		p.ColsAmount = dualRand(g, 2, 10)
		p.ColsGap = g.Integer(10, 1000)
	case "rows":
		p.RowsBrightness = dualRand(g, 0, 255)
		p.RowsAlpha = dualRand(g, 0, 100)
		p.RowsScale = g.Integer(20, 200)
		p.RowsAmount = dualRand(g, 2, 10)
		p.RowsGap = g.Integer(10, 1000)
	case "lines":
		p.LinesBrightness = dualRand(g, 0, 255)
		p.LinesAlpha = dualRand(g, 0, 100)
		p.LinesWidth = dualRand(g, 1, 50)
	case "sprites":
		packs := []gen.SpritesPack{}
		for _, pk := range gen.AllSpritePacks() {
			if g.Boolean() {
				packs = append(packs, pk)
			}
		}
		p.SpritesPacks = packs
		p.SpritesRotationEnabled = g.Boolean()
	case "modes":
		modes := []gen.CompositionMode{}
		for _, m := range gen.AllCompositionModes() {
			if g.Boolean() {
				modes = append(modes, m)
			}
		}
		p.CompositionModes = modes
	}
}

func dualRand(g *gen.RNG, min, max int) gen.Dual {
	return gen.Dual{g.Integer(min, max), g.Integer(min, max)}
}

func randomColor(g *gen.RNG) gen.ColorRGB {
	return gen.ColorRGB{R: uint8(g.Integer(0, 255)), G: uint8(g.Integer(0, 255)), B: uint8(g.Integer(0, 255))}
}

// ---- small helpers -----------------------------------------------------------

func renderTemplate(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := tpl.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = buf.WriteTo(w)
}

func atoi(s string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
		return v
	}
	return def
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func checked(f url.Values, name string) bool { return f.Get(name) != "" }

func dualOf(f url.Values, base string, min, max int) gen.Dual {
	lo := clamp(atoi(f.Get(base+"Min"), min), min, max)
	hi := clamp(atoi(f.Get(base+"Max"), max), min, max)
	if lo > hi {
		lo, hi = hi, lo
	}
	return gen.Dual{lo, hi}
}

func hasPack(packs []gen.SpritesPack, v string) bool {
	for _, p := range packs {
		if string(p) == v {
			return true
		}
	}
	return false
}

func hasMode(modes []gen.CompositionMode, m gen.CompositionMode) bool {
	for _, x := range modes {
		if x == m {
			return true
		}
	}
	return false
}

func parseHex(s string) (gen.ColorRGB, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return gen.ColorRGB{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return gen.ColorRGB{}, false
	}
	return gen.ColorRGB{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v)}, true
}

func toHex(c gen.ColorRGB) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}
