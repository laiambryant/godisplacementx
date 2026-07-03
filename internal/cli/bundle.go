package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"godisplacementx/internal/gen"
)

type bundleFlags struct {
	config     string
	resolution int
	width      int
	height     int
	seed       uint64
	invert     bool
	gradient   string
	emits      []string
}

func newBundleCmd() *cobra.Command {
	f := &bundleFlags{}
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Render several output maps from a single generation pass",
		Long: `Render multiple output maps (grayscale, color, normal) in one process.

The height field is generated once per distinct seed and reused for every map
derived from it, so producing albedo + normal + height costs a single Generate
instead of one per map. Each --emit selects a mode and output path, with an
optional per-map seed (useful to fill separate R/G/B/roughness/height channels
from independent fields):

  --emit grayscale:height.png --emit color:albedo.png --emit normal:normal.png
  --emit grayscale:111:r.png  --emit grayscale:222:g.png

The --emit value is "mode:path" or "mode:seed:path"; the path may contain colons
(e.g. Windows drive letters).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBundle(cmd, f)
		},
	}

	fl := cmd.Flags()
	fl.StringVar(&f.config, "config", "", "path to a Params JSON config file")
	fl.IntVar(&f.resolution, "resolution", 2048, "square output resolution (e.g. 1024, 2048, 4096, 8192)")
	fl.IntVar(&f.width, "width", 0, "output width (overrides --resolution)")
	fl.IntVar(&f.height, "height", 0, "output height (overrides --resolution)")
	fl.Uint64Var(&f.seed, "seed", 0, "default RNG seed for emits without an explicit seed (default: random)")
	fl.BoolVar(&f.invert, "invert", false, "invert the output colours")
	fl.StringVar(&f.gradient, "gradient", "", "color-mode gradient stops, e.g. \"#00ffff,#9500ff,#ffe500\"")
	fl.StringArrayVar(&f.emits, "emit", nil, "output map: \"mode:path\" or \"mode:seed:path\" (repeatable)")

	return cmd
}

func runBundle(cmd *cobra.Command, f *bundleFlags) error {
	params := gen.Default()
	if f.config != "" {
		p, err := loadParams(f.config)
		if err != nil {
			return err
		}
		params = p
	}

	defaultSeed := f.seed
	if !cmd.Flags().Changed("seed") {
		defaultSeed = gen.RandomSeed()
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

	if len(f.emits) == 0 {
		return fmt.Errorf("at least one --emit is required")
	}
	emits := make([]gen.EmitSpec, 0, len(f.emits))
	for _, s := range f.emits {
		e, err := parseEmit(s, defaultSeed)
		if err != nil {
			return err
		}
		emits = append(emits, e)
	}

	return gen.RenderBundle(params, w, h, f.invert, gradient, emits)
}

// parseEmit parses a single --emit value. Format is "mode:path" or
// "mode:seed:path"; an optional numeric seed prefix is recognised only when it
// parses as an unsigned integer, so paths containing colons (Windows drive
// letters) stay intact.
func parseEmit(s string, defaultSeed uint64) (gen.EmitSpec, error) {
	head := strings.SplitN(s, ":", 2)
	if len(head) != 2 || head[0] == "" || head[1] == "" {
		return gen.EmitSpec{}, fmt.Errorf("invalid --emit %q, want \"mode:path\" or \"mode:seed:path\"", s)
	}
	mode := gen.OutputMode(head[0])
	switch mode {
	case gen.OutputGrayscale, gen.OutputColor, gen.OutputNormal:
	default:
		return gen.EmitSpec{}, fmt.Errorf("invalid --emit mode %q (want grayscale, color or normal)", head[0])
	}

	seed := defaultSeed
	path := head[1]
	if i := strings.IndexByte(path, ':'); i >= 0 {
		if v, err := strconv.ParseUint(path[:i], 10, 64); err == nil {
			seed = v
			path = path[i+1:]
		}
	}
	if path == "" {
		return gen.EmitSpec{}, fmt.Errorf("invalid --emit %q, empty path", s)
	}
	return gen.EmitSpec{Mode: mode, Path: path, Seed: seed}, nil
}
