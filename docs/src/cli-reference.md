# CLI Reference

## axiom run

Run behavioral tests.

```
axiom run [test-name] [flags]
```

If a test name is provided as a positional argument, only that test runs. This overrides `--filter`.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--all` | `-a` | `false` | Run all tests, ignoring cache |
| `--filter` | `-f` | | Run tests matching a glob pattern (e.g., `"test_auth*"`) |
| `--tag` | `-t` | | Run tests matching any of the given tags (comma-separated) |
| `--dir` | `-d` | `.axiom/` | Path to test directory |
| `--verbose` | `-v` | `false` | Show full agent reasoning for all tests |
| `--model` | `-m` | | Override the LLM model |
| `--bail` | `-b` | `false` | Stop on first failure |
| `--json` | | `false` | Output results as JSON (shorthand for `--format json`) |
| `--format` | | `text` | Output format: `text`, `json`, or `github` |
| `--concurrency` | `-c` | `0` (auto) | Number of tests to run in parallel |
| `--retries` | | `0` | Re-run failed tests up to N times; if a retry passes, mark as flaky |
| `--dry-run` | | `false` | Preview which tests would run and estimate token cost |
| `--watch` | `-w` | `false` | Watch for file changes and re-run affected tests |
| `--strict` | | `false` | Treat flaky tests (passed on retry) as failures |
| `--quiet` | `-q` | `false` | Suppress full output, only emit CI summary line to stderr |
| `--provider` | `-p` | | Override the LLM provider: `anthropic`, `openai`, or `gemini` |

### Examples

```bash
# Run all tests, ignoring cache
axiom run --all

# Run a single test by name
axiom run test_auth_middleware

# Run tests matching a pattern
axiom run --filter "test_auth*"

# Run only security-tagged tests
axiom run --tag security

# Run with 3 parallel tests, stop on first failure
axiom run -c 3 --bail

# Preview what would run without calling the API
axiom run --dry-run

# Output GitHub-flavored Markdown (for PR comments)
axiom run --format github

# Output JSON (for CI pipelines)
axiom run --json

# Retry failures once to detect flaky tests
axiom run --retries 1

# Use a more capable model for this run
axiom run --model claude-sonnet-4-6

# Watch for file changes and re-run affected tests
axiom run --watch

# Suppress output, just get exit code (for CI)
axiom run --quiet
```

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | All tests passed (or cached/skipped) |
| `1` | One or more tests failed |
| `2` | Configuration or setup error |

## axiom init

Analyze the codebase and generate behavioral tests.

```
axiom init
```

Requires an API key for your configured provider. Creates `.axiom/tests.yml` and `axiom.yml`. Fails if `.axiom/` already exists.

## axiom add

Generate a test from a natural-language description.

```
axiom add <intent> [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--model` | `-m` | | Override LLM model |
| `--file` | `-f` | `tests.yml` | Target YAML file inside `.axiom/` |
| `--run` | | `false` | Run the new test immediately after adding |

### Examples

```bash
axiom add "all API routes require authentication"
axiom add "no package imports from the CLI layer" --file architecture.yml
axiom add "database connections are always closed" -f db.yml
```

## axiom list

List all tests and their cached status.

```
axiom list [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | | Path to test directory |
| `--json` | | `false` | Output as JSON |

Shows each test with its status: `pending`, `cached (pass)`, `cached (fail)`, `stale (pass)`, `stale (fail)`. Includes a total test count.

## axiom show

Show cached reasoning from the last run.

```
axiom show [test-name] [flags]
```

If a test name is provided, shows only that test's reasoning. Otherwise shows all cached reasoning.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | | Path to test directory |
| `--json` | | `false` | Output as JSON |
| `--diff` | | `false` | Show diff against previous reasoning |

### Examples

```bash
# Show all cached reasoning
axiom show

# Show reasoning for a specific test
axiom show test_auth_middleware

# Output as JSON
axiom show --json
```

## axiom validate

Lint test YAML files for structural issues.

```
axiom validate [flags]
```

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--dir` | `-d` | | Path to test directory |

Checks for:
- Invalid glob syntax in `on` patterns
- Missing `on` patterns (tests that can never be cached)
- Very short conditions (likely too vague)

## axiom version

Print the axiom version.

```
axiom version
```

## axiom cache clear

Clear the test cache, forcing all tests to re-run.

```
axiom cache clear
```

Respects `cache.dir` from `axiom.yml` if configured.

## axiom cache info

Show cache statistics: entry count, file size, oldest/newest entries, and per-test pass/fail breakdown.

```
axiom cache info
```
