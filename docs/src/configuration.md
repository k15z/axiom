# Configuration

## axiom.yml

Place `axiom.yml` in your project root. All fields are optional -- defaults are applied for anything omitted.

```yaml
# LLM model to use (default: claude-haiku-4-5-20251001)
model: claude-haiku-4-5-20251001

# LLM provider: anthropic (default), openai, or gemini
# If omitted, provider is inferred from the model name
# provider: openai

# Custom API endpoint for OpenAI-compatible providers (Ollama, vLLM, etc.)
# Ignored unless provider is openai
# base_url: http://localhost:11434/v1

# Directory containing test YAML files (default: .axiom/)
test_dir: .axiom/

# Cache settings
cache:
  enabled: true           # Set to false to disable caching entirely
  dir: .axiom/.cache/     # Where cache files are stored (includes notes.json for agent memory)

# Agent settings
agent:
  max_iterations: 30      # Max tool-use turns per test (default: 30)
  max_tokens: 10000       # Max tokens per LLM response (default: 10000)
  timeout: 0              # Per-test timeout in seconds (0 = no timeout)
  tool_timeout: 30        # Per-tool timeout in seconds (default: 30)
```

### Defaults

| Field | Default |
|-------|---------|
| `model` | `claude-haiku-4-5-20251001` |
| `provider` | inferred from model name, or `anthropic` |
| `test_dir` | `.axiom/` |
| `cache.enabled` | `true` |
| `cache.dir` | `.axiom/.cache/` |
| `agent.max_iterations` | `30` |
| `agent.max_tokens` | `10000` |
| `agent.timeout` | `0` (no timeout) |
| `agent.tool_timeout` | `30` |

### Model Selection

Choose a model based on your needs:

| Model | Speed | Cost | Best For |
|-------|-------|------|----------|
| `claude-haiku-4-5-20251001` | Fast | Low | Daily CI, simple conditions |
| `claude-sonnet-4-6` | Medium | Medium | Complex architectural analysis |
| `claude-opus-4-6` | Slow | High | Deep security audits |

You can override the model per-run with `--model`:

```bash
axiom run --model claude-sonnet-4-6
```

Or per-test in the YAML:

```yaml
test_complex_security_audit:
  model: claude-sonnet-4-6
  condition: >
    ...
```

## Multi-Provider Support

Axiom supports Anthropic, OpenAI, and Gemini. The provider is auto-detected from the model name, or set explicitly in `axiom.yml`:

### Anthropic (Default)

```yaml
model: claude-opus-4-6
# Provider is inferred from model name
```

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

### OpenAI

```yaml
provider: openai
model: gpt-4o
```

```bash
export OPENAI_API_KEY=sk-...
```

### Gemini

```yaml
provider: gemini
model: gemini-2.0-flash
```

```bash
export GEMINI_API_KEY=AIza...
```

### OpenAI-Compatible (Ollama, vLLM, etc.)

```yaml
provider: openai
model: llama3        # or any model name
base_url: http://localhost:11434/v1
```

```bash
export OPENAI_API_KEY=not-needed  # or set to any non-empty value
```

### Streaming Behavior

Anthropic streams results as the agent works -- you see live progress in the terminal (spinner updates, tool calls). OpenAI and Gemini do not stream; results appear after the agent finishes each response. This is a provider-level difference, not an axiom limitation. All three providers produce the same final output.

If you're used to Anthropic's live feedback and switch to OpenAI or Gemini, tests may appear to "hang" before showing results. They're not stuck -- the provider just doesn't support incremental output.

## API Key

Set the API key for your provider via environment variable or `.env` file:

```bash
export ANTHROPIC_API_KEY=sk-ant-...   # or OPENAI_API_KEY, GEMINI_API_KEY
```

Or create a `.env` file in your project root:

```
ANTHROPIC_API_KEY=sk-ant-...
```

Axiom loads `.env` automatically. Environment variables take precedence over `.env` values. Add `.env` to your `.gitignore`.

## Per-Test Overrides

Individual tests can override global settings:

```yaml
test_expensive_analysis:
  model: claude-sonnet-4-6     # Use a more capable model
  timeout: 120                 # 2-minute timeout
  max_iterations: 50           # Allow more exploration
  on:
    - src/**/*.py
  condition: >
    ...
```

Overrides apply only to that test. All other tests use the global `axiom.yml` settings.

## Cache

### How Caching Works

Axiom hashes the content of files matching each test's `on` globs. If the hashes match the last passing run, the test is skipped.

A test re-runs when:
- It failed last time (failed tests always re-run)
- Any file matching its `on` globs has changed
- The config hash changed (different model, max_iterations, or max_tokens)
- `--all` flag is used
- The test has no `on` globs (always runs)

### Cache Location

Cache state lives in `.axiom/.cache/results.json`. Add this to `.gitignore` for local development, but consider caching it as a CI artifact for faster CI runs.

### Clearing the Cache

```bash
axiom cache clear
```

### Config-Aware Invalidation

Changing `model`, `max_iterations`, or `max_tokens` in `axiom.yml` automatically invalidates cached results, since different settings may produce different verdicts.

## Agent Memory

Axiom's agent builds notes about your codebase across runs, stored in `.axiom/.cache/notes.json`. Notes are automatically invalidated when referenced files change. This reduces token usage and speeds up subsequent runs.

See [Agent Memory](./agent-memory.md) for details on how notes work, cost savings, and debugging.
