# Thinking in Axiom

Axiom tests are not unit tests. They're closer to automated code review -- a senior engineer who checks the same invariants on every commit, never forgets, and shows their work.

This page explains the mental model. Read it before writing your first test.

## Intent, Not Implementation

A good axiom test describes a property that should be true about your codebase, not how that property is implemented.

**Intent:** "Setup errors should produce a different exit code than test failures."
**Implementation:** "The CLI should wrap errors in a SetupError type and check using errors.As."

The intent version survives refactors. If someone changes the error type name or switches to a different error-handling pattern, the property is still testable. The implementation version breaks the moment anything changes.

This is the single most important thing to internalize. When you write a condition, ask: "If someone refactored the code completely but preserved the behavior, would this test still make sense?"

## What Axiom Is Good At

Axiom works best for **cross-cutting behavioral properties** -- things that span multiple files and can't be captured by a single unit test or lint rule:

- "All route handlers require authentication" (spans every route file)
- "Errors include enough context to diagnose the source" (spans every package)
- "The agent package doesn't import from the CLI layer" (an architectural boundary)
- "No user input reaches SQL queries without parameterization" (a security invariant)

These properties have two things in common: they involve relationships between parts of the codebase, and they break silently during refactors.

## What Axiom Is Not Good At

Some things are cheaper and better handled by other tools:

- **Formatting and style** -- use `gofmt`, `prettier`, `black`
- **Known code patterns** -- use Semgrep or your linter
- **Type correctness** -- that's what compilers are for
- **Function-level behavior** -- use unit tests
- **Circular imports** -- your build tool already catches these

If a property can be expressed as a lint rule, express it as a lint rule. Axiom costs money per run. Use it for the things only a human (or LLM) can verify.

## The Cost Model

Every test run makes API calls to an LLM. Costs depend on codebase size, test complexity, and model choice:

- **Haiku:** $0.01--0.05 per test. Use for daily CI and simple conditions.
- **Sonnet:** $0.05--0.20 per test. Use for complex architectural analysis.
- **Opus:** $0.20+ per test. Use for deep security audits.

Caching makes this manageable. After the first run, tests whose trigger files haven't changed are skipped entirely -- zero cost. On a typical project, most tests are cached on most runs.

Use `axiom run --dry-run` to see what would run and the estimated cost before committing to an API call.

## Writing Falsifiable Conditions

A good condition is **falsifiable** -- there's a clear way for the agent to determine it doesn't hold. "The code should be well-organized" is not falsifiable. "The agent package should not import from the CLI layer" is: the agent checks import statements and either finds a violation or doesn't.

Rules of thumb:

- **Name the property, not the implementation.** "All errors include diagnostic context" instead of "all errors use `fmt.Errorf` with `%w`."
- **Be specific about scope.** "All route handlers in src/routes/" is better than "all handlers." Narrow scope makes the agent faster and the test cheaper.
- **Describe what should be true, not what shouldn't exist.** "Database queries use parameterized statements" is better than "there are no SQL injection vulnerabilities." The positive framing gives the agent something concrete to verify.
- **Include one concrete example if it helps.** "Concurrent writes must use compare-and-swap (e.g., `UPDATE ... WHERE status = X RETURNING id`)" helps the agent know what pattern to look for without hardcoding it.

## The `on` Field Is a Hint, Not a Boundary

The `on` globs tell the agent where to start looking and which files drive cache invalidation. But the agent can read any file in the repo. If a test's `on` points at `src/routes/**/*.py` and the agent needs to check a middleware file in `src/middleware/`, it will.

Narrow `on` patterns make tests faster (better starting hints) and caching more effective (fewer files to hash). But don't overthink it -- the agent is good at exploring.

## Axiom Tests Are Cached Assertions

Think of each test as a cached assertion about your codebase at a point in time. When the relevant files change, the assertion is re-evaluated. When they don't, the cached result stands.

This means:
- **First runs are expensive.** The agent explores from scratch. Budget accordingly.
- **Subsequent runs are cheap.** Mostly cached. The cost scales with how much code changed, not how many tests you have.
- **Failed tests always re-run.** Even if the files haven't changed. The assumption is you've fixed something and want to verify.

## Next

- [Getting Started](./getting-started.md) -- install axiom and run your first test
- [Writing Tests](./writing-tests.md) -- YAML format, glob syntax, tags, and per-test overrides
- [Examples](./examples.md) -- real tests across Go, Python, TypeScript, Java, and Rust
