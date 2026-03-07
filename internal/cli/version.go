package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "dev"

// SetVersion sets the version string displayed by `axiom version`.
// Called from main with the value injected via ldflags.
func SetVersion(v string) {
	version = v
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the axiom version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("axiom %s\n", version)
		},
	}
}
