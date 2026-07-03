package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"godisplacementx/internal/gen"
)

type generateFlags struct {
	config           string
	out              string
	resolution       int
	width            int
	height           int
	seed             uint64
	iterations       int
	background       int
	rect             bool
	grid             bool
	cols             bool
	rows             bool
	lines            bool
	sprites          bool
	spritesRotation  bool
	seamless         bool
	spritePacks      []string
	compositionModes []string
	mode             string
	invert           bool
	gradient         string
	randomize        bool
}

func newGenerateCmd() *cobra.Command {
	f := &generateFlags{}
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Render a displacement map to a PNG file",
		Long: `Render a displacement map to a PNG.

Parameters come from defaults, an optional --config JSON file, optional
--randomize, then any explicitly provided flags (which take precedence). Use
--seed for reproducible output.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerate(cmd, f)
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&f.config, "config", "", "path to a Params JSON config file")
	fl.StringVarP(&f.out, "out", "o", "", "output PNG path (default DisplacementX_<w>x<h>_<timestamp>.png)")
	fl.IntVar(&f.resolution, "resolution", 2048, "square output resolution (e.g. 1024, 2048, 4096, 8192)")
	fl.IntVar(&f.width, "width", 0, "output width (overrides --resolution)")
	fl.IntVar(&f.height, "height", 0, "output height (overrides --resolution)")
	fl.Uint64Var(&f.seed, "seed", 0, "RNG seed for reproducible output (default: random)")
	fl.IntVar(&f.iterations, "iterations", 0, "number of drawing iterations (10-2000)")
	fl.IntVar(&f.background, "background", 0, "background brightness 0-255")
	fl.BoolVar(&f.rect, "rect", true, "enable the rectangles layer")
	fl.BoolVar(&f.grid, "grid", true, "enable the grid layer")
	fl.BoolVar(&f.cols, "cols", true, "enable the columns layer")
	fl.BoolVar(&f.rows, "rows", true, "enable the rows layer")
	fl.BoolVar(&f.lines, "lines", true, "enable the lines layer")
	fl.BoolVar(&f.sprites, "sprites", false, "enable the sprites layer")
	fl.BoolVar(&f.spritesRotation, "sprites-rotation", true, "rotate sprites randomly by 90 degrees")
	fl.BoolVar(&f.seamless, "seamless", false, "generate a seamless / tileable texture")
	fl.StringSliceVar(&f.spritePacks, "sprite-packs", nil, "sprite packs: classic,bigdata,aggromaxx,crappack")
	fl.StringSliceVar(&f.compositionModes, "composition-modes", nil, "composition (blend) modes, e.g. multiply,screen")
	fl.StringVar(&f.mode, "mode", "grayscale", "output mode: grayscale, normal, color")
	fl.BoolVar(&f.invert, "invert", false, "invert the output colours")
	fl.StringVar(&f.gradient, "gradient", "", "color-mode gradient stops, e.g. \"#00ffff,#9500ff,#ffe500\"")
	fl.BoolVar(&f.randomize, "randomize", false, "randomize all parameters before applying explicit flags")

	return cmd
}

func runGenerate(cmd *cobra.Command, f *generateFlags) error {
	params := gen.Default()
	if f.config != "" {
		p, err := loadParams(f.config)
		if err != nil {
			return err
		}
		params = p
	}

	// Choose the seed up front so --randomize is reproducible and the seed can
	// be reported.
	seed := f.seed
	if !cmd.Flags().Changed("seed") {
		seed = gen.RandomSeed()
	}

	if f.randomize {
		params.Randomize(gen.NewRNG(seed))
	}

	if err := applyOverrides(cmd, f, &params); err != nil {
		return err
	}

	w, h := f.resolution, f.resolution
	if f.width > 0 {
		w = f.width
	}
	if f.height > 0 {
		h = f.height
	}

	gradient, err := parseGradient(f.gradient)
	if err != nil {
		return err
	}

	res, err := gen.Render(gen.RenderRequest{
		Params:   params,
		Width:    w,
		Height:   h,
		Seed:     seed,
		HasSeed:  true,
		Mode:     gen.OutputMode(f.mode),
		Invert:   f.invert,
		Gradient: gradient,
	})
	if err != nil {
		return err
	}

	out := f.out
	if out == "" {
		ts := time.Now().Format("2006-01-02-150405")
		out = fmt.Sprintf("DisplacementX_%dx%d_%s.png", w, h, ts)
	}

	file, err := os.Create(out)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := gen.EncodePNG(file, res.Canvas); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%dx%d, seed %d, %d ms)\n", out, w, h, res.Seed, res.DurationMS)
	return nil
}

// applyOverrides applies any explicitly-set flags on top of params.
func applyOverrides(cmd *cobra.Command, f *generateFlags, params *gen.Params) error {
	changed := func(name string) bool { return cmd.Flags().Changed(name) }

	if changed("iterations") {
		params.Iterations = f.iterations
	}
	if changed("background") {
		params.BackgroundBrightness = f.background
	}
	if changed("rect") {
		params.RectEnabled = f.rect
	}
	if changed("grid") {
		params.GridEnabled = f.grid
	}
	if changed("cols") {
		params.ColsEnabled = f.cols
	}
	if changed("rows") {
		params.RowsEnabled = f.rows
	}
	if changed("lines") {
		params.LinesEnabled = f.lines
	}
	if changed("sprites") {
		params.SpritesEnabled = f.sprites
	}
	if changed("sprites-rotation") {
		params.SpritesRotationEnabled = f.spritesRotation
	}
	if changed("seamless") {
		params.SeamlessTextureEnabled = f.seamless
	}
	if changed("sprite-packs") {
		packs, err := parsePacks(f.spritePacks)
		if err != nil {
			return err
		}
		params.SpritesPacks = packs
	}
	if changed("composition-modes") {
		modes, err := parseModes(f.compositionModes)
		if err != nil {
			return err
		}
		params.CompositionModes = modes
	}
	return nil
}
