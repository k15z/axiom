package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/output"
	"github.com/k15z/axiom/internal/provider"
	"github.com/k15z/axiom/internal/runner"
	"github.com/k15z/axiom/internal/scaffold"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newAddCmd() *cobra.Command {
	var model string
	var file string
	var runAfter bool

	cmd := &cobra.Command{
		Use:   "add <intent>",
		Short: "Generate a test from a natural-language description",
		Long: `Interactively generate a single behavioral test from a description.

The LLM explores your codebase and produces a test that captures your intent.
You can review and confirm before it's written to disk.

Examples:
  axiom add "all API routes require authentication"
  axiom add "no package imports from the CLI layer"
  axiom add "database connections are always closed" --file db.yml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			intent := args[0]

			// Load config for model default and test_dir
			cfg, err := config.Load(config.LoadOpts{ResolveKey: true})
			if err != nil {
				return &SetupError{Err: err}
			}

			testDir := strings.TrimRight(cfg.TestDir, "/")
			if _, err := os.Stat(testDir); os.IsNotExist(err) {
				return &SetupError{Err: fmt.Errorf("test directory %s not found — run %s first", testDir, color.CyanString("axiom init"))}
			}

			if model == "" {
				model = cfg.Model
			}

			repoRoot, _ := filepath.Abs(".")

			// Progress display
			tty := isatty.IsTerminal(os.Stderr.Fd())
			spin := newSpinner(tty, "exploring codebase…")
			spin.start()

			p := provider.FromConfig(provider.ProviderConfig{
				Provider: cfg.Provider,
				APIKey:   cfg.APIKey,
				BaseURL:  cfg.BaseURL,
			})

			yamlContent, err := scaffold.GenerateTest(
				context.Background(),
				p, model, repoRoot, intent,
				func(msg string) {
					spin.update(msg)
				},
			)
			spin.stop()

			if err != nil {
				return fmt.Errorf("failed to generate test: %w\nCheck your API key and network connection, or run `axiom doctor` to diagnose.", err)
			}

			if err := validateTestYAML(yamlContent); err != nil {
				return fmt.Errorf("generated invalid test: %w", err)
			}

			// Display the generated test
			fmt.Println()
			gray := color.New(color.FgHiBlack).SprintFunc()
			fmt.Println(gray("Generated test:"))
			fmt.Println()
			fmt.Println(yamlContent)
			fmt.Println()

			// Determine target file
			targetFile := file
			if tty && !cmd.Flags().Changed("file") {
				if chosen, ok := promptFileChoice(testDir); ok {
					targetFile = chosen
				}
			}

			// Interactive confirmation
			targetPath := filepath.Join(testDir, targetFile)
			if !tty {
				// Non-interactive: just write it
				return appendTest(targetPath, yamlContent)
			}

			fmt.Printf("Add this test to %s? [Y/n] ", color.CyanString(targetPath))
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))

			if answer == "" || answer == "y" || answer == "yes" {
				if err := appendTest(targetPath, yamlContent); err != nil {
					return err
				}
				fmt.Printf("Test added to %s\n", color.CyanString(targetPath))

				// Offer to run the new test
				testName := extractTestName(yamlContent)
				if testName != "" {
					shouldRun := runAfter
					if !shouldRun && tty {
						fmt.Printf("Run this test now? [Y/n] ")
						runAnswer, _ := reader.ReadString('\n')
						runAnswer = strings.TrimSpace(strings.ToLower(runAnswer))
						shouldRun = runAnswer == "" || runAnswer == "y" || runAnswer == "yes"
					}
					if shouldRun {
						return runNewTest(cfg, testName)
					}
				}
				return nil
			}

			fmt.Println("Discarded.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&model, "model", "m", "", "Override LLM model")
	cmd.Flags().StringVarP(&file, "file", "f", "tests.yml", "Target YAML file inside .axiom/")
	cmd.Flags().BoolVar(&runAfter, "run", false, "Run the new test immediately after adding")

	return cmd
}

// validateTestYAML checks that the YAML content is a valid axiom test definition:
// must parse as YAML, have at least one top-level key with a non-empty "condition" field.
func validateTestYAML(content string) error {
	var raw yaml.Node
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	if raw.Kind != yaml.DocumentNode || len(raw.Content) == 0 {
		return fmt.Errorf("empty YAML document")
	}
	mapping := raw.Content[0]
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) < 2 {
		return fmt.Errorf("YAML must contain at least one test definition")
	}

	// Check each test has a condition
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		keyNode := mapping.Content[i]
		valNode := mapping.Content[i+1]
		var def struct {
			Condition string `yaml:"condition"`
		}
		if err := valNode.Decode(&def); err != nil {
			return fmt.Errorf("test %q: %w", keyNode.Value, err)
		}
		if def.Condition == "" {
			return fmt.Errorf("test %q: condition is required", keyNode.Value)
		}
	}
	return nil
}

// extractTestName parses YAML content and returns the first top-level key (the test name).
func extractTestName(yamlContent string) string {
	var raw yaml.Node
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		return ""
	}
	if raw.Kind != yaml.DocumentNode || len(raw.Content) == 0 {
		return ""
	}
	mapping := raw.Content[0]
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) < 2 {
		return ""
	}
	return mapping.Content[0].Value
}

// runNewTest discovers tests and runs just the named test through the standard runner.
func runNewTest(cfg config.Config, testName string) error {
	tests, err := discovery.Discover(cfg.TestDir)
	if err != nil {
		return fmt.Errorf("failed to load test files: %w", err)
	}

	results, err := runner.Run(context.Background(), cfg, tests, runner.Options{
		Filter: testName,
		All:    true, // skip cache for new test
	})
	if err != nil {
		return err
	}

	output.Print(results, cfg.Model, false, cfg.TestDir)

	if output.HasFailures(results) {
		return &RunFailureError{ExitCode: 1, Msg: "new test failed"}
	}
	return nil
}

// promptFileChoice lists YAML files in testDir and asks the user to pick one.
// Returns the chosen filename and true, or ("", false) if there's only one file
// or no files (caller should use the default).
func promptFileChoice(testDir string) (string, bool) {
	entries, err := os.ReadDir(testDir)
	if err != nil {
		return "", false
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yml") || strings.HasSuffix(e.Name(), ".yaml") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	if len(files) <= 1 {
		return "", false
	}

	fmt.Println("Multiple test files found:")
	for i, f := range files {
		fmt.Printf("  %d) %s\n", i+1, f)
	}
	fmt.Printf("Choose a file [1-%d]: ", len(files))

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(answer)

	var idx int
	if _, err := fmt.Sscanf(answer, "%d", &idx); err != nil || idx < 1 || idx > len(files) {
		// Invalid input — use first file
		fmt.Printf("Using %s\n", files[0])
		return files[0], true
	}
	return files[idx-1], true
}

// appendTest appends a test YAML block to a file, creating it if needed.
func appendTest(path, yamlContent string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// New file: write with header
		header := "# Generated by axiom add\n# See: https://github.com/k15z/axiom\n\n"
		return os.WriteFile(path, []byte(header+yamlContent+"\n"), 0o644)
	}

	// Existing file: append with separator
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	_, err = fmt.Fprintf(f, "\n%s\n", yamlContent)
	return err
}
