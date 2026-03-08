# Known Bugs

Discovered 2026-03-08 via CLI bug bashing session. Grouped by severity.

## High

### BUG-1: Path formatting — missing separator between directory and filename

**Files:** `internal/cli/list.go:59`, `internal/cli/validate.go:76`, `internal/output/output.go:37,358`

**Repro:**
```bash
axiom list -d /tmp/test-dir/.axiom
# Output shows: /tmp/test-dir/.axiomarchitecture.yml
# Expected:     /tmp/test-dir/.axiom/architecture.yml
```

**Cause:** `fmt.Printf("\n  %s%s\n", cfg.TestDir, currentFile)` concatenates the test directory and relative filename without a separator. Works with the default config (`.axiom/` has a trailing slash) but breaks when `-d` provides a path without a trailing slash.

**Fix:** Use `filepath.Join(cfg.TestDir, currentFile)` or normalize `TestDir` to always have a trailing slash.

---

## Medium

### BUG-2: `axiom cache clear` errors when cache doesn't exist

**Repro:**
```bash
axiom cache clear  # without having run tests first
# Error: clearing cache: remove .axiom/.cache/results.json: no such file or directory
```

**Expected:** Clearing a non-existent cache should be a silent no-op (idempotent), similar to `rm -f` vs `rm`.

**File:** `internal/cli/cache.go:36`

---

### BUG-3: `axiom doctor` hints "run `axiom doctor`" when doctor itself fails

**Repro:**
```bash
axiom doctor  # without API key
# ...
# Error: doctor found 1 issue(s)
# Hint: run `axiom doctor` to diagnose issues.
```

**Cause:** `root.go:34-35` unconditionally appends the hint for all non-SetupError errors. The `doctor` command returns a plain `fmt.Errorf` (not `SetupError`) on line 155, so it gets the generic hint — which is circular.

**File:** `internal/cli/root.go:34-35`, `internal/cli/doctor.go:155`

---

### BUG-4: Dry-run header says `axiom --dry-run` instead of `axiom run --dry-run`

**Repro:**
```bash
axiom run --dry-run
# Output:  axiom --dry-run
# Expected: axiom run --dry-run
```

`--dry-run` is a flag on the `run` subcommand, not a root flag. The header misrepresents the CLI invocation.

**File:** `internal/output/output.go:342`

---

### BUG-5: Running a nonexistent test name produces no error or feedback

**Repro:**
```bash
axiom run nonexistent_test --dry-run
# Output: just the header "axiom --dry-run" and nothing else
```

**Expected:** Should print "no test matching 'nonexistent_test' found" or similar.

**File:** `internal/cli/run.go:105-119` — filtered results can be empty with no feedback to the user.

---

### BUG-6: `--filter` / `--tag` with no matches produces no feedback

**Repro:**
```bash
axiom run --dry-run -f "zzz*"
axiom run --dry-run -t "nonexistent"
# Output: just the header, no summary, no "0 tests matched" message
```

**Same root cause as BUG-5.** When filtering produces zero results, the dry-run output silently prints an empty list. The actual `run` path (non-dry-run) likely has the same issue.

---

## Low

### BUG-7: `--version` flag doesn't work (only `version` subcommand)

**Repro:**
```bash
axiom --version
# Error: unknown flag: --version
```

Standard CLI convention (following `git --version`, `docker --version`, `go version`, etc.) is to support both `--version` and a `version` subcommand.

---

### BUG-8: Negative values accepted for `--concurrency` and `--retries`

**Repro:**
```bash
axiom run --dry-run -c -1       # silently accepted
axiom run --dry-run --retries -1  # silently accepted
```

**Expected:** Validate that these values are non-negative. Negative concurrency falls through to auto-detection (benign) but negative retries could cause unexpected behavior.

**File:** `internal/cli/run.go` — no validation on these flag values.

---

### BUG-9: `--json` and `--format` flags can conflict silently

**Repro:**
```bash
axiom run --json --format github
# --json is silently ignored because --format is not "text"
```

**Cause:** `run.go:144`: `if jsonOut && outputFormat == "text"` — the `--json` shorthand only applies when `--format` is the default value. Passing both flags should produce an error or at least a warning.

---

### BUG-10: Grammar — "1 tests" instead of "1 test" in cost estimate

**Repro:**
```bash
axiom run --dry-run test_agent_is_leaf_dependency
# worst-case cost estimate: ~$0.0550 (35.0K tokens/test × 1 tests)
```

Should be "1 test" (singular).

**File:** `internal/output/output.go` — the cost estimate format string.

---

### BUG-11: Empty test names and names with special characters accepted without validation

**Repro:** Create a YAML test file with:
```yaml
"":
  condition: "empty name"
"test/with/slashes":
  condition: "slashes in name"
```

Both pass discovery and validation without error.

**Risk:** Empty names display as blank lines in output. Slashes in names could break cache key paths or show output. Spaces are less risky but unusual.

**File:** `internal/discovery/discovery.go` — no name validation beyond duplicate checking.

---

### BUG-12: Absolute paths and parent traversal in `on:` globs not warned about

**Repro:**
```yaml
test_dangerous_glob:
  on: ["/etc/passwd"]
  condition: "should warn"
```
```bash
axiom validate
# test_dangerous_glob [ok]  — no warning
```

**Expected:** Validate should warn about `on:` patterns using absolute paths or `../` traversal. These patterns will never match project files and indicate a user mistake (or an attempt to read outside the project).

**File:** `internal/cli/validate.go` — glob validation only checks syntax, not semantics.

---

### BUG-13: `--quiet` flag has no effect in `--dry-run` mode

**Repro:**
```bash
axiom run --quiet --dry-run
# Output identical to: axiom run --dry-run
```

**File:** `internal/cli/run.go:102-127` — the dry-run code path returns early before the `quiet` flag is checked.

---

### BUG-14: `doctor` loads config twice

**File:** `internal/cli/doctor.go:50` and `internal/cli/doctor.go:62`

Config is loaded once inside the `yamlErr == nil` block (line 50 for display), then unconditionally again on line 62 (for remaining checks, ignoring errors). If `axiom.yml` exists but is malformed, the first load catches the error but the second load silently swallows it and uses defaults — so the rest of doctor runs against wrong config.

---

### BUG-15: `run.go` uses `os.Exit()` directly instead of returning errors

**File:** `internal/cli/run.go:182,184,188`

```go
if output.HasFailures(results) {
    os.Exit(1)
}
```

Calling `os.Exit()` inside a cobra `RunE` bypasses deferred cleanup functions and cobra's own error/exit handling. Should return a typed error and let the root command handle exit codes.
