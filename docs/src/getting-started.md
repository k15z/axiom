# Getting Started

Three commands, two minutes, one working test.

## 1. Install

```bash
curl -fsSL https://raw.githubusercontent.com/k15z/axiom/main/install.sh | sh
```

Also available via [Homebrew](https://brew.sh/) (`brew tap k15z/homebrew-tap && brew install axiom`) or [Go](https://go.dev/) (`go install github.com/k15z/axiom/cmd/axiom@latest`).

## 2. Set Your API Key

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Get a key from [Anthropic](https://console.anthropic.com/). Axiom also supports [OpenAI](https://platform.openai.com/api-keys) and [Gemini](https://aistudio.google.com/apikey) -- see [Configuration](./configuration.md#multi-provider-support).

You can also put the key in a `.env` file in your project root. Axiom loads it automatically.

## 3. Run Your First Test

From your project root:

```bash
axiom add "all API routes require authentication" --run
```

Axiom generates a test from your description, writes it to `.axiom/tests.yml`, and runs it immediately:

```
  axiom

  .axiom/tests.yml
    ✗ test_api_routes_require_authentication (9.2s)
      Route GET /admin/users in src/routes/admin.py bypasses auth
      middleware — accesses request.user without verify_token().

  0 passed · 1 failed
```

That's it. You have a behavioral test that runs on every commit.

Without `--run`, axiom shows the generated YAML and asks for confirmation before writing. A typical test costs $0.01--0.05 with Haiku.

---

## What's Next

### Write more tests

Describe another property, or write YAML by hand:

```bash
axiom add "no package imports from the CLI layer"
```

```yaml
# .axiom/security.yml
test_no_sql_injection:
  on:
    - src/database/**/*.py
  condition: >
    All database queries must use parameterized statements or an ORM.
    No raw string interpolation should construct SQL queries.
```

The `on` field tells axiom which files to watch -- when they change, the test re-runs. When they don't, results are cached and skipped. See [Writing Tests](./writing-tests.md) for the full format.

### Generate tests from your codebase

```bash
axiom init
```

Axiom analyzes your project and generates a batch of tests in `.axiom/tests.yml`.

### Run and manage tests

```bash
axiom run                        # run tests (skip cached)
axiom run --all                  # ignore cache, run everything
axiom run test_auth_middleware   # run a single test
axiom run --dry-run              # preview what would run + cost estimate
axiom run --watch                # re-run on file changes
axiom validate                   # check test YAML for problems
```

### Add to CI

```yaml
# .github/workflows/axiom.yml
- uses: k15z/axiom@v0
  with:
    api_key: ${{ secrets.ANTHROPIC_API_KEY }}
```

See [CI Integration](./ci-integration.md) for GitHub Actions, GitLab, CircleCI, and Jenkins.

### Go deeper

- [Thinking in Axiom](./mental-model.md) -- when to use behavioral tests and how to write good ones
- [Examples](./examples.md) -- real tests across Go, Python, TypeScript, Java, and Rust
- [Configuration](./configuration.md) -- models, providers, timeouts, and caching
- [CLI Reference](./cli-reference.md) -- every command and flag
