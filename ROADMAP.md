# Roadmap

Planned improvements for axiom, roughly in priority order.

## Agent Tools

- **`read_file` line ranges** — add optional `start_line`/`end_line` params so the agent can read a slice of a large file instead of the whole thing. Biggest single efficiency win for real codebases.
- **Fix `grep` glob filter to match paths** — currently matches only the filename (`d.Name()`), so `internal/**/*.go` doesn't work. Should match the relative path so the agent can scope searches to subdirectories.
- **`tree` tool** — recursive directory listing with a depth limit. Gives the agent a structural overview in one call instead of chaining `list_dir`.

## Developer Experience

- **`axiom run <test-name>`** — run a single test by name as a positional arg, without needing `--filter`
- **Watch mode** (`axiom run --watch`) — re-run affected tests when source files change, using `on` globs as file watchers
- **`axiom add <intent>`** — interactively generate a single test from a natural-language description (e.g. `axiom add "all API routes require auth"`)
- **`axiom explain <test-name>`** — show the cached reasoning from the last run without re-running the test

## Robustness

- **Streaming API calls** — use streaming for long agent runs to show real-time reasoning, not just tool-call summaries
- **Go unit tests for core packages** — table-driven tests for `safePath`, `glob.Match`, `cache.ShouldSkip`, discovery ordering, verdict parsing

## CI / Adoption

- **Reusable GitHub Action** — publish `uses: k15z/axiom-action@v1` so users don't have to write CI boilerplate

## Future Ideas

- **Custom tool plugins** — let users define additional tools the agent can use (e.g. run a linter, query a database schema)
- **Multi-provider support** — support OpenAI, Gemini, or local models as alternatives to Anthropic
- **Test dependencies** — allow tests to declare dependencies on other tests
- **Snapshot testing** — save and diff agent reasoning across runs to detect regressions in test behavior
