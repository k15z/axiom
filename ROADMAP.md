# Roadmap

Planned improvements for axiom, roughly in priority order.

## Done

Shipped features — kept here for reference.

- **Go unit tests for core packages** — table-driven tests for safePath, glob.Match, cache.ShouldSkip, discovery ordering, verdict parsing
- **Per-tool timeouts** — individual timeout for each agent tool call so one slow operation doesn't eat the entire test budget
- **Flaky test detection & retries** — `--retries N` re-runs failures; if a retry passes, marks the test as flaky
- **Cache invalidation on config change** — changing model, max_iterations, or max_tokens invalidates cached results
- **Duplicate test name detection** — warn or error during discovery if two tests share the same name
- **Watch mode** (`axiom run --watch`) — re-run affected tests when source files change via fsnotify
- **`axiom validate`** — lint test YAML for invalid globs, missing `on` patterns, vague conditions
- **Progress indicator** — "3/10 tests complete" during runs; periodic lines in CI/non-TTY
- **Dry-run mode** (`axiom run --dry-run`) — preview which tests would run, cached status, estimated token cost
- **Per-test config overrides** — `timeout`, `model`, and `max_iterations` in test YAML
- **Test tags/filtering** — `tags: [security, auth]` in test YAML, `axiom run --tag security`
- **Token budget hints** — system message injected when agent approaches token limit
- **Exit code semantics** — 0 = all pass, 1 = failures, 2 = config/setup error
- **PR comment summaries** — `axiom run --format github` outputs GitHub-flavored Markdown for PR comments
- **Reusable GitHub Action** — `uses: k15z/axiom@main` for one-step CI with cache and PR comments
- **Multi-provider support** — Anthropic, OpenAI, Gemini, and OpenAI-compatible endpoints via `provider` and `base_url`
- **Documentation site** — mdBook-based docs with GitHub Pages deployment
- **Enhanced examples** — 11 example tests across architecture, security, error handling, and code quality
- **Agent memory / codebase notes** — agent persists investigation notes across runs with file-hash staleness tracking
- **Agent reasoning diff** (`axiom show --diff`) — compare current cached reasoning against the previous run's reasoning with +/- line markers

## Reliability & Trust

- **Expand unit test coverage** — increase coverage for runner, config, and CLI packages

## Cost & Performance

- **Smarter concurrency defaults** — rate-limit-aware backoff at the runner level to avoid API quota spikes (basic auto-detection done; backoff pending)
- **CI cache persistence** — document caching `.axiom/.cache/` for non-GitHub CI systems (GitHub Action handles this automatically)

## Agent Quality

- **Verbose tool tracing** (`axiom run --trace`) — log every tool call, its arguments, output size, and duration to a file for post-mortem debugging

## Future Ideas

Larger features that expand what axiom can do.

- **Custom tool plugins** — let `axiom.yml` define additional tools the agent can use (e.g. run a linter, query a database schema, call an API endpoint)
- **Test dependencies** — allow tests to declare `depends_on: [other_test]` so they only run after prerequisites pass
- **Snapshot testing** — save and diff agent reasoning across runs to detect regressions in test behavior
- **Condition quality scoring** — use a fast model to rate test conditions on specificity, measurability, and relevance before running them
