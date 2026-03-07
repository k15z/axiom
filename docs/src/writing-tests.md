# Writing Tests

Tests live in `.axiom/` as YAML files. Axiom discovers them recursively, so you can organize into subdirectories.

## Test Format

Each YAML file contains one or more tests as top-level keys:

```yaml
test_name:
  on:
    - src/**/*.go
  condition: >
    Description of what should be true about the code.
```

## Fields

| Field | Required | Description |
|-------|----------|-------------|
| `condition` | **Yes** | Plain-English assertion about the code |
| `on` | No | Glob patterns for cache triggers and agent starting hints |
| `tags` | No | List of tags for filtering (e.g., `[security, auth]`) |
| `model` | No | Override the LLM model for this test |
| `timeout` | No | Override the per-test timeout (seconds) |
| `max_iterations` | No | Override the max agent tool-use turns |

## The `condition` Field

This is the most important field. The agent reads it and explores your codebase to determine if it holds.

### Good Conditions

Good conditions are **specific**, **measurable**, and reference **concrete patterns**:

```yaml
test_path_traversal_prevention:
  on:
    - internal/agent/tools.go
  condition: >
    All agent tools that accept file paths (read_file, list_dir, glob, grep)
    must validate that resolved paths do not escape the repository root
    directory. There should be a shared path validation function that converts
    relative paths to absolute, then checks they are prefixed by the repo root.
    Paths like "../../../etc/passwd" must be rejected.
```

```yaml
test_atomic_balance_updates:
  on:
    - src/ledger/**/*.py
  condition: >
    All balance mutations must use SELECT FOR UPDATE within a transaction.
    No balance update should read the current value and then write without
    holding a row-level lock. Check for patterns where balance is read in
    one query and updated in a separate query without FOR UPDATE.
```

### Bad Conditions

Avoid vague, subjective, or unmeasurable conditions:

```yaml
# BAD: too vague
test_code_quality:
  condition: "The code should be clean and well-organized"

# BAD: subjective
test_good_naming:
  condition: "Variable names should be descriptive"

# BAD: no concrete pattern to verify
test_performance:
  condition: "The application should be fast"
```

## The `on` Field

`on` accepts a list of glob patterns. These serve two purposes:

1. **Cache key** -- axiom hashes the content of matching files. If none changed since the last passing run, the test is skipped.
2. **Starting hint** -- the agent is told to start its investigation with these files. It can read any file in the repo from there.

### Glob Syntax

| Pattern | Matches |
|---------|---------|
| `src/*.go` | Go files directly in `src/` |
| `src/**/*.go` | Go files anywhere under `src/` |
| `**/*.yml` | YAML files anywhere in the repo |
| `internal/agent/*.go` | Go files in the agent package |

### Tests Without `on`

If you omit `on`, the test:
- **Always runs** (never cached)
- **Has no starting hint** (agent starts from scratch)

Use `axiom validate` to identify tests missing `on` patterns:

```bash
axiom validate
```

## Tags

Add tags to organize and filter tests:

```yaml
test_auth_required:
  tags: [security, auth]
  on:
    - src/routes/**/*.py
  condition: >
    All route handlers must call verify_token() before accessing request.user.
```

Run only tests with a specific tag:

```bash
axiom run --tag security
```

Multiple tags (comma-separated, OR logic):

```bash
axiom run --tag "security,auth"
```

Tags are case-insensitive: `CI` matches `ci`.

## Per-Test Overrides

Override model, timeout, or iteration limits per test:

```yaml
test_complex_analysis:
  model: claude-sonnet-4-6
  timeout: 120
  max_iterations: 50
  on:
    - src/**/*.py
  condition: >
    The payment processing pipeline must handle all edge cases...
```

This is useful for:
- **Expensive tests** that need a more capable model
- **Complex tests** that need more iterations to explore
- **Slow tests** that need a longer timeout

## Agent Memory

Axiom's agent learns about your codebase across runs. On the first run, the agent explores thoroughly. On subsequent runs (if files haven't changed), the agent skips redundant exploration and goes straight to evaluation.

This caching of observations (called "notes") is automatic and transparent:

- **First run:** Agent explores broadly, builds notes about architecture, patterns, file locations
- **Subsequent runs:** Agent reads cached notes, checks what files changed, re-evaluates affected tests
- **When files change:** Notes for affected tests are automatically invalidated and rebuilt

The result: faster tests over time, and lower token usage after the initial run.

Notes are stored in `.axiom/.cache/notes.json` and can safely be gitignored (like the rest of the cache).

## Organizing Tests

Structure `.axiom/` by category:

```
.axiom/
  architecture.yml    # Layer boundaries, dependency rules
  security.yml        # Auth, input validation, secrets
  error-handling.yml  # Error wrapping, exit codes
  features/
    payments.yml      # Payment-specific invariants
    auth.yml          # Auth-specific invariants
```

All YAML files are discovered recursively. Hidden directories (starting with `.`) are skipped.

## Test Names

Test names must be unique across all files. Duplicate names cause a discovery error:

```
Error: duplicate test name "test_auth": defined in security.yml and features/auth.yml
```

Convention: use `test_` prefix with `snake_case` descriptive names.
