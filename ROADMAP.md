# Roadmap

Planned improvements for axiom, roughly in priority order.

## Reliability & Trust

These make axiom results something you can depend on in CI.

- ~~**Go unit tests for core packages** — table-driven tests for `safePath`, `glob.Match`, `cache.ShouldSkip`, discovery ordering, verdict parsing (tree tool tests done; others pending)~~ *Shipped*
- ~~**Per-tool timeouts** — individual timeout for each agent tool call (grep on a huge repo, reading a massive file) so one slow operation doesn't eat the entire test budget~~ *Shipped in PR #12*
- ~~**Flaky test detection & retries** — if a test flips between pass/fail across runs with no file changes, flag it as flaky. Optional `--retries N` to re-run failures before reporting them~~ *Shipped in PR #20*
- ~~**Cache invalidation on config change** — changing model, max_iterations, or max_tokens should invalidate cached results since a different config may produce different verdicts~~ *Shipped in PR #15*
- ~~**Duplicate test name detection** — warn or error during discovery if two tests share the same name instead of silently dropping one~~ *Shipped in PR #17*

## Developer Experience

Day-to-day workflow improvements for test authors.

- **Watch mode** (`axiom run --watch`) — re-run affected tests when source files change, using `on` globs as file watchers *(PR #22 open)*
- ~~**`axiom validate`** — lint test YAML: check that glob patterns are valid, conditions aren't empty platitudes ("code is clean"), and warn about tests with no `on` globs (always re-run, never cached)~~ *Shipped in PR #16*
- ~~**Progress indicator** — show "3/10 tests complete" during runs instead of just a spinner. In non-TTY/CI mode, print periodic progress lines instead of silence~~ *Shipped in PRs #13, #18*
- ~~**Dry-run mode** (`axiom run --dry-run`) — show which tests would run, which are cached, and estimated token cost without actually calling the API~~ *Shipped in PR #14*
- ~~**Per-test config overrides** — allow `timeout`, `model`, and `max_iterations` in test YAML so expensive tests can use a more capable model or get more time~~ *Shipped in PR #19*
- ~~**Test tags/filtering** — add optional `tags: [security, auth]` to test YAML, run subsets with `axiom run --tag security`~~ *Shipped in PR #19*

## Cost & Performance

Make axiom viable for large test suites and daily CI.

- **Cost estimation before run** — show estimated token cost based on test count and average historical usage, with a `--budget` flag to abort if projected cost exceeds a threshold
- **Smarter concurrency defaults** — auto-detect a reasonable `-c` value (e.g. 3-5) instead of defaulting to 1, with rate-limit-aware backoff at the runner level to avoid API quota spikes
- **CI cache persistence** — document and support caching `.axiom/.cache/` as a CI artifact so repeated CI runs skip unchanged tests (works today, just needs guidance + GitHub Action support)

## CI / Adoption

Lower the barrier to running axiom in CI pipelines.

- **Reusable GitHub Action** — publish `uses: k15z/axiom-action@v1` that installs axiom, restores cache, runs tests, and posts a PR comment summary with pass/fail/cached counts *(PR #26 open)*
- **PR comment summaries** — `axiom run --format github` outputs a markdown summary suitable for posting as a PR comment (test table, cost, cache hit rate) *(PR #23 open)*
- ~~**Exit code semantics** — document and ensure clean exit codes: 0 = all pass, 1 = failures, 2 = config/setup error, so CI can distinguish "tests failed" from "axiom is broken"~~ *Shipped*

## Agent Quality

Make the agent smarter and more observable.

- ~~**Token budget hints** — when the agent is approaching its token limit, inject a system message like "You are running low on tokens. Please state your verdict now." instead of hard-cutting~~ *Shipped in PR #18*
- **Agent reasoning diff** — `axiom show --diff` compares current cached reasoning against the previous run's reasoning, highlighting what changed (useful for debugging flips)
- **Verbose tool tracing** (`axiom run --trace`) — log every tool call, its arguments, output size, and duration to a file for post-mortem debugging of stuck or slow tests

## Future Ideas

Larger features that expand what axiom can do.

- **Custom tool plugins** — let `axiom.yml` define additional tools the agent can use (e.g. run a linter, query a database schema, call an API endpoint)
- **Multi-provider support** — support OpenAI, Gemini, or local models as alternatives to Anthropic *(PR #24 open)*
- **Test dependencies** — allow tests to declare `depends_on: [other_test]` so they only run after prerequisites pass
- **Snapshot testing** — save and diff agent reasoning across runs to detect regressions in test behavior
- **Condition quality scoring** — use a fast model to rate test conditions on specificity, measurability, and relevance before running them
