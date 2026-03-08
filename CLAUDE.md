# Axiom

Behavioral testing framework for codebases, powered by Claude. Tests are YAML files in `.axiom/` that define glob-based file triggers and natural language conditions verified by an LLM agent.

## Build & Test

```bash
go build ./...          # Build all packages
go vet ./...            # Lint
go test ./...           # Unit tests
go run ./cmd/axiom/ run --all  # Run all axiom behavioral tests (requires ANTHROPIC_API_KEY)
```

## Project Structure

- `cmd/axiom/` — CLI entrypoint (uses cobra)
- `internal/agent/` — LLM agent loop and tool definitions
- `internal/cli/` — CLI commands (run, init, list, show, validate, cache, add)
- `internal/runner/` — Test execution orchestration
- `internal/discovery/` — YAML test file parsing
- `internal/cache/` — Content-hash based test caching
- `internal/config/` — Config loading and .env parsing
- `internal/glob/` — Go-native glob matching
- `internal/notes/` — Agent memory / codebase notes caching
- `internal/output/` — Terminal, JSON, and GitHub Markdown output formatting
- `internal/display/` — Live terminal spinner
- `internal/provider/` — LLM provider abstraction (Anthropic, OpenAI, Gemini)
- `internal/scaffold/` — Test template generation
- `internal/watch/` — File watching for `--watch` mode (fsnotify)
- `internal/types/` — Shared type definitions
- `docs/` — mdBook documentation site (deployed to GitHub Pages)
- `action.yml` — Reusable GitHub Action for CI
- `.axiom/` — Behavioral test definitions (YAML)
- `.github/workflows/ci.yml` — CI: build, vet, test on PRs; axiom run on main push
- `.github/workflows/docs.yml` — Docs: build and deploy mdBook to GitHub Pages

## Workflow

- **All changes go through PRs** — never commit directly to `main`. Create a branch, do the work, open a PR, merge after CI passes.
- Branch naming: `claude/<descriptive-name>` for Claude Code sessions
- CI runs `go build`, `go vet`, `go test -race` on all PRs
- PRs target `main`
- Before opening a PR, verify `go build ./... && go vet ./... && go test ./...` passes locally
- After creating a PR, wait for CI checks to pass before merging
- Use `gh pr create` and `gh pr merge` for PR operations

## Key Dependencies

- `anthropic-sdk-go` — Claude API client
- `cobra` — CLI framework
- `fatih/color` — Terminal colors
- `tidwall/gjson` + `sjson` — JSON manipulation
- `fsnotify/fsnotify` — File system watching (watch mode)

## Configuration

- `axiom.yml` — Project config (model, provider, base_url, agent settings, cache)
- `.env` — API keys (gitignored, never commit): `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `GEMINI_API_KEY`

## CLI Development Rules

These rules exist because we've shipped bugs in each of these categories. Follow them strictly.

### Path handling
- **Always use `filepath.Join()`** when combining directory and filename components. Never use `fmt.Sprintf("%s%s", dir, file)` — it breaks when the directory lacks a trailing slash.
- Test path display with both trailing-slash (`".axiom/"`) and no-trailing-slash (`"/tmp/test"`) directory values.

### Error handling in cobra commands
- **Never call `os.Exit()` inside a `RunE` function.** Return a typed error (`SetupError`, or a new type for failure-exit) and let `root.go`'s `Execute()` handle exit codes. `os.Exit()` skips deferred cleanup.
- **Wrap errors with context.** Use `fmt.Errorf("doing X: %w", err)` so users can trace the cause.
- **Make hint messages contextual.** The root error handler should not suggest "run `axiom doctor`" when the failing command IS doctor. Check the command name before emitting hints.

### Flag validation
- **Validate numeric flags early.** Reject negative values for `--concurrency`, `--retries`, and similar flags at the top of `RunE`, before doing any work.
- **Detect conflicting flags.** If two flags are mutually exclusive (e.g., `--json` and `--format`), return an error when both are set.

### User feedback
- **Never produce silent empty output.** If a filter, tag, or test name matches nothing, print a clear message: "no tests matched filter 'X'". This applies to dry-run, list, show, and run.
- **Idempotent cleanup commands.** `cache clear` on a non-existent cache should be a no-op, not an error.
- **Pluralization matters.** Use "1 test" not "1 tests". Use a helper or conditional format string.

### Input validation
- **Validate test names during discovery.** Reject empty names and names containing path separators (`/`, `\`).
- **Validate glob semantics, not just syntax.** Warn on absolute paths (`/etc/...`) and parent traversal (`../`) in `on:` patterns — they'll never match project files.

### Testing
- When adding or modifying a CLI command, test it manually with:
  - Default config (no flags)
  - Non-default `-d` directory (without trailing slash)
  - Edge case inputs (empty, negative numbers, special characters)
  - The `--json` output variant (if applicable)
- See `BUGS.md` for the full list of known issues and their root causes.
