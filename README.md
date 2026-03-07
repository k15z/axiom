# axiom

AI-driven behavioral tests for your codebase. Write plain-English conditions in YAML, and axiom verifies them against your source code using an agentic LLM.

## Why

Unit tests verify *implementation*. Axiom tests verify *intent*. Define invariants, architectural constraints, and race-condition guards in natural language — axiom's agent explores your code and tells you if they hold.

## Quick Start

```bash
go install github.com/k15z/axiom/cmd/axiom@latest

axiom init        # creates .axiom/ with a sample test and axiom.yml config
axiom run                        # runs all tests (only re-runs if trigger files changed)
axiom run test_auth_middleware   # run a single test by name
axiom run --all                  # runs all tests regardless of cache
```

Set `ANTHROPIC_API_KEY` in your environment or a `.env` file at the project root — axiom loads it automatically.

## Test Format

Tests live in `.axiom/` as YAML files (recursively discovered). Each file can contain multiple tests:

```yaml
test_double_finalization_prevention:
  on:
    - orchestrator/app/*.py
  condition: >
    There must be a compare-and-swap (CAS) or equivalent mechanism that prevents a market
    from being finalized twice under concurrent requests. If two validator verdicts arrive
    simultaneously and both trigger consensus, only one should result in a RESOLVED market
    and payout insertion. The second attempt must detect that the market is already RESOLVED
    and exit gracefully without inserting duplicate payouts. This likely uses
    "UPDATE ... WHERE status = RESOLVING RETURNING id" or a similar atomic check. Verify
    the race condition cannot result in double payouts.
```

### Fields

| Field | Required | Description |
|-------|----------|-------------|
| `on` | No | Glob patterns that serve as both the cache trigger and a starting hint for the agent. If omitted, the test always runs. |
| `condition` | **Yes** | Plain-English assertion about the code. The agent explores the codebase and determines pass/fail. |

### How `on` works

`on` globs serve two purposes:

1. **Cache key** — axiom hashes the content of matching files. If none changed since the last passing run, the test is skipped.
2. **Starting hint** — the agent is told to start its investigation with these patterns. It can read any file in the repo from there.

Failed tests always re-run, regardless of whether trigger files changed.

Tests can be organized into subdirectories (e.g. `.axiom/security/`, `.axiom/architecture/`) — axiom discovers them recursively.

## CLI

```
axiom <command> [flags]

Commands:
  init                  Initialize axiom in the current project
  run [test-name]       Run behavioral tests (optionally a single test by name)
  list                  List all tests and their cached status
  cache clear           Clear the cache, forcing all tests to re-run

Flags (run):
  -a, --all                Run all tests, ignoring cache
  -f, --filter string      Run tests matching a glob pattern (e.g. "test_auth*")
  -d, --dir string         Path to test directory (default: .axiom/)
  -v, --verbose            Show full agent reasoning for all tests
  -m, --model string       LLM model to use (overrides config)
  -c, --concurrency int    Number of tests to run in parallel (default: 1)
  -b, --bail               Stop on first failure
      --json               Output results as JSON (for CI)
  -w, --watch              Watch for file changes and re-run affected tests
```

### CI Usage

Exit codes:

| Code | Meaning |
|------|---------|
| `0` | All tests passed (or cached/skipped) |
| `1` | One or more tests failed |
| `2` | Configuration or setup error (missing API key, bad YAML, test dir not found) |

This lets CI distinguish "tests failed" from "axiom is broken":

```yaml
# GitHub Actions
- name: Run axiom tests
  run: axiom run --all
  env:
    ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

In non-TTY environments (CI), axiom prints per-test progress lines to stderr as tests complete, plus a greppable summary line: `axiom: 8 passed, 1 failed, 1 cached`.

## Configuration

`axiom.yml` at the project root (optional):

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
```

`ANTHROPIC_API_KEY` can be set in the environment or a `.env` file at the project root. Existing environment variables take precedence over `.env`.

## Caching

LLM calls are slow — caching is essential. Axiom uses content hashes to skip tests whose trigger files haven't changed.

Cache state lives in `.axiom/.cache/results.json`:

```json
{
  "test_name": {
    "last_run": "2025-01-01T00:00:00Z",
    "file_hashes": { "src/foo.py": "sha256..." },
    "result": "pass"
  }
}
```

A test is skipped when: it passed last run **and** no file matching `on` has changed.

## How It Works

1. **Discovery** — recursively scan `.axiom/` for YAML files, parse all test definitions.
2. **Cache check** — hash files matching `on` globs, compare to stored hashes. Skip if unchanged and previously passed.
3. **Agent evaluation** — an LLM agent receives the condition and `on` globs as a starting hint, then uses tools to explore the codebase until it can make a determination.
4. **Result** — agent responds `VERDICT: PASS` or `VERDICT: FAIL` with reasoning. Cache is updated. Exit code reflects overall result.

### Agent Tools

The agent can call these tools as many times as needed:

| Tool | Description |
|------|-------------|
| `read_file` | Read a file with line numbers |
| `glob` | Find files matching a pattern (supports `**`) |
| `grep` | Search file contents by regex |
| `list_dir` | List directory contents |

All tools are sandboxed to the repository root — path traversal is rejected.

## Output

Passing tests show a one-line summary by default. Use `--verbose` for full reasoning.

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

## Project Structure

```
axiom/
├── axiom.yml              # project config (model, agent settings, cache)
├── .axiom/                # test definitions (recursively discovered)
│   ├── architecture.yml
│   ├── security.yml
│   └── features/          # subdirectories supported
│       └── auth.yml
├── cmd/axiom/main.go
└── internal/
    ├── agent/             # agentic tool loop
    ├── cache/             # content-hash cache
    ├── cli/               # cobra commands
    ├── config/            # config + .env loading
    ├── discovery/         # recursive YAML test parsing
    ├── display/           # live terminal spinner
    ├── output/            # terminal + JSON output
    └── runner/            # orchestration + parallel execution
```
