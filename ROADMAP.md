# Roadmap

Planned improvements for axiom, roughly in priority order.

## Developer Experience

- **Watch mode** (`axiom run --watch`) — re-run affected tests when source files change, using `on` globs as file watchers
- **`axiom run <test-name>`** — run a single test by name without needing `--filter`
- **Smarter `axiom init`** — detect project language and generate a relevant example test instead of a Go-specific one
- **Token/cost reporting** — show API usage summary after runs (e.g. "4 tests, 12 API calls, ~$0.03") to help users budget

## CI / Adoption

- **Reusable GitHub Action** — publish `uses: k15z/axiom-action@v1` so users don't have to write CI boilerplate
- **Go unit tests for core packages** — table-driven tests for `safePath`, `glob.Match`, `cache.ShouldSkip`, discovery ordering

## Robustness

- **Replace system `grep`** — use a Go-native grep implementation to avoid dependency on the `grep` binary (Windows, minimal containers)
- **Streaming API calls** — use streaming for long agent runs to show real-time progress and avoid timeouts

## Future Ideas

- **Custom tool plugins** — let users define additional tools the agent can use (e.g. run a linter, query a database schema)
- **Multi-provider support** — support OpenAI, Gemini, or local models as alternatives to Anthropic
- **Test dependencies** — allow tests to declare dependencies on other tests
- **Snapshot testing** — save and diff agent reasoning across runs to detect regressions in test behavior
