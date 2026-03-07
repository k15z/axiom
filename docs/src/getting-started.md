# Getting Started

This guide takes you from zero to running your first axiom test in under 5 minutes.

## Prerequisites

- Go 1.25 or later
- An API key from a supported provider:
  - [Anthropic](https://console.anthropic.com/) (default)
  - [OpenAI](https://platform.openai.com/api-keys)
  - [Google Gemini](https://aistudio.google.com/apikey)

## Install

```bash
go install github.com/k15z/axiom/cmd/axiom@latest
```

Verify the installation:

```bash
axiom --help
```

## Set Your API Key

Create a `.env` file in your project root:

```bash
echo "ANTHROPIC_API_KEY=sk-ant-..." > .env
```

Or export it directly:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Axiom loads `.env` automatically. Existing environment variables take precedence.

## Initialize

From your project root, run:

```bash
axiom init
```

This uses an LLM to analyze your codebase and generate relevant behavioral tests. It creates:

- `.axiom/tests.yml` -- generated test definitions
- `axiom.yml` -- project configuration

## Run Tests

```bash
axiom run
```

Axiom discovers all YAML files in `.axiom/`, checks the cache, and runs any tests whose trigger files have changed. On the first run, all tests execute.

You'll see output like:

```
  axiom

  .axiom/tests.yml
    ✓ test_no_sql_injection (8.4s)
      All database queries use parameterized statements via the ORM
    ✗ test_auth_middleware (9.2s)
      Route GET /admin/users bypasses auth -- accesses request.user without verify_token()
    ○ test_rate_limiting (cached)

  2 passed · 1 failed · 1 cached
```

## Add Your Own Test

This is where axiom really shines. Describe what you want to test in plain English, and axiom generates the test for you:

```bash
axiom add "all API routes require authentication"
```

Axiom explores your codebase, understands the relevant code, generates a test with proper `on` globs and a specific condition, and asks for confirmation before writing it. You can also run the test immediately:

```bash
axiom add "database connections are always closed" --run
```

This is the fastest way to go from an idea to a verified property of your codebase.

## Validate Your Tests

Before running tests (especially after `init` or `add`), validate them:

```bash
axiom validate
```

This catches common issues before you spend API calls: invalid glob syntax, missing `on` patterns (tests that can never be cached), and vague conditions. Make this a habit.

## Preview Before Running

See what would run and the estimated token cost without calling the API:

```bash
axiom run --dry-run
```

This is especially useful before your first real run or in CI to estimate costs.

## Run a Single Test

```bash
axiom run test_auth_middleware
```

## Run All Tests (Ignore Cache)

```bash
axiom run --all
```

## Write Tests Manually

You can also write tests by hand. Create `.axiom/my_tests.yml`:

```yaml
test_readme_exists:
  condition: >
    The project must have a README.md file in the root directory
    that includes installation instructions and a usage section.
```

Run it:

```bash
axiom run test_readme_exists
```

See [Writing Tests](./writing-tests.md) for the full YAML format and tips on writing effective conditions.

## Watch Mode

Re-run affected tests automatically when source files change:

```bash
axiom run --watch
```

Axiom watches files matching your tests' `on` globs and re-runs only the affected tests when changes are detected. Press Ctrl+C to stop.

## Next Steps

- [Writing Tests](./writing-tests.md) -- learn the YAML format and how to write effective conditions
- [Examples](./examples.md) -- curated test examples across security, architecture, and code quality
- [Configuration](./configuration.md) -- customize model, providers, timeouts, and caching
- [CI Integration](./ci-integration.md) -- set up axiom in GitHub Actions
