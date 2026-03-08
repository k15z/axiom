package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/k15z/axiom/internal/cache"
	"github.com/k15z/axiom/internal/config"
	"github.com/k15z/axiom/internal/runner"
	"github.com/spf13/cobra"
)

func newCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the test cache",
	}
	cmd.AddCommand(newCacheClearCmd())
	cmd.AddCommand(newCacheInfoCmd())
	return cmd
}

func newCacheClearCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the test cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.LoadOpts{TestDir: dir})
			if err != nil {
				return &SetupError{Err: err}
			}
			if err := runner.ClearCache(cfg.Cache.Dir); err != nil {
				return fmt.Errorf("clearing cache: %w", err)
			}
			fmt.Println("Cache cleared.")
			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	return cmd
}

func newCacheInfoCmd() *cobra.Command {
	var dir string

	cmd := &cobra.Command{
		Use:   "info",
		Short: "Show cache statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.LoadOpts{TestDir: dir})
			if err != nil {
				return &SetupError{Err: err}
			}

			configHash := cache.HashConfig(cfg.Model, cfg.Agent.MaxIterations, cfg.Agent.MaxTokens, cfg.Provider, cfg.BaseURL)
			c, err := cache.Load(cfg.Cache.Dir, configHash)
			if err != nil {
				return fmt.Errorf("loading cache: %w\nThe cache file may be corrupted. Run `axiom cache clear` to reset it.", err)
			}

			entries := c.Entries()
			gray := color.New(color.FgHiBlack)
			green := color.New(color.FgGreen)
			red := color.New(color.FgRed)

			if len(entries) == 0 {
				fmt.Println("Cache is empty. Run `axiom run` to populate it.")
				return nil
			}

			// Compute stats
			var passed, failed int
			var oldest, newest time.Time
			for _, e := range entries {
				if e.Result == "pass" {
					passed++
				} else {
					failed++
				}
				if oldest.IsZero() || e.LastRun.Before(oldest) {
					oldest = e.LastRun
				}
				if newest.IsZero() || e.LastRun.After(newest) {
					newest = e.LastRun
				}
			}

			// File size
			var sizeStr string
			if info, err := os.Stat(c.FilePath()); err == nil {
				sizeStr = formatSize(info.Size())
			} else {
				sizeStr = "unknown"
			}

			fmt.Println()
			fmt.Printf("  Cache: %s\n", gray.Sprint(c.FilePath()))
			fmt.Printf("  Size:  %s\n", sizeStr)
			fmt.Printf("  Tests: %d (", len(entries))
			if passed > 0 {
				green.Printf("%d passed", passed)
			}
			if passed > 0 && failed > 0 {
				fmt.Print(", ")
			}
			if failed > 0 {
				red.Printf("%d failed", failed)
			}
			fmt.Println(")")
			if !oldest.IsZero() {
				fmt.Printf("  Oldest: %s\n", gray.Sprint(oldest.Format(time.RFC3339)))
			}
			if !newest.IsZero() {
				fmt.Printf("  Newest: %s\n", gray.Sprint(newest.Format(time.RFC3339)))
			}
			fmt.Println()

			return nil
		},
	}

	cmd.Flags().StringVarP(&dir, "dir", "d", "", "Path to test directory")
	return cmd
}

func formatSize(bytes int64) string {
	switch {
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
