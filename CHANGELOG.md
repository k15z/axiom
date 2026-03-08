# Changelog

All notable changes to axiom are documented here.

## [0.1.2] - 2026-03-07

Engine reliability, CLI polish, distribution infrastructure, and expanded test coverage.

### Added

- **Multi-provider support** -- Anthropic, OpenAI, Gemini, and OpenAI-compatible endpoints (Ollama, vLLM) via `provider` and `base_url` config
- **Reusable GitHub Action** -- `uses: k15z/axiom@v0` for one-step CI with cache restore, test execution, and PR comment posting
- **Watch mode** (`axiom run --watch`) -- re-run affected tests automatically when source files change
- **Flaky test detection** (`--retries N`) -- re-run failures and mark as flaky if a retry passes
- **`axiom add`** -- generate a test from a natural-language description with `axiom add "your intent" --run`
- **`axiom validate`** -- lint test YAML for invalid globs, missing patterns, and vague conditions
- **`axiom show --diff`** -- compare current cached reasoning against the previous run
- **Dry-run mode** (`--dry-run`) -- preview which tests would run and estimate token cost
- **Per-test config overrides** -- `timeout`, `model`, and `max_iterations` in test YAML
- **Test tags and filtering** -- `tags: [security, auth]` in YAML, `axiom run --tag security`
- **Agent memory** -- persists codebase investigation notes across runs, auto-invalidated when files change
- **PR comment summaries** (`--format github`) -- GitHub-flavored Markdown output for CI
- **Documentation site** -- mdBook-based docs deployed to GitHub Pages
- **Binary releases** -- pre-built binaries for macOS (arm64/amd64), Linux (arm64/amd64), and Windows via goreleaser
- **`axiom cache info`** -- show cache entry count, total size, oldest entry, per-test status
- **`--strict` mode** -- treat flaky tests as failures
- **`--quiet` / `-q`** -- suppress full output, emit only CI summary line
- **`axiom list --json`** -- JSON output for programmatic consumption
- **`axiom version`** -- version command with ldflags support
- **Token budget hints** -- system message injected when agent approaches token limit
- **Progress indicator** -- live "3/10 tests complete" during runs
- **Per-tool timeouts** -- individual timeout per agent tool call
- **Pre-flight glob validation** -- checks glob syntax before any API calls, exits with code 2 on malformed patterns

### Changed

- **Provider reuse** -- one shared SDK client per run instead of constructing per-test; progress callback injected per-call
- **Exit code semantics** -- 0 = all pass, 1 = failures, 2 = config/setup error (consistent across all commands)
- **`axiom init` respects configured provider** -- no longer hardcodes Anthropic
- **`init` and `add` respect `test_dir` config** -- uses `cfg.TestDir` instead of hardcoded `.axiom/`
- **Cache tracks provider and base_url** -- switching providers invalidates cached results
- **Getting-started docs restructured** -- leads with `axiom add` as the entry point

### Fixed

- Gemini `google/` prefix bug causing 404 errors on certain model name formats
- Goroutine leak in `toolGrep` and directory walk functions (now context-aware)
- Retry loop context cancellation (`break` inside `select` only broke the select, not the `for` loop)
- Exit code for `list` and `show` commands (setup errors now correctly return code 2)
- Hardcoded `.axiom/` in output (now shows actual configured `test_dir`)
- Double space in dry-run header output
- Cache corruption handled gracefully (warns on stderr instead of failing silently)
- `cache clear` respects custom `cache.dir` config
- Quiet mode now includes flaky test count in summary

## [0.1.1] - 2026-02-15

Major feature development phase. Added most of the CLI commands, caching, multi-provider support, and CI integration.

## [0.1.0] - 2026-01-15

Initial implementation of axiom.
