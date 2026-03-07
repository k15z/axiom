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

### Behavioral vs Implementation Tests

The most important principle for writing good axiom tests: **assert behavior, not implementation**. Tests should describe *what* property should hold, not *how* it is implemented. This makes tests robust to refactoring -- if someone renames a function, restructures packages, or changes an approach, the behavioral property is still testable.

**Good (behavioral):** Describes the property that matters.

```yaml
test_agent_is_leaf_dependency:
  on:
    - internal/agent/**/*.go
  condition: >
    The agent package should be a leaf dependency -- it should not import
    any higher-level packages from this project (such as CLI commands, test
    runners, output formatting, or test discovery).
```

**Bad (implementation-specific):** Hardcodes specific package names the agent shouldn't import.

```yaml
test_agent_has_no_upstream_imports:
  on:
    - internal/agent/*.go
  condition: >
    The agent package must not import any of these packages:
    internal/cli, internal/runner, internal/output, internal/discovery,
    or internal/cache. Check all import blocks in internal/agent/.
```

The good version lets the agent figure out which packages are "higher-level." If you add a new package later, the test still works. The bad version requires manual updates every time the package list changes.

More examples:

| Bad (implementation) | Good (behavioral) |
|---|---|
| "Functions should wrap errors using `fmt.Errorf('desc: %w', err)`" | "Error messages should include enough context to diagnose where the problem originated" |
| "Check that each exported func has a comment on the line immediately before it" | "All public API surfaces should be documented" |
| "The CLI should wrap errors in a SetupError type and check using errors.As" | "Setup errors should cause a different exit code than test failures" |
| "There should be a shared path validation function that checks the repo root prefix" | "Agent tools that accept file paths must prevent path traversal outside the repo" |

### What NOT to Test with Axiom

Some properties are better enforced by other tools:

- **Circular imports** -- already caught by `go build` (or your language's compiler)
- **Code formatting** -- use `gofmt`, `prettier`, etc.
- **Type correctness** -- that is what compilers and type checkers are for
- **Specific function behavior** -- use unit tests
- **Linting rules** -- use `golangci-lint`, `eslint`, etc.

Axiom is best for **cross-cutting behavioral properties** that span multiple files and are hard to express as unit tests or lint rules.

### Good Conditions

Good conditions are **specific**, **behavioral**, and describe **what should be true**:

```yaml
test_agent_tools_prevent_path_traversal:
  on:
    - internal/agent/**/*.go
  condition: >
    Agent tools that accept file paths must prevent path traversal attacks.
    Resolved paths should be confined to the repository root -- attempts to
    access files outside the repo (e.g., "../../../etc/passwd") must be rejected.
```

```yaml
test_atomic_balance_updates:
  on:
    - src/ledger/**/*.py
  condition: >
    All balance mutations must happen atomically. A balance should never
    be read and then updated in separate operations without holding a lock,
    as this creates race conditions.
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

Also avoid over-specified conditions that dictate implementation details:

```yaml
# BAD: prescribes exact implementation
test_errors:
  condition: >
    Functions must wrap errors using fmt.Errorf("description: %w", err).
    Check that all error return paths use the %w verb.

# BAD: lists specific names that may change
test_imports:
  condition: >
    The agent package must not import internal/cli, internal/runner,
    internal/output, internal/discovery, or internal/cache.
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
