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
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		// Determine which command ran (if any).
		cmdName := ""
		if c, _, _ := root.Find(os.Args[1:]); c != nil {
			cmdName = c.Name()
		}

		var rfe *RunFailureError
		if errors.As(err, &rfe) {
			os.Exit(rfe.ExitCode)
		}

		var se *SetupError
		if errors.As(err, &se) {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			if cmdName != "doctor" {
				fmt.Fprintf(os.Stderr, "Hint: run `axiom doctor` to diagnose setup issues.\n")
			}
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		if cmdName != "doctor" {
			fmt.Fprintf(os.Stderr, "Hint: run `axiom doctor` to diagnose issues.\n")
		}
		os.Exit(1)
	}
}

// NewRootCmd builds and returns the root cobra command with all subcommands attached.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "axiom",
		Short:         "AI-driven behavioral tests for your codebase",
		Long:          "Write plain-English conditions in YAML, and axiom verifies them against your source code using an agentic LLM.",
		Version:       version,
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
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newVersionCmd())

	return root
}
