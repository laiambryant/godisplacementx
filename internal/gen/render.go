package gen

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"time"
)

// OutputMode selects the post-processing applied to the rendered height field.
type OutputMode string

const (
	OutputGrayscale OutputMode = "grayscale"
	OutputNormal    OutputMode = "normal"
	OutputColor     OutputMode = "color"
)

// RenderRequest bundles everything needed to produce a final image. It is shared
// by the CLI and the GUI so seed handling and post-processing stay consistent.
type RenderRequest struct {
	Params   Params
	Width    int
	Height   int
	Seed     uint64
	HasSeed  bool // when false, a random seed is chosen and reported back
	Mode     OutputMode
	Invert   bool
	Gradient []ColorRGB // used when Mode == OutputColor; defaults if empty
}

// RenderResult is the outcome of a render.
type RenderResult struct {
	Canvas     *Canvas
	Seed       uint64
	DurationMS int64
}

// Render selects a seed, generates the height field, and applies post-processing
// (normal/color/invert). The seed actually used is reported so runs can be
// reproduced.
func Render(req RenderRequest) (*RenderResult, error) {
	if req.Width <= 0 || req.Height <= 0 {
		return nil, fmt.Errorf("invalid size %dx%d", req.Width, req.Height)
	}
	seed := req.Seed
	if !req.HasSeed {
		seed = RandomSeed()
	}

	start := time.Now()
	g := NewRNG(seed)
	c, err := Generate(req.Params, req.Width, req.Height, g)
	if err != nil {
		return nil, err
	}

	if err := applyMode(c, req.Mode, req.Gradient); err != nil {
		return nil, err
	}

	if req.Invert {
		ApplyInvert(c)
	}

	return &RenderResult{
		Canvas:     c,
		Seed:       seed,
		DurationMS: time.Since(start).Milliseconds(),
	}, nil
}

// applyMode applies the post-processing selected by mode to a generated height
// field. Grayscale (or empty) leaves the canvas untouched. Shared by Render and
// RenderBundle so the CLI and GUI stay consistent.
func applyMode(c *Canvas, mode OutputMode, gradient []ColorRGB) error {
	switch mode {
	case OutputNormal:
		ApplyNormal(c)
	case OutputColor:
		stops := gradient
		if len(stops) < 2 {
			stops = DefaultGradient()
		}
		ApplyColor(c, BuildPalette(stops))
	case OutputGrayscale, "":
		// nothing
	default:
		return fmt.Errorf("unknown output mode %q", mode)
	}
	return nil
}

// EmitSpec describes one output map of a bundle render: which post-processing
// Mode to apply, the file Path to write, and the Seed of the height field it is
// derived from. Emits sharing a Seed reuse a single (expensive) Generate pass.
type EmitSpec struct {
	Mode OutputMode
	Path string
	Seed uint64
}

// RenderBundle generates every distinct height field exactly once (running the
// unique seeds concurrently) and writes each requested output map. This replaces
// N separate process spawns + N Generate passes with a single in-process render,
// which is the dominant cost. Post-processing (color/normal/invert) is cheap and
// runs per emit on a private copy of the shared field.
func RenderBundle(p Params, w, h int, invert bool, gradient []ColorRGB, emits []EmitSpec) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("invalid size %dx%d", w, h)
	}
	if len(emits) == 0 {
		return fmt.Errorf("no emit specs")
	}

	// Group emit indices by seed, preserving first-seen order for determinism.
	bySeed := map[uint64][]int{}
	order := make([]uint64, 0, len(emits))
	for i, e := range emits {
		if _, ok := bySeed[e.Seed]; !ok {
			order = append(order, e.Seed)
		}
		bySeed[e.Seed] = append(bySeed[e.Seed], i)
	}

	errs := make([]error, len(order))
	var wg sync.WaitGroup
	for k, seed := range order {
		wg.Add(1)
		go func(k int, seed uint64) {
			defer wg.Done()
			base, err := Generate(p, w, h, NewRNG(seed))
			if err != nil {
				errs[k] = err
				return
			}
			for _, idx := range bySeed[seed] {
				e := emits[idx]
				c := base.Clone()
				if err := applyMode(c, e.Mode, gradient); err != nil {
					errs[k] = err
					return
				}
				if invert {
					ApplyInvert(c)
				}
				if err := WritePNGFile(e.Path, c); err != nil {
					errs[k] = err
					return
				}
			}
		}(k, seed)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// RandomSeed returns a cryptographically random seed.
func RandomSeed() uint64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return uint64(time.Now().UnixNano())
	}
	return binary.LittleEndian.Uint64(b[:])
}
