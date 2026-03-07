# Axiom vs Other Tools

Axiom occupies a unique space -- it verifies behavioral properties of code using an LLM agent. Here's how it compares to tools you might already use.

## Axiom vs Linters (ESLint, golangci-lint, Pylint)

**Linters** check for syntactic patterns, style violations, and known anti-patterns using static rules.

**Axiom** verifies semantic properties that span multiple files and require understanding code behavior.

| | Linters | Axiom |
|---|---|---|
| **What it checks** | Syntax, style, known patterns | Behavioral invariants, architectural intent |
| **Scope** | Single file at a time | Cross-file, cross-package analysis |
| **Configuration** | Rules with parameters | Plain English conditions |
| **Example** | "Don't use `eval()`" | "No user input reaches `eval()` after passing through the sanitizer" |

**Use both.** Linters catch surface-level issues cheaply. Axiom catches the deeper properties that linters can't express.

## Axiom vs ArchUnit / ArchUnitNET

**ArchUnit** enforces architectural rules in Java/C# using code-based test definitions (e.g., "classes in package X should not depend on package Y").

**Axiom** does similar work but uses natural language instead of a language-specific API, and works across any language.

| | ArchUnit | Axiom |
|---|---|---|
| **Languages** | Java, C# | Any language |
| **Test format** | Java/C# code | YAML with plain English |
| **What it checks** | Package dependencies, naming, annotations | Any behavioral property describable in English |
| **Setup** | Add library dependency, write tests in code | Install binary, write YAML |
| **Determinism** | Fully deterministic | LLM-based (cached for consistency) |

**Choose ArchUnit** if you're in a Java/C# shop and want deterministic, zero-cost architectural checks.

**Choose Axiom** if you work across languages, want to express nuanced properties in English, or need to check things beyond import graphs (security invariants, concurrency safety, error handling patterns).

## Axiom vs Semgrep

**Semgrep** matches code patterns using structural rules (like regex for ASTs). It excels at finding specific code patterns across a codebase.

**Axiom** reasons about code behavior and intent, not just pattern matching.

| | Semgrep | Axiom |
|---|---|---|
| **How it works** | AST pattern matching | LLM agent explores code |
| **Rules** | Pattern syntax (language-aware) | Plain English |
| **Strengths** | Finding specific code patterns fast | Verifying behavioral properties that require understanding context |
| **Cost** | Free (open source) | API costs per run (cached) |
| **Example** | "Find `db.query(f'...')` calls" | "All database queries use parameterized statements -- no raw string interpolation should construct SQL" |

**Use Semgrep** for known vulnerability patterns, taint tracking, and enforcing specific code patterns at scale.

**Use Axiom** for higher-level properties that require understanding code flow, architecture, and intent -- things that can't be expressed as a single pattern.

## Axiom vs Manual Code Review

**Code review** is the gold standard for catching architectural violations, security issues, and design problems. But it's slow, inconsistent, and doesn't scale.

**Axiom** automates the kinds of checks that senior engineers do in code review.

| | Code Review | Axiom |
|---|---|---|
| **Speed** | Hours to days | Seconds to minutes |
| **Consistency** | Varies by reviewer | Same check every time (cached) |
| **Coverage** | Reviewers focus on the diff | Checks the full codebase |
| **Cost** | Engineer time | API costs |
| **Catches** | Everything a human notices | Properties you've defined |

**Axiom doesn't replace code review.** It automates the repeatable parts -- "did someone bypass auth?", "are all errors wrapped?", "does the agent package stay isolated?" -- so reviewers can focus on design, clarity, and edge cases.

## Axiom vs Unit Tests

**Unit tests** verify that specific functions produce expected outputs for given inputs.

**Axiom tests** verify that structural and behavioral properties hold across the codebase.

| | Unit Tests | Axiom |
|---|---|---|
| **What it tests** | Function behavior | Codebase properties |
| **Scope** | Single function/module | Cross-cutting concerns |
| **Example** | "`hashPassword('abc')` returns a 64-char hex string" | "All passwords are hashed before storage -- no plaintext passwords should reach the database layer" |
| **When it runs** | Every commit (fast) | Every commit or PR (slower, cached) |

**Use both.** Unit tests verify implementation correctness. Axiom tests verify that architectural intent is maintained as the codebase evolves.

## Summary

| Tool | Best For | Limitation |
|---|---|---|
| **Linters** | Style, syntax, known patterns | Can't reason about behavior |
| **ArchUnit** | Java/C# architecture rules | Language-specific, import-graph only |
| **Semgrep** | Code pattern matching | Can't reason about intent or context |
| **Code Review** | Everything | Slow, inconsistent, doesn't scale |
| **Unit Tests** | Function-level correctness | Can't check cross-cutting properties |
| **Axiom** | Behavioral invariants, architecture, security | API cost, LLM non-determinism |

The best setup uses axiom alongside your existing tools -- linters for cheap surface checks, unit tests for correctness, semgrep for known patterns, and axiom for the behavioral properties that only a human (or LLM) can verify.
