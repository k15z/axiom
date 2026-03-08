# axiom

[![tested with axiom](https://img.shields.io/badge/tested%20with-axiom-green)](https://github.com/k15z/axiom)

Behavioral tests for your codebase. Write invariants in plain English, and axiom's LLM agent verifies them against your source code.

```yaml
test_all_routes_require_auth:
  on:
    - src/routes/**/*.py
    - src/middleware/**/*.py
  condition: >
    All route handlers that access user data must require authentication.
    Public endpoints (health checks, login, registration) are exempt.
```

```
  ✗ test_all_routes_require_auth (9.2s)
    Route GET /admin/users in src/routes/admin.py bypasses auth middleware —
    it accesses request.user without calling verify_token().
```

Unit tests verify that functions work. Axiom tests verify that properties hold -- architectural boundaries, security invariants, concurrency safety, error handling contracts. These break silently during refactors and get caught in code review, when someone remembers to look. Axiom runs them automatically.

## Install

```bash
# One-line install (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/k15z/axiom/main/install.sh | sh

# Homebrew
brew tap k15z/homebrew-tap && brew install axiom

# Go
go install github.com/k15z/axiom/cmd/axiom@latest
```

## Quick Start

```bash
export ANTHROPIC_API_KEY=sk-ant-...   # or add to .env

# Add and run a test in one command
axiom add "all API routes require authentication" --run

# Or generate a batch of tests from your codebase
axiom init
axiom run                        # runs tests (skips unchanged)
axiom run --all                  # ignores cache
```

Axiom loads `.env` automatically. A typical test costs $0.01--0.05 with Haiku.

## Test Format

Tests live in `.axiom/` as YAML files, discovered recursively. Each file can contain multiple tests:

```yaml
test_no_sql_injection:
  on:
    - src/database/**/*.py
  condition: >
    All database queries must use parameterized statements or an ORM.
    No raw string interpolation or f-strings should be used to construct
    SQL queries.
```

| Field | Required | Description |
|-------|----------|-------------|
| `condition` | **Yes** | Plain-English assertion the agent evaluates against your code |
| `on` | No | Glob patterns -- cache trigger and starting hint for the agent |
| `tags` | No | Filter with `--tag` (e.g. `tags: [security, auth]`) |
| `model` | No | Override the model for this test |
| `timeout` | No | Per-test timeout in seconds |
| `max_iterations` | No | Max agent tool-use turns |

`on` globs serve two purposes: they determine the cache key (axiom hashes matching files and skips tests when nothing changed) and they tell the agent where to start exploring. The agent can read any file in the repo from there. Failed tests always re-run.

## CI

The fastest way to add axiom to CI:

```yaml
# .github/workflows/axiom.yml
name: Axiom Tests
on: [pull_request]

permissions:
  pull-requests: write

jobs:
  axiom:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: k15z/axiom@v0
        with:
          api_key: ${{ secrets.ANTHROPIC_API_KEY }}
```

This installs axiom, restores the cache, runs tests, posts a PR comment with results, and fails the step on test failures.

Exit codes: `0` all passed, `1` test failures, `2` setup error.

## CLI

```
axiom run [test-name]       Run tests (cached by default)
axiom run --all             Ignore cache, run everything
axiom run --tag security    Run tests matching a tag
axiom run --watch           Re-run on file changes
axiom run --dry-run         Preview what would run + cost estimate
axiom add <intent>          Generate a test from a description
axiom add <intent> --run    Generate and run immediately
axiom init                  Generate tests from your codebase
axiom list                  List tests and cached status
axiom show <test-name>      Show cached reasoning
axiom show --diff           Compare reasoning against previous run
axiom validate              Lint test YAML for issues
axiom cache clear           Clear cache, force re-run
axiom cache info            Show cache statistics
```

Key flags: `--verbose` (full reasoning), `--bail` (stop on first failure), `--json` (JSON output), `--quiet` (CI summary only), `--retries N` (flaky detection), `--model` (override model), `--provider` (anthropic/openai/gemini).

## Configuration

`axiom.yml` at the project root (all fields optional):

```yaml
model: claude-haiku-4-5-20251001   # default model
test_dir: .axiom/

cache:
  enabled: true
  dir: .axiom/.cache/

agent:
  max_iterations: 30    # max tool-use turns per test
  max_tokens: 10000     # max tokens per LLM response
  timeout: 0            # per-test timeout in seconds (0 = no timeout)
  tool_timeout: 30      # per-tool timeout in seconds
```

### Providers

Axiom supports Anthropic (default), OpenAI, Gemini, and OpenAI-compatible endpoints. The provider is auto-detected from the model name, or set explicitly:

```yaml
# OpenAI
provider: openai
model: gpt-4o

# Gemini
provider: gemini
model: gemini-2.0-flash

# OpenAI-compatible (Ollama, vLLM, etc.)
provider: openai
model: llama3
base_url: http://localhost:11434/v1
```

Set the matching API key: `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, or `GEMINI_API_KEY`.

## How It Works

1. **Discovery** -- scan `.axiom/` for YAML test files
2. **Cache check** -- hash files matching `on` globs, skip unchanged passing tests
3. **Agent evaluation** -- the agent explores your codebase using read_file, grep, glob, and list_dir until it reaches a verdict
4. **Result** -- `VERDICT: PASS` or `VERDICT: FAIL` with reasoning. Cache updated. Exit code set.

All agent tools are sandboxed to the repository root. The agent builds notes about your codebase across runs (stored in `.axiom/.cache/notes.json`), skipping redundant exploration on subsequent runs. Notes are invalidated when referenced files change.

## Output

```
  axiom

  .axiom/concurrency.yml
    ✓ test_double_finalization_prevention (8.4s)
      CAS mechanism found via UPDATE...WHERE status = RESOLVING RETURNING id
    ✓ test_atomic_balance_updates (6.1s)
      All balance mutations use SELECT FOR UPDATE within a transaction

  .axiom/auth.yml
    ✗ test_auth_middleware (9.2s)
      Route handler GET /admin/users in src/routes/admin.py bypasses
      auth middleware — it accesses request.user without verify_token().

    ○ test_rate_limiting (cached)

  3 passed · 1 failed · 1 cached
```

## Documentation

Full docs at [k15z.github.io/axiom](https://k15z.github.io/axiom/) -- writing tests, examples, CI integration (GitHub Actions, GitLab, CircleCI, Jenkins), configuration, agent memory, and troubleshooting.
