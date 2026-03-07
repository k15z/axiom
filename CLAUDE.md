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
- `fsnotify/fsnotify` — File system watching (watch mode)

## Configuration

- `axiom.yml` — Project config (model, provider, base_url, agent settings, cache)
- `.env` — API keys (gitignored, never commit): `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `GEMINI_API_KEY`
