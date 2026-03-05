package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const sampleTest = `# Example axiom behavioral test
# See: https://github.com/k15z/axiom

test_example:
  on:
    - "**/*.go"
  condition: >
    All exported functions should have descriptive names that clearly
    indicate their purpose. Single-letter function names or overly
    abbreviated names like "proc" or "hldr" should not be used for
    exported functions.
`

const sampleConfig = `# axiom configuration
# See: https://github.com/k15z/axiom
model: claude-haiku-4-5-20251001
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize axiom in the current project",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := ".axiom"

			if _, err := os.Stat(dir); err == nil {
				return fmt.Errorf("%s already exists", dir)
			}

			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("creating %s: %w", dir, err)
			}

			examplePath := filepath.Join(dir, "example.yml")
			if err := os.WriteFile(examplePath, []byte(sampleTest), 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", examplePath, err)
			}

			// Write axiom.yml config at the project root
			if err := os.WriteFile("axiom.yml", []byte(sampleConfig), 0o644); err != nil {
				return fmt.Errorf("writing axiom.yml: %w", err)
			}

			fmt.Println("Created .axiom/ with an example test and axiom.yml config.")
			fmt.Println("Edit .axiom/example.yml to write your first behavioral test.")
			return nil
		},
	}
}
