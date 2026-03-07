# Getting Started

This guide takes you from zero to running your first axiom test in under 5 minutes.

## Prerequisites

- Go 1.21 or later
- An Anthropic API key ([get one here](https://console.anthropic.com/))

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

## Run a Single Test

```bash
axiom run test_auth_middleware
```

## Run All Tests (Ignore Cache)

```bash
axiom run --all
```

## Write Your First Test

Create `.axiom/my_tests.yml`:

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

## Add a Test Interactively

Let the LLM generate a test from a description:

```bash
axiom add "all API routes require authentication"
```

Axiom explores your codebase, generates a test, and asks for confirmation before writing it.

## Next Steps

- [Writing Tests](./writing-tests.md) -- learn the YAML format and how to write effective conditions
- [Examples](./examples.md) -- curated test examples across security, architecture, and code quality
- [Configuration](./configuration.md) -- customize model, timeouts, and caching
- [CI Integration](./ci-integration.md) -- set up axiom in GitHub Actions
