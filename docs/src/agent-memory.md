# Agent Memory

Axiom's agent builds notes about your codebase as it explores during test runs. These notes persist across runs, allowing the agent to skip redundant exploration and go straight to evaluating what changed. This reduces token costs and speeds up subsequent runs.

## How It Works

On the **first run**, the agent explores your codebase thoroughly -- reading files, searching for patterns, and building an understanding of your project's architecture. It records its observations as notes.

On **subsequent runs**, the agent reads its cached notes before starting. If the relevant files haven't changed, it skips re-exploration and focuses on evaluating the test condition against its existing knowledge.

When **files change**, notes that reference those files are automatically invalidated. The agent re-explores the affected areas and updates its notes.

### What Gets Noted

The agent stores two kinds of observations:

- **Codebase-level notes** -- architecture patterns, package structure, common conventions, key file locations. These help the agent orient itself across all tests.
- **Per-test notes** -- specific file contents, function signatures, implementation details relevant to individual test conditions. These speed up re-evaluation of specific tests.

Each note tracks which files it references. When any referenced file changes (detected via content hash), the note is marked stale and rebuilt on the next run.

## Storage

Notes are stored in `.axiom/.cache/notes.json` alongside the test result cache. This file is:

- **Auto-generated** -- you never need to create or edit it manually
- **Safe to gitignore** -- it's machine-specific and rebuilds automatically
- **Included in CI cache** -- if you cache `.axiom/.cache/` in CI (recommended), notes carry over between runs

## Cost Savings

The impact depends on your codebase size and test count:

- **First run**: Full exploration. This is the most expensive run.
- **Second run (no changes)**: Significantly cheaper. The agent reads cached notes and skips most tool calls.
- **Subsequent runs (with changes)**: Only files matching changed `on` globs trigger re-exploration. Unchanged tests use cached notes.

The more tests you have, the more you benefit -- codebase-level notes are shared across all tests.

## Debugging Notes

### Viewing Notes

Notes are stored as JSON in `.axiom/.cache/notes.json`. You can inspect them directly:

```bash
cat .axiom/.cache/notes.json | python3 -m json.tool
```

### Stale Notes

If a test seems to be using outdated information (e.g., reporting on code that no longer exists), the note staleness tracking may have missed a file. This can happen if:

- The test's `on` globs don't cover all relevant files
- A file was renamed rather than modified

**Fix:** Clear the cache and re-run:

```bash
axiom cache clear
axiom run
```

### Notes Growing Too Large

Over many runs, notes can accumulate. If `.axiom/.cache/notes.json` becomes very large, clear it:

```bash
axiom cache clear
```

This clears both test results and notes. The agent will rebuild notes on the next run.

## When to Clear Notes

Clear notes (via `axiom cache clear`) when:

- You've done a major refactor that changed the project structure
- Tests are producing stale or incorrect reasoning
- You've renamed many files (staleness tracking is hash-based, not path-based)
- You want a clean baseline for cost measurement

You do **not** need to clear notes when:

- You make normal code changes -- staleness tracking handles this automatically
- You add or remove tests -- notes are per-test and independent
- You change `axiom.yml` settings -- config changes invalidate test results separately
