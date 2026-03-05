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
- `internal/cli/` — CLI commands (run, init, list, cache)
- `internal/runner/` — Test execution orchestration
- `internal/discovery/` — YAML test file parsing
- `internal/cache/` — Content-hash based test caching
- `internal/config/` — Config loading and .env parsing
- `internal/glob/` — Go-native glob matching
- `internal/output/` — Terminal and JSON output formatting
- `internal/display/` — Live terminal spinner
- `internal/scaffold/` — Test template generation
- `internal/types/` — Shared type definitions
- `.axiom/` — Behavioral test definitions (YAML)
- `.github/workflows/ci.yml` — CI: build, vet, test on PRs; axiom run on main push

## Workflow

- Branch naming: `claude/<descriptive-name>` for Claude Code sessions
- CI runs `go build`, `go vet`, `go test` on all PRs
- PRs target `main`
- After creating a PR, wait for CI checks to pass before merging
- Use `gh pr create` and `gh pr merge` for PR operations

## Key Dependencies

- `anthropic-sdk-go` — Claude API client
- `cobra` — CLI framework
- `fatih/color` — Terminal colors
- `tidwall/gjson` + `sjson` — JSON manipulation

## Configuration

- `axiom.yml` — Project config (model, agent settings)
- `.env` — API keys (gitignored, never commit)
