# Axiom vs Other Tools

Axiom isn't trying to replace your linter or your test suite. It fills a gap: verifying behavioral properties that span multiple files and require understanding intent, not just matching patterns.

Here's how it fits alongside tools you probably already use.

## vs Linters (ESLint, golangci-lint, Pylint)

Linters match syntactic patterns within single files. They're fast, cheap, and catch a lot. But they can't reason about behavior across files.

A linter can flag `eval()`. Axiom can verify that no user input reaches `eval()` after passing through your sanitizer pipeline -- a property that requires tracing data flow across modules.

**Use both.** Linters for surface-level hygiene, axiom for the cross-cutting properties linters can't express.

## vs ArchUnit / ArchUnitNET

ArchUnit is the closest tool to axiom for architectural testing. It enforces rules like "classes in package X should not depend on package Y" using Java/C# test code. It's deterministic, free, and deeply integrated with the JVM ecosystem.

The tradeoffs are real:

- ArchUnit is language-specific (Java, C#). Axiom works on any codebase.
- ArchUnit rules are code. Axiom tests are plain English. This changes who can write and review them.
- ArchUnit checks import graphs and annotations. Axiom can check anything the agent can reason about -- security properties, error handling patterns, concurrency safety.
- ArchUnit is deterministic. Axiom uses an LLM, which means results are cached for consistency but not guaranteed identical across model versions.

If you're in a Java shop and your constraints are purely structural (package dependencies, naming conventions, annotation usage), ArchUnit is the better tool. If you need to verify behavioral properties -- "all database connections are closed in error paths," "no API route bypasses rate limiting" -- or you work across languages, axiom is the better fit.

## vs Semgrep

Semgrep does structural pattern matching on ASTs. It's like grep but language-aware -- it understands syntax trees, not just text. It's excellent for finding known vulnerability patterns and enforcing code conventions at scale.

Axiom reasons about code behavior and intent. "Find all `db.query(f'...')` calls" is a Semgrep rule. "All database queries use parameterized statements -- no raw string interpolation should construct SQL" is an axiom test. The axiom version catches f-strings, `.format()`, `%` formatting, and any other interpolation pattern the agent encounters, without you enumerating them.

Semgrep is free, fast, and deterministic. Axiom costs money per run (cached after passing). Use Semgrep for known patterns you can express structurally. Use axiom for higher-level properties that require understanding context and intent.

## vs Code Review

The honest comparison: axiom automates the repeatable parts of what a senior engineer does in code review. "Did someone bypass auth?" "Are errors wrapped with context?" "Does the agent package stay isolated from the CLI layer?"

Code review catches everything a human notices, but it's slow, inconsistent across reviewers, and doesn't scale. Axiom checks the same properties on every commit, the same way, every time.

Axiom doesn't replace code review. It frees reviewers to focus on design, clarity, and edge cases instead of re-checking known invariants.

## vs Unit Tests

Unit tests verify that `hashPassword('abc')` returns a 64-char hex string. Axiom tests verify that all passwords are hashed before storage -- that no plaintext password reaches the database layer anywhere in the codebase.

Different scope, different purpose. Unit tests cover function-level correctness. Axiom covers cross-cutting properties that span files and packages. Use both.

## When to Use What

| Tool | Best for | Doesn't do |
|---|---|---|
| **Linters** | Style, syntax, known anti-patterns | Cross-file reasoning |
| **ArchUnit** | Java/C# import graphs, annotations | Behavioral properties, non-JVM languages |
| **Semgrep** | Structural pattern matching at scale | Reasoning about intent or context |
| **Code review** | Everything, eventually | Consistency, speed, scale |
| **Unit tests** | Function-level correctness | Cross-cutting codebase properties |
| **Axiom** | Behavioral invariants, architecture, security | Deterministic guarantees, free |

The best setup: linters for cheap hygiene, unit tests for correctness, semgrep for known patterns, and axiom for the behavioral properties that only a human -- or an LLM -- can verify.
