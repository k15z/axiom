# Axiom

Axiom is an AI-driven behavioral testing framework for codebases. Write plain-English conditions in YAML, and axiom verifies them against your source code using an LLM agent.

## Why Axiom?

Unit tests verify *implementation*. Axiom tests verify *intent*.

Some properties of a codebase are hard to capture with traditional tests:

- **Architectural constraints** -- "The agent package must not import from the CLI layer"
- **Security invariants** -- "All file path inputs must be validated against directory traversal"
- **Design patterns** -- "Every database query must use parameterized statements"
- **Race condition guards** -- "Concurrent market finalization must use compare-and-swap"

These are the things that break silently during refactors, that senior engineers catch in code review, and that new team members don't know to check. Axiom automates this.

## How It Works

1. You write a test condition in plain English inside a YAML file
2. Axiom's LLM agent reads your codebase using tools (file reading, grep, glob)
3. The agent explores until it can determine if the condition holds
4. You get a pass/fail verdict with specific file paths and line numbers

```yaml
test_no_sql_injection:
  on:
    - src/database/**/*.py
  condition: >
    All database queries must use parameterized statements or an ORM.
    No raw string interpolation or f-strings should be used to construct
    SQL queries. Check for patterns like f"SELECT ... {variable}" or
    "SELECT ... " + variable.
```

## Key Features

- **YAML test definitions** with natural language conditions
- **Content-hash caching** -- skip tests when trigger files haven't changed
- **Parallel execution** with configurable concurrency
- **CI-friendly** -- exit codes, JSON output, GitHub PR comment format
- **Per-test overrides** for model, timeout, and iteration limits
- **Tag-based filtering** to run subsets of tests
- **Flaky test detection** with automatic retries
