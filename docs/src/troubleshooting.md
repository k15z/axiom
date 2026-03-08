# Troubleshooting

## Common Issues

### "API key is not set"

Axiom requires an API key for your configured provider. The key name depends on which provider you're using:

**For Anthropic (default):**
```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**For OpenAI:**
```bash
export OPENAI_API_KEY=sk-...
```

**For Gemini:**
```bash
export GEMINI_API_KEY=AIza...
```

**For OpenAI-compatible (Ollama, vLLM):**
```bash
export OPENAI_API_KEY=not-needed  # or any non-empty value
```

You can also set these in a `.env` file:

```bash
echo "ANTHROPIC_API_KEY=sk-ant-..." > .env
```

If using `.env`, make sure it's in the project root (where you run `axiom`).

### "Provider is not set and cannot be inferred from model"

Make sure your model name includes a provider prefix:
- Anthropic: `claude-opus-4-6`, `claude-sonnet-4-6`, etc.
- OpenAI: `gpt-4o`, `gpt-4-turbo`, etc.
- Gemini: `gemini-2.0-flash`, `gemini-1.5-pro`, etc.

Or set the provider explicitly in `axiom.yml`:
```yaml
provider: openai
model: gpt-4o
```

See [Configuration](./configuration.md#multi-provider-support) for provider setup details.

### "test directory not found"

Run `axiom init` first, or specify the test directory:

```bash
axiom run --dir path/to/tests/
```

### Tests Always Re-run (Never Cached)

Check if your tests have `on` patterns:

```bash
axiom validate
```

Tests without `on` globs can never be cached. Add relevant file patterns:

```yaml
test_example:
  on:
    - src/**/*.go    # Cache based on these files
  condition: >
    ...
```

### "Agent exceeded maximum iterations"

The agent used all its turns without reaching a verdict. This usually means:

- The condition is too vague (agent doesn't know what to look for)
- The codebase is very large and the agent is exploring too broadly
- The iteration limit is too low for the complexity

**Fixes:**
- Make the condition more specific (name files, patterns, functions)
- Add narrower `on` globs to give the agent better starting hints
- Increase `max_iterations` in `axiom.yml` or per-test

### Rate Limit Errors (429)

Axiom automatically retries with exponential backoff (5s, 15s, 30s, 60s). If you're running many tests in parallel, reduce concurrency:

```bash
axiom run -c 2
```

### Tests Appear to Hang (OpenAI / Gemini)

Anthropic streams results as the agent works -- you see live progress. OpenAI and Gemini do not stream; results appear after the agent finishes each response. The test is still running. See [Streaming Behavior](./configuration.md#streaming-behavior) for details.

### Tests Are Slow

- **Use caching:** Don't pass `--all` unless you need it
- **Increase concurrency:** `axiom run -c 5`
- **Use Haiku** for simple tests, Sonnet for complex ones
- **Narrow your `on` globs** to avoid unnecessary re-runs
- **Set timeouts** to bound slow tests: `timeout: 60` in test YAML

## Debugging

### See Full Agent Reasoning

```bash
axiom run --verbose
```

This shows the complete reasoning for every test, not just the summary.

### Review Cached Reasoning

```bash
# See all cached reasoning
axiom show

# See reasoning for a specific test
axiom show test_auth_middleware

# Output as JSON for scripting
axiom show --json
```

### Preview Without Running

```bash
axiom run --dry-run
```

Shows which tests would run, which are cached, and estimated token cost.

### Validate Test Files

```bash
axiom validate
```

Checks for:
- Invalid glob syntax
- Missing `on` patterns (never cached)
- Very short conditions (likely too vague)

### Check Exit Codes

```bash
axiom run; echo "Exit code: $?"
```

| Code | Meaning |
|------|---------|
| `0` | All passed |
| `1` | Test failures |
| `2` | Setup/config error |

## Getting Help

- File issues at [github.com/k15z/axiom/issues](https://github.com/k15z/axiom/issues)
- Check the [CLI Reference](./cli-reference.md) for all available flags
