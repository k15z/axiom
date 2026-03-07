# CI Integration

Axiom is designed to run in CI pipelines. This page covers GitHub Actions setup, cache persistence, and PR comment integration.

## GitHub Action (Recommended)

The easiest way to run axiom in CI is with the reusable GitHub Action:

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
      - uses: k15z/axiom@main
        with:
          api_key: ${{ secrets.ANTHROPIC_API_KEY }}
```

This handles everything: installing axiom and Go, restoring/saving the cache, running tests, posting a PR comment with results, and failing the step if any tests fail.

### Action Inputs

| Input | Default | Description |
|-------|---------|-------------|
| `api_key` | *required* | API key for your provider |
| `provider` | `anthropic` | LLM provider: `anthropic`, `openai`, or `gemini` |
| `model` | | Override the model (uses `axiom.yml` default if omitted) |
| `test_dir` | `.axiom/` | Path to test directory |
| `go_version` | `1.21` | Go version to install |
| `comment` | `true` | Post results as a PR comment |
| `args` | | Extra arguments passed to `axiom run` |

### Using with OpenAI or Gemini

```yaml
- uses: k15z/axiom@main
  with:
    api_key: ${{ secrets.OPENAI_API_KEY }}
    provider: openai
```

## Manual Setup

For more control or other CI systems, run axiom directly.

### Basic Setup (Anthropic)

```yaml
name: Axiom Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  axiom:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install axiom
        run: go install github.com/k15z/axiom/cmd/axiom@latest

      - name: Run axiom tests
        run: axiom run --all
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

### Setup with OpenAI or Gemini

To use OpenAI or Gemini instead, set the appropriate environment variable:

```yaml
      - name: Run axiom tests
        run: axiom run --all
        env:
          OPENAI_API_KEY: ${{ secrets.OPENAI_API_KEY }}
```

And add `provider` and `model` to your `axiom.yml`:

```yaml
provider: openai
model: gpt-4o
```

For Gemini, use:
```yaml
provider: gemini
model: gemini-2.0-flash
```

See [Multi-Provider Support](./configuration.md#multi-provider-support) for details.

### With Cache Persistence

Persist `.axiom/.cache/` between CI runs so unchanged tests are skipped:

```yaml
name: Axiom Tests

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  axiom:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install axiom
        run: go install github.com/k15z/axiom/cmd/axiom@latest

      - name: Restore axiom cache
        uses: actions/cache@v4
        with:
          path: .axiom/.cache/
          key: axiom-${{ runner.os }}-${{ hashFiles('.axiom/**/*.yml') }}
          restore-keys: |
            axiom-${{ runner.os }}-

      - name: Run axiom tests
        run: axiom run
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

The cache key includes a hash of your test YAML files, so the cache is invalidated when tests change. The `restore-keys` fallback reuses the most recent cache even if test definitions changed (axiom's content-hash check handles the rest).

### With PR Comment Summaries

Post test results as a PR comment using `--format github`:

```yaml
name: Axiom Tests

on:
  pull_request:
    branches: [main]

permissions:
  pull-requests: write

jobs:
  axiom:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Install axiom
        run: go install github.com/k15z/axiom/cmd/axiom@latest

      - name: Restore axiom cache
        uses: actions/cache@v4
        with:
          path: .axiom/.cache/
          key: axiom-${{ runner.os }}-${{ hashFiles('.axiom/**/*.yml') }}
          restore-keys: |
            axiom-${{ runner.os }}-

      - name: Run axiom tests
        id: axiom
        run: |
          axiom run --format github > axiom-results.md 2>/dev/null || true
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}

      - name: Post PR comment
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          path: axiom-results.md

      - name: Check for failures
        run: axiom run
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

This workflow:
1. Runs axiom and captures the GitHub Markdown output
2. Posts (or updates) a sticky PR comment with the results
3. Runs axiom again to get the correct exit code (the first run's exit code was suppressed for comment posting)

The `<!-- axiom-summary -->` HTML comment in the output lets `sticky-pull-request-comment` find and replace previous comments instead of creating duplicates.

## Exit Codes

Axiom uses distinct exit codes so CI can distinguish between test failures and setup problems:

| Code | Meaning | CI Action |
|------|---------|-----------|
| `0` | All tests passed | Pipeline passes |
| `1` | One or more tests failed | Pipeline fails |
| `2` | Configuration error (missing API key, bad YAML) | Pipeline fails (fix config) |

## CI Output

In non-TTY environments (CI), axiom prints progress lines to stderr as tests complete, plus a greppable summary:

```
axiom: 8 passed, 1 failed, 1 cached
```

## Cost Management

Use `--dry-run` to preview costs before committing to a CI run:

```bash
axiom run --dry-run
```

This shows which tests would run, which are cached, and a worst-case token cost estimate.

## GitLab CI

```yaml
axiom:
  image: golang:1.21
  script:
    - go install github.com/k15z/axiom/cmd/axiom@latest
    - axiom run --all
  variables:
    ANTHROPIC_API_KEY: $ANTHROPIC_API_KEY
  cache:
    paths:
      - .axiom/.cache/
```

## General CI Tips

- **Cache `.axiom/.cache/`** between runs to avoid re-running unchanged tests
- **Use `--format github`** on pull requests for readable summaries
- **Use `--bail`** to fail fast and save API costs on obviously broken branches
- **Use `--tag`** to run different test suites on different triggers (e.g., security tests on main only)
- **Set `--concurrency`** to 3-5 for faster runs (default auto-detects)
- **Add `--retries 1`** to detect flaky tests without failing the build
