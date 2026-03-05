package cli

import (
	"github.com/spf13/cobra"
)

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
	root.AddCommand(newListCmd())
	root.AddCommand(newShowCmd())
	root.AddCommand(newCacheCmd())

	return root
}
