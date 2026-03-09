package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/agent"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/discovery"
	"github.com/k15z/axiom/internal/provider"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check your axiom setup for common issues",
		Long:  "Validates configuration, API key, test directory, and optional dependencies. Run this first when something isn't working.",
		RunE: func(cmd *cobra.Command, args []string) error {
			green := color.New(color.FgGreen)
			red := color.New(color.FgRed)
			yellow := color.New(color.FgYellow)
			gray := color.New(color.FgHiBlack)

			fmt.Println()
			color.New(color.Bold).Println("  axiom doctor")
			fmt.Println()

			errors := 0

			// 1. Config file
			_, yamlErr := os.ReadFile("axiom.yml")
			cfg, loadErr := config.Load(config.LoadOpts{TestDir: dir})
			if yamlErr != nil {
				if os.IsNotExist(yamlErr) {
					yellow.Print("    ~ ")
					fmt.Println("axiom.yml not found (using defaults)")
					gray.Println("      This is fine — axiom works without a config file.")
				} else {
					red.Print("    ✗ ")
					fmt.Printf("axiom.yml: %s\n", yamlErr)
					errors++
				}
			} else {
				if loadErr != nil {
					red.Print("    ✗ ")
					fmt.Printf("axiom.yml: %s\n", loadErr)
					errors++
				} else {
					green.Print("    ✓ ")
					fmt.Printf("axiom.yml (model: %s)\n", cfg.Model)
				}
			}
			if loadErr != nil {
				fmt.Println()
				return fmt.Errorf("failed to load config: %w", loadErr)
			}

			// 2. API key
			resolveErr := cfg.ResolveKey()
			if resolveErr != nil {
				red.Print("    ✗ ")
				fmt.Println(resolveErr)
				switch cfg.Provider {
				case "anthropic":
					gray.Println("      Get a key at console.anthropic.com and add it to .env as ANTHROPIC_API_KEY=sk-...")
				case "openai":
					gray.Println("      Get a key at platform.openai.com and add it to .env as OPENAI_API_KEY=sk-...")
				case "gemini":
					gray.Println("      Get a key at aistudio.google.com and add it to .env as GEMINI_API_KEY=...")
				default:
					gray.Printf("      Set the API key for %q in .env or your environment.\n", cfg.Provider)
				}
				errors++
			} else {
				green.Print("    ✓ ")
				fmt.Printf("API key (%s)\n", cfg.Provider)

				// 2b. Provider connectivity — make a cheap API call to verify the key works
				p := provider.FromConfig(provider.ProviderConfig{
					Provider: cfg.Provider,
					APIKey:   cfg.APIKey,
					BaseURL:  cfg.BaseURL,
				})
				ctx, cancel := context.WithTimeout(cmd.Context(), 15*time.Second)
				defer cancel()
				_, chatErr := p.Chat(ctx, provider.ChatParams{
					Model:  cfg.Model,
					System: "Respond with OK.",
					Messages: []provider.Message{
						{Role: "user", Content: []provider.ContentBlock{{Type: "text", Text: "ping"}}},
					},
					MaxTokens: 1,
				})
				if chatErr != nil {
					red.Print("    ✗ ")
					fmt.Printf("provider connectivity failed: %s\n", chatErr)
					gray.Println("      Check your API key, network connection, and provider status.")
					errors++
				} else {
					green.Print("    ✓ ")
					fmt.Printf("provider reachable (%s)\n", cfg.Model)
				}
			}

			// 3. Test directory
			testDir := cfg.TestDir
			if _, statErr := os.Stat(testDir); os.IsNotExist(statErr) {
				red.Print("    ✗ ")
				fmt.Printf("test directory %q not found\n", testDir)
				gray.Println("      Run `axiom init` to create it, or `axiom add` to create your first test.")
				errors++
			} else {
				tests, discErr := discovery.Discover(testDir)
				if discErr != nil {
					red.Print("    ✗ ")
					fmt.Printf("test directory %q: %s\n", testDir, discErr)
					errors++
				} else if len(tests) == 0 {
					yellow.Print("    ~ ")
					fmt.Printf("test directory %q exists but contains no tests\n", testDir)
					gray.Println("      Run `axiom add` to create your first test.")
				} else {
					green.Print("    ✓ ")
					fmt.Printf("test directory %q (%d test(s))\n", testDir, len(tests))

					// Validate glob syntax in discovered tests
					if pfErr := preflightValidate(tests); pfErr != nil {
						red.Print("    ✗ ")
						fmt.Printf("invalid glob patterns found — run `axiom validate` for details\n")
						errors++
					}
				}
			}

			// 4. ripgrep (optional)
			if agent.RgAvailable() {
				green.Print("    ✓ ")
				fmt.Println("ripgrep (rg) installed")
			} else {
				yellow.Print("    ~ ")
				fmt.Println("ripgrep (rg) not found")
				gray.Println("      Install ripgrep for faster grep searches: https://github.com/BurntSushi/ripgrep#installation")
			}

			fmt.Println()
			if errors > 0 {
				red.Printf("  %d issue(s) found\n", errors)
				fmt.Println()
				return fmt.Errorf("doctor found %d issue(s)", errors)
			}
			green.Println("  All checks passed")
			fmt.Println()
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	return cmd
}
