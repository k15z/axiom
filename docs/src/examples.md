# Examples

These examples demonstrate patterns you can adapt for your projects. The first section shows Go examples from axiom's own codebase. Cross-language examples for Python, TypeScript, Java, and Rust follow.

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

## Python Examples

### Django/Flask Authentication

```yaml
test_views_require_authentication:
  tags: [security]
  on:
    - src/views/**/*.py
    - src/routes/**/*.py
  condition: >
    All view functions or route handlers that access user data must
    require authentication. They should use @login_required,
    @auth_required, or equivalent decorators, or check
    request.user.is_authenticated before proceeding. Public endpoints
    (health checks, login, registration) are exempt.
```

### SQL Injection Prevention

```yaml
test_no_raw_sql_interpolation:
  tags: [security]
  on:
    - src/**/*.py
  condition: >
    All database queries must use parameterized statements or the ORM
    query builder. No raw string interpolation (f-strings, .format(),
    or % formatting) should be used to construct SQL queries. Look for
    patterns like f"SELECT ... {variable}", "SELECT ...".format(), or
    "SELECT ... %s" % variable in calls to cursor.execute(), raw(),
    or similar database functions.
```

### Async Resource Cleanup

```yaml
test_async_resources_cleaned_up:
  tags: [reliability]
  on:
    - src/**/*.py
  condition: >
    All async context managers (aiohttp sessions, database connections,
    file handles) must be used with "async with" to ensure cleanup on
    exit. There should be no bare .close() calls without a try/finally
    or context manager wrapping them.
```

## JavaScript / TypeScript Examples

### Express Middleware Ordering

```yaml
test_auth_middleware_before_routes:
  tags: [security]
  on:
    - src/app.ts
    - src/middleware/**/*.ts
    - src/routes/**/*.ts
  condition: >
    Authentication middleware must be registered before route handlers
    in the Express app setup. The auth middleware should run on all
    routes except explicitly whitelisted public paths (health check,
    login, public API docs). No route handler should access req.user
    without auth middleware running first.
```

### No Secrets in Client Bundles

```yaml
test_no_secrets_in_client_code:
  tags: [security]
  on:
    - src/client/**/*.ts
    - src/client/**/*.tsx
    - src/frontend/**/*.ts
  condition: >
    Client-side code must not contain API keys, secrets, or private
    configuration values. Check for hardcoded strings that look like
    API keys, any imports from server-only config modules, and direct
    references to process.env (which should only appear in server code
    or build-time config).
```

### React Hook Dependencies

```yaml
test_useeffect_has_dependencies:
  tags: [code-quality]
  on:
    - src/components/**/*.tsx
    - src/hooks/**/*.ts
  condition: >
    All useEffect calls must include a dependency array (the second
    argument). An empty array [] is acceptable for mount-only effects.
    Missing the dependency array entirely causes the effect to run on
    every render, which is almost always a bug.
```

## Java Examples

### Spring Controller Authorization

```yaml
test_controllers_have_authorization:
  tags: [security]
  on:
    - src/main/java/**/controller/**/*.java
  condition: >
    All REST controller methods that modify data (POST, PUT, DELETE,
    PATCH) must have authorization annotations (@PreAuthorize,
    @Secured, or @RolesAllowed). GET endpoints that return sensitive
    data should also be protected. Only health check and public info
    endpoints are exempt.
```

### No Field Injection

```yaml
test_no_field_injection:
  tags: [architecture]
  on:
    - src/main/java/**/*.java
  condition: >
    Spring beans must use constructor injection, not field injection.
    There should be no @Autowired annotations on fields. Dependencies
    should be declared as final constructor parameters, with
    @RequiredArgsConstructor or an explicit constructor. This ensures
    immutability and makes dependencies explicit for testing.
```

### Exception Handling Consistency

```yaml
test_controllers_use_exception_handler:
  tags: [error-handling]
  on:
    - src/main/java/**/controller/**/*.java
    - src/main/java/**/exception/**/*.java
  condition: >
    Controllers should not catch exceptions and manually build error
    responses. Instead, exceptions should propagate to a
    @ControllerAdvice / @ExceptionHandler that maps them to
    consistent error response DTOs. This ensures uniform error format
    across all endpoints.
```

## Rust Examples

### Unsafe Code Boundaries

```yaml
test_unsafe_is_contained:
  tags: [security]
  on:
    - src/**/*.rs
  condition: >
    Unsafe blocks must be confined to dedicated modules or functions
    with "unsafe" or "unchecked" in their name. No unsafe code should
    appear in business logic, request handlers, or high-level
    orchestration code. Each unsafe block must have a SAFETY comment
    explaining why it is sound.
```

### Error Handling with Result Types

```yaml
test_no_unwrap_in_production_code:
  tags: [reliability]
  on:
    - src/**/*.rs
  condition: >
    Production code (everything outside tests/ and examples/) must not
    use .unwrap() or .expect() on Result or Option types, except in
    cases where the value is statically guaranteed to be present (e.g.,
    compile-time constants, regex patterns). All fallible operations
    should use the ? operator or explicit match/if-let handling.
```

### Send + Sync Safety

```yaml
test_shared_state_is_sync:
  tags: [concurrency]
  on:
    - src/**/*.rs
  condition: >
    All types stored in shared application state (e.g., passed to
    actix-web's App::app_data or axum's Extension) must implement
    Send + Sync. Shared mutable state must use Arc<Mutex<T>>,
    Arc<RwLock<T>>, or an atomic type. There should be no raw
    RefCell or Rc in shared state.
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
