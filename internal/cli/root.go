package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// SetupError indicates a configuration or setup problem (missing API key,
// bad YAML, test directory not found). Commands should wrap pre-run errors
// in SetupError so the CLI can exit with code 2 instead of 1.
type SetupError struct {
	Err error
}

func (e *SetupError) Error() string { return e.Err.Error() }
func (e *SetupError) Unwrap() error { return e.Err }

// Execute runs the root command and exits with the appropriate code:
//
//	0 — success
//	1 — test failures (or general runtime error)
//	2 — configuration/setup error
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		var se *SetupError
		if errors.As(err, &se) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "axiom",
		Short:         "AI-driven behavioral tests for your codebase",
		Long:          "Write plain-English conditions in YAML, and axiom verifies them against your source code using an agentic LLM.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	root.AddCommand(newRunCmd())
	root.AddCommand(newInitCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newCacheCmd())
	root.AddCommand(newValidateCmd())

	return root
}
