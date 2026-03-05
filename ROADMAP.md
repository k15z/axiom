# Roadmap

Planned improvements for axiom, roughly in priority order.

## Developer Experience

- **Watch mode** (`axiom run --watch`) — re-run affected tests when source files change, using `on` globs as file watchers

## Robustness

- **Go unit tests for core packages** — table-driven tests for `safePath`, `glob.Match`, `cache.ShouldSkip`, discovery ordering, verdict parsing (tree tool tests done; others pending)

## CI / Adoption

- **Reusable GitHub Action** — publish `uses: k15z/axiom-action@v1` so users don't have to write CI boilerplate

## Future Ideas

- **Custom tool plugins** — let users define additional tools the agent can use (e.g. run a linter, query a database schema)
- **Multi-provider support** — support OpenAI, Gemini, or local models as alternatives to Anthropic
- **Test dependencies** — allow tests to declare dependencies on other tests
- **Snapshot testing** — save and diff agent reasoning across runs to detect regressions in test behavior
