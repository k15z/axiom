# Roadmap

Revised 2026-03-08 by the full team (Alex, Sam, Priya, Jordan, Morgan).

This is a strategy document, not a feature backlog. Items are ordered by impact on adoption. If it's not on this list, we're not building it.

## Next Up

Ship these in the next 1-2 weeks. Any SWE can pick up any item. Items are roughly ordered -- earlier items unblock later ones.

### Distribution (unblocked -- ship now)

We have goreleaser and binary releases. What's missing is the last mile.

1. **Tag v0.1.2 + CHANGELOG** -- Tag the current release as v0.1.2. Write a changelog from the Done section. This unblocks everything below. Day 1.
2. **GitHub Action `@v0` ref** -- Point `@v0` at the tagged release so CI users can pin to a stable ref instead of `@main`. `@main` signals instability to CI adopters.
3. **Curl installer** -- `curl -fsSL https://axiom.dev/install.sh | sh` (or equivalent under current domain). Shell script that detects OS/arch and pulls the right binary from GitHub Releases. Day's work.
4. **Homebrew formula** -- `brew install axiom`. Goreleaser already has the tap config (`k15z/homebrew-tap`); once the release is tagged, the formula auto-publishes. Getting into homebrew-core comes later with traction.
5. **README badge** -- `[![tested with axiom](https://img.shields.io/badge/tested%20with-axiom-green)]` -- free viral distribution. Every repo that adds it becomes an ad for axiom. Ship badge snippet in README and docs on day one.

### Feedback Loop

Jordan's point: if failures don't explain themselves well, faster tests and easier install don't matter. Fix the feedback loop before chasing distribution scale.

6. **Token cost display always-on** -- Cost display exists behind `--costs` flag. Make it default-on with `--no-costs` opt-out. Show cost per test inline: `test_auth_middleware (8.4s, ~$0.02)`. Show run total in summary line. Include in `--quiet` summary too (CI users need budget visibility). The pricing lookup and `estimateCost` infrastructure is already in `internal/output/output.go`.
7. **Error messages as onboarding** -- Systematic pass on every user-facing error in `internal/cli/`. Every error follows the pattern: what happened + what to do + where to learn more. Example: "ANTHROPIC_API_KEY is not set. Set it in your environment or create a .env file. See: https://axiom.dev/docs/getting-started". Most users who churn do so at the error message, not the docs. Config/API key errors are already decent; the rest of `internal/cli/` needs an audit.
8. **Config shim cleanup** -- Remove deprecated `LoadWithoutKey` and `LoadMinimal` wrappers in `config.go` and migrate their callers (`init.go`, `cache.go`, `validate.go`, `run.go`) to `Load(LoadOpts{})`. The consolidation into `Load(LoadOpts)` is already done; this is the final cleanup of legacy callers.

### Docs (first-class, not afterthought)

Morgan's call: docs improvements ship alongside code, not after. README updates MUST ship simultaneously with distribution changes -- don't launch curl/Homebrew without updating the install section.

13. **Introduction page rewrite** -- Current intro opens with a description, not a hook. Replace the 10-bullet feature list with a before/after: show a property that breaks silently in a refactor, show the axiom test that catches it. A developer from HN needs to understand in 10 seconds: what does this do, why should I care, what does it look like.
14. **"Thinking in Axiom" mental model page** -- New page between Introduction and Getting Started. Covers: axiom tests are automated code review (not unit tests), they verify intent (not implementation), they're expensive but cached, they're best for cross-cutting properties that span files. This is why users write bad tests -- they don't have the right mental model. Absorb the best parts of the comparisons page here.
15. **Getting-started rewrite** -- Restructure: (a) 30-second install via curl, (b) `axiom add "your first test"` with immediate payoff, (c) explain what happened and how to write more. Include a GIF or asciinema recording. Cut jargon. Link prominently to the example tests in `.axiom/` -- they're genuinely well-written and the best teaching material we have.
16. **README install section update** -- Lead with curl installer, then Homebrew, then `go install` as fallback. Must ship same day as distribution items 1-4.
17. **Consolidate Agent Memory docs** -- Currently explained in three places (intro, Writing Tests, dedicated page) with different framing. Brief mention in Writing Tests, full explanation on dedicated page, remove from intro feature list.
18. **Streaming behavior docs** -- Anthropic shows live progress; OpenAI and Gemini show results after completion. Zero code change, just set expectations.

## Medium-Term

Target: 1-3 months. These expand axiom's reach but depend on the sprint above being done.

### Content Before Infrastructure

Before building registry infra, prove the test format works at scale by shipping content.

- **Bundled example test packs (20+)** -- Curate tests for common frameworks and concerns: Express/Fastify auth patterns, Django ORM safety, React component boundaries, SQL injection, OWASP top 10 basics, Go error handling idioms, Python type safety. Ship them in a `examples/` directory in the repo. These become the seed content for the registry and the best marketing material we have.
- **`axiom add` category menu** -- When run without arguments, show a picker: security, architecture, code quality, error handling, performance. Each category has 3-5 template tests. No framework detection magic -- just good defaults. Collect data on what users pick to inform future detection heuristics.

### Registry MVP

Only after bundled packs exist and we know the test format works.

- **`axiom install <pack>`** -- GitHub-backed index of test packs. Start with 10 curated packs pulled from the bundled examples. Simple: a pack is a GitHub repo with YAML files and a manifest. `axiom install` clones into `.axiom/vendor/` or similar. No versioning, no namespacing, no publish workflow yet. Validate the concept before building the platform.

### DX Polish

- **`axiom show` displays model used** -- Show which model produced cached reasoning. Matters when per-test model overrides are in play.
- **Shareable test results** -- `axiom run --format html` or `--format link` that produces a self-contained HTML report or uploads to a paste service. Makes it easy to share results in Slack, PRs, or tweets. Viral loop: people share results, others see axiom for the first time.

## Future Vision

Ideas, not commitments. We'll revisit these when the fundamentals above are solid.

### Milestones

- **v1.0.0** -- Ship when the product is stable, the install story is polished, docs are solid, and we have real users in production. v1 is a stability promise, not a feature milestone.

### Strategic Bets

These compound on each other but require significant investment. Order matters.

- **Hosted GitHub App** -- Zero-setup axiom: install the app, axiom runs on PRs automatically. No Go install, no API key management, no CI config. This is the product that turns axiom from a dev tool into a platform. Requires: hosted backend, GitHub App OAuth, webhook handling, multi-tenant runner, billing.
- **Learning backend** -- Aggregate anonymized agent traces (tool calls, reasoning, outcomes) across opt-in users. More data = smarter agent = cheaper runs = more users. Build after the GitHub App provides the user base.
- **Full test registry** -- Expand the MVP into a public index with namespace ownership, versioning, publisher accounts, and community governance. Build after the MVP proves demand.

### Speculative

These might matter later. Not building them now.

- **Custom tool plugins** -- Let `axiom.yml` define additional agent tools (run a linter, query a DB schema, call an API). Powerful but complex surface area.
- **Snapshot testing** -- Diff agent reasoning across runs to detect regressions in test behavior. Interesting for stability tracking.
- **Verbose tool tracing** (`--trace`) -- Log every tool call with args, output size, and duration. Useful for debugging slow tests.

### Cut

Removed from the roadmap. Not happening unless users specifically request them.

- Config inheritance (monorepo/enterprise, premature)
- OpenAI/Gemini streaming (high effort, low adoption impact)
- Expected failures / `expect_fail` (tests should always hold)
- Condition quality scoring (premature, need more users first)
- Test dependencies / `depends_on` (tests should be independent)

## Done

Shipped features -- kept here for reference.

- Ripgrep shelling for grep tool (`toolGrepRg` with `exec.Command`, explicit args, Go fallback)
- `axiom doctor` (config, API key + connectivity, test directory, rg detection, glob validation)
- Discovery line numbers in YAML errors (`keyNode.Line` in `discovery.go`)
- Config loading consolidation (`Load(LoadOpts)` single entry point, `LoadMinimal` error-swallowing fix)
- Failure output contrast fix (`printFailureReasoning` uses default text, not gray)
- Condition displayed alongside failure output (`printCondition`)
- Go unit tests for core packages
- Per-tool timeouts
- Flaky test detection & retries (`--retries N`)
- Cache invalidation on config change
- Duplicate test name detection
- Watch mode (`axiom run --watch`)
- `axiom validate`
- Progress indicator
- Dry-run mode (`axiom run --dry-run`)
- Per-test config overrides
- Test tags/filtering (`--tag`)
- Token budget hints
- Exit code semantics (0/1/2)
- PR comment summaries (`--format github`)
- Reusable GitHub Action
- Multi-provider support (Anthropic, OpenAI, Gemini)
- Documentation site (mdBook + GitHub Pages)
- Enhanced examples (11 tests)
- Agent memory / codebase notes
- Agent reasoning diff (`axiom show --diff`)
- Infrastructure error distinction
- Verdict parsing tightening
- `axiom version`
- `--strict` mode
- `axiom add` UX improvements
- `axiom init` multi-provider
- Shared retry logic with jitter
- Runner-level rate-limit backoff
- Expanded unit test coverage (67+ tests)
- SetupError wrapping
- CI improvements
- `--quiet` / `-q` flag
- `axiom list --json`
- `axiom cache info` + corruption warning
- `cache clear` respects config
- `init` and `add` respect `test_dir` config
- Cache tracks provider and base_url changes
- Context-aware tools (goroutine leak fix)
- Retry loop context cancellation fix
- stripGooglePrefix Gemini bug fix
- math/rand/v2 standardization
- Exit code fix for list/show commands
- Hardcoded `.axiom/` path fix in output
- Quiet mode flaky count
- Double space fix in dry-run header
- Binary releases via goreleaser
- `--dir` flag for cache commands
- Getting-started restructure
- README Quick Start update
- Docs cleanup
- `testExecutor` extraction
- Structured error types
- Reuse provider across tests
- Auto-validate before running
