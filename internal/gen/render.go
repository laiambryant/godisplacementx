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
	// Fast trades the bit-exact determinism contract for speed: output is
	// visually equivalent (within a couple of units per channel) but not
	// reproducible byte-for-byte across runs, machines or versions.
	Fast bool
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
	c, err := generateWith(req.Params, req.Width, req.Height, g, req.Fast)
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
// which is the dominant cost. Emits sharing a seed post-process and write in
// parallel, each on a private copy of the shared field — except plain grayscale
// with no invert, which reads the shared field directly (eliding a full-canvas
// clone). fast selects the non-deterministic fast paths (see RenderRequest.Fast).
func RenderBundle(p Params, w, h int, invert bool, gradient []ColorRGB, emits []EmitSpec, fast bool) error {
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

	errs := make([]error, len(emits))
	var wg sync.WaitGroup
	for _, seed := range order {
		indices := bySeed[seed]
		wg.Go(func() {
			base, err := generateWith(p, w, h, NewRNG(seed), fast)
			if err != nil {
				errs[indices[0]] = err
				return
			}
			var emitWG sync.WaitGroup
			for _, idx := range indices {
				emitWG.Go(func() {
					errs[idx] = writeEmit(base, emits[idx], invert, gradient, fast)
				})
			}
			emitWG.Wait()
		})
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// writeEmit post-processes one output map and writes it. Modes that transform
// pixels work on a private clone; plain grayscale output writes straight from
// the shared field, which concurrent emits only read.
func writeEmit(base *Canvas, e EmitSpec, invert bool, gradient []ColorRGB, fast bool) error {
	c := base
	if emitTransformsPixels(e.Mode, invert) {
		c = base.Clone()
		if err := applyMode(c, e.Mode, gradient); err != nil {
			return err
		}
		if invert {
			ApplyInvert(c)
		}
	}
	return WriteMapFile(e.Path, c, e.Mode, fast)
}

func emitTransformsPixels(mode OutputMode, invert bool) bool {
	if invert {
		return true
	}
	switch mode {
	case OutputGrayscale, "":
		return false
	}
	return true
}

// RandomSeed returns a cryptographically random seed. crypto/rand.Read never
// fails (guaranteed since Go 1.24).
func RandomSeed() uint64 {
	var b [8]byte
	_, _ = cryptorand.Read(b[:])
	return binary.LittleEndian.Uint64(b[:])
}
