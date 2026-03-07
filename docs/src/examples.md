# Examples

These examples are real tests from axiom's own codebase (in `.axiom/`). They demonstrate patterns you can adapt for your projects.

## Architecture Tests

Verify structural constraints that prevent dependency rot.

### Layer Boundaries

```yaml
test_clean_package_dependencies:
  tags: [architecture]
  on:
    - internal/**/*.go
  condition: >
    The internal packages should have clean dependency flow: cli depends on
    config, discovery, runner, and output. Runner depends on agent, cache,
    config, and discovery. Agent should NOT import from cli, runner, output,
    or discovery. Cache should NOT import from agent, runner, cli, or output.
    No circular dependencies should exist.
```

**Why this works:** The condition names specific packages and forbidden import directions. The agent can verify this by checking `import` statements.

### CLI Isolation

```yaml
test_cli_does_not_import_agent:
  tags: [architecture]
  on:
    - internal/cli/*.go
    - internal/agent/*.go
  condition: >
    CLI command files in internal/cli/ must not directly import the
    internal/agent package. The CLI layer should interact with the agent
    only through the runner package, which handles orchestration. Check
    the import statements of all files in internal/cli/.
```

**Why this works:** It specifies a clear boundary (CLI must not import agent) and tells the agent exactly how to check it (look at import statements).

## Security Tests

Verify that security-critical code handles edge cases correctly.

### Path Traversal Prevention

```yaml
test_path_traversal_prevention:
  tags: [security]
  on:
    - internal/agent/tools.go
  condition: >
    All agent tools that accept file paths (read_file, list_dir, glob, grep)
    must validate that resolved paths do not escape the repository root
    directory. There should be a shared path validation function that converts
    relative paths to absolute, then checks they are prefixed by the repo root.
    Paths like "../../../etc/passwd" must be rejected.
```

### No Arbitrary Command Execution

```yaml
test_no_arbitrary_command_execution:
  tags: [security]
  on:
    - internal/agent/*.go
  condition: >
    The agent must not have any tool that allows arbitrary shell command
    execution. The only tool that runs a subprocess should be grep, and it
    must construct the command itself from validated parameters -- it should
    never pass user/LLM-provided strings directly as the command to exec.
    No tool should use sh -c or similar shell invocation.
```

### API Key Safety

```yaml
test_api_key_not_leaked:
  tags: [security]
  on:
    - internal/**/*.go
  condition: >
    The Anthropic API key must never be logged, printed to stdout/stderr,
    or included in error messages. Search for any fmt.Print, log.Print,
    or fmt.Errorf calls that reference the API key variable. The key should
    only be passed to the Anthropic client constructor.
```

## Error Handling Tests

Verify that errors are wrapped with context and produce correct exit codes.

### Error Wrapping

```yaml
test_errors_have_context:
  tags: [error-handling]
  on:
    - internal/**/*.go
  condition: >
    All errors returned from exported functions should be wrapped with
    context using fmt.Errorf("description: %w", err) rather than returned
    bare. This ensures error messages include enough context to diagnose
    the source of the problem. Check that functions in the runner, cache,
    discovery, and config packages wrap errors before returning them.
```

### Exit Code Semantics

```yaml
test_setup_errors_exit_code_2:
  tags: [error-handling]
  on:
    - internal/cli/root.go
    - internal/cli/run.go
  condition: >
    Setup errors (missing API key, bad YAML, test directory not found) must
    produce exit code 2, not exit code 1. The CLI should wrap these in a
    SetupError type, and the root command's error handler should check for
    SetupError and call os.Exit(2). Test failures should produce exit code 1.
```

## Code Quality Tests

Verify documentation and code organization standards.

### GoDoc Coverage

```yaml
test_exported_functions_documented:
  tags: [code-quality]
  on:
    - internal/**/*.go
  condition: >
    All exported functions and types in internal packages should have GoDoc
    comments. Check that each func or type starting with an uppercase letter
    has a comment immediately preceding it. Focus on the agent, runner, cache,
    and discovery packages. Test files (*_test.go) are excluded from this check.
```

### No Circular Dependencies

```yaml
test_no_circular_imports:
  tags: [code-quality, architecture]
  on:
    - internal/**/*.go
  condition: >
    No package under internal/ should have circular import dependencies.
    Each package's imports (found in the import blocks of its .go files)
    should form a DAG. For example, if package A imports package B, then
    package B must not import package A, either directly or transitively.
```

## Patterns for Good Tests

### Be Specific About What to Check

Instead of "error handling is good," specify:
- Which functions to check
- What pattern constitutes proper handling
- What the agent should look for (specific code patterns)

### Reference Concrete Code Patterns

```yaml
# Good: tells the agent what to grep for
condition: >
  Database connections must be closed using defer db.Close() or
  defer rows.Close() immediately after being opened.

# Vague: agent has to guess what "properly" means
condition: >
  Database connections should be properly managed.
```

### Scope Tests with `on` Globs

Narrow `on` patterns help the agent start in the right place and make caching effective:

```yaml
# Good: focused on the relevant package
on:
  - src/auth/**/*.py

# Too broad: changes anywhere invalidate the cache
on:
  - "**/*.py"
```

### Use Tags for Organization

Tags let you run related tests together:

```bash
# Run all security tests before a release
axiom run --tag security

# Run fast architecture checks on every PR
axiom run --tag architecture
```
