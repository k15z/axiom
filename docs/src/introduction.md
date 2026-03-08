# Axiom

You refactored the auth module last week. Unit tests passed. But a route handler now bypasses authentication because someone moved the middleware registration. Nobody noticed until it hit production.

Axiom catches this. Write the invariant once in plain English:

```yaml
test_all_routes_require_auth:
  on:
    - src/routes/**/*.py
    - src/middleware/**/*.py
  condition: >
    All route handlers that access user data must require authentication.
    Public endpoints (health checks, login, registration) are exempt.
```

Axiom's agent reads your codebase, explores the relevant files, and tells you whether the property holds -- with specific file paths and line numbers when it doesn't.

```
  ✗ test_all_routes_require_auth (9.2s)
    Route GET /admin/users in src/routes/admin.py bypasses auth middleware —
    it accesses request.user without calling verify_token().
```

This runs on every commit. The agent caches its work, so unchanged code is skipped.

## What Axiom Tests

Unit tests verify that functions return the right output. Axiom tests verify that properties hold across your codebase:

- **Architectural boundaries** -- "The agent package must not import from the CLI layer"
- **Security invariants** -- "All file paths must be validated against directory traversal"
- **Concurrency safety** -- "Concurrent market finalization must use compare-and-swap"
- **Error handling contracts** -- "Setup errors must produce exit code 2, not exit code 1"

These properties break silently during refactors. Senior engineers catch them in code review -- when they remember to look. Axiom encodes those checks so they run automatically.

## How It Works

1. You write a test condition in plain English inside a YAML file
2. Axiom's agent explores your codebase using tools (file reading, grep, glob)
3. The agent determines whether the condition holds
4. You get a pass/fail verdict with file paths and line numbers

The agent can read any file in your repo. The `on` globs tell it where to start and which files to watch for caching.

Tests are cached by content hash -- if the files haven't changed since the last passing run, the test is skipped. A typical test costs $0.01--0.05 with Haiku.
