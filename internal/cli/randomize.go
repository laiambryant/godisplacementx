package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"godisplacementx/internal/gen"
)

func newRandomizeCmd() *cobra.Command {
	var seed uint64
	var out string
	cmd := &cobra.Command{
		Use:   "randomize",
		Short: "Print a randomized Params JSON config",
		Long:  "Generate a randomized set of parameters (the same as the GUI's \"Randomize all\") and print it as JSON, suitable for `generate --config`.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s := seed
			if !cmd.Flags().Changed("seed") {
				s = gen.RandomSeed()
			}
			p := gen.Default()
			p.Randomize(gen.NewRNG(s))

			// Params is plain data; MarshalIndent cannot fail on it.
			data, _ := json.MarshalIndent(p, "", "  ")
			data = append(data, '\n')

			if out == "" {
				_, err := os.Stdout.Write(data)
				return err
			}
			if err := os.WriteFile(out, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "wrote %s (seed %d)\n", out, s)
			return nil
		},
	}
	cmd.Flags().Uint64Var(&seed, "seed", 0, "RNG seed (default: random)")
	cmd.Flags().StringVarP(&out, "out", "o", "", "write JSON to a file instead of stdout")
	return cmd
}
