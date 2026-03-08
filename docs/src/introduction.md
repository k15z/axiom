# Axiom

You refactored the auth module last week. The tests passed. The PR got approved. But buried in the diff, a route handler lost its middleware registration -- and now `/admin/users` serves data without checking credentials.

Nobody catches this until a security review three months later. Or a penetration test. Or worse.

This is the class of bug axiom exists for. You write the invariant once:

```yaml
test_all_routes_require_auth:
  on:
    - src/routes/**/*.py
    - src/middleware/**/*.py
  condition: >
    All route handlers that access user data must require authentication.
    Public endpoints (health checks, login, registration) are exempt.
```

Axiom's agent reads your code, traces the relevant paths, and tells you exactly what broke:

```
  ✗ test_all_routes_require_auth (9.2s)
    Route GET /admin/users in src/routes/admin.py bypasses auth middleware —
    it accesses request.user without calling verify_token().
```

This runs on every commit. Unchanged code is cached and skipped. A typical test costs $0.01--0.05 with Haiku.

## Not Unit Tests -- Behavioral Tests

Unit tests verify that functions return the right output. Axiom tests verify that *properties hold across your codebase* -- the kind of things senior engineers check in code review, when they remember to look:

- "All route handlers require authentication"
- "No user input reaches SQL queries without parameterization"
- "The agent package doesn't import from the CLI layer"
- "Error messages include enough context to diagnose the source"

These properties span multiple files. They break silently during refactors. They can't be expressed as lint rules or unit tests. Axiom encodes them so they run automatically.

## How It Works

You write a test condition in plain English inside a YAML file. Axiom's agent explores your codebase using file reading, grep, and glob tools. It determines whether the condition holds and returns a pass/fail verdict with specific file paths and line numbers. The `on` globs tell the agent where to start and drive cache invalidation -- but the agent can read any file in the repo.

## Get Started

Install axiom and run your first test in under two minutes: [Getting Started](./getting-started.md).

Or read [Thinking in Axiom](./mental-model.md) to understand when and why to use behavioral tests.
