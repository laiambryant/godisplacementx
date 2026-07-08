package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"godisplacementx/internal/gen"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("godisplacementx %s (%s)\n", Version, gen.BuildVariant)
		},
	}
}
