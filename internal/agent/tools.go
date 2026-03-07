package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/k15z/axiom/internal/glob"
)

const maxOutputBytes = 100_000 // truncate tool output beyond this

// ToolDefs returns the tool definitions for the agent.
func ToolDefs() []anthropic.ToolUnionParam {
	return []anthropic.ToolUnionParam{
		{OfTool: &anthropic.ToolParam{
			Name:        "read_file",
			Description: anthropic.String("Read the contents of a file. Returns the file contents with line numbers. Use start_line/end_line to read a specific range instead of the whole file."),
			InputSchema: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative path to the file from the repository root",
					},
					"start_line": map[string]any{
						"type":        "integer",
						"description": "First line to read (1-based, inclusive). If omitted, reads from the beginning.",
					},
					"end_line": map[string]any{
						"type":        "integer",
						"description": "Last line to read (1-based, inclusive). If omitted, reads to the end.",
					},
				},
				"required": []string{"path"},
			}),
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "glob",
			Description: anthropic.String("Find files matching a glob pattern. Returns a list of matching file paths. Supports ** for recursive matching."),
			InputSchema: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Glob pattern (e.g. 'src/**/*.py', '*.go', 'internal/*/config.go')",
					},
				},
				"required": []string{"pattern"},
			}),
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "grep",
			Description: anthropic.String("Search file contents using a regex pattern. Returns matching lines with file paths and line numbers."),
			InputSchema: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Regex pattern to search for",
					},
					"glob": map[string]any{
						"type":        "string",
						"description": "Optional glob to filter which files to search (e.g. '*.py'). If omitted, searches all files.",
					},
				},
				"required": []string{"pattern"},
			}),
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "list_dir",
			Description: anthropic.String("List the contents of a directory. Returns file and directory names."),
			InputSchema: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative path to the directory from the repository root. Use '.' for the root.",
					},
				},
				"required": []string{"path"},
			}),
		}},
		{OfTool: &anthropic.ToolParam{
			Name:        "tree",
			Description: anthropic.String("Recursively list a directory tree with depth limit. Returns an indented tree of files and directories, useful for understanding project structure."),
			InputSchema: jsonSchema(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "Relative path to the directory from the repository root. Use '.' for the root.",
					},
					"depth": map[string]any{
						"type":        "integer",
						"description": "Maximum depth to recurse (default 3). 1 = immediate children only.",
					},
				},
				"required": []string{"path"},
			}),
		}},
	}
}

func jsonSchema(v any) anthropic.ToolInputSchemaParam {
	raw, _ := json.Marshal(v)
	var schema anthropic.ToolInputSchemaParam
	json.Unmarshal(raw, &schema)
	return schema
}

// ExecuteTool dispatches a tool call and returns the result string.
// If timeout > 0, the tool is cancelled after that duration (in addition to any
// deadline already on ctx). This prevents a slow grep on a large repo from
// consuming the entire test's time budget.
func ExecuteTool(ctx context.Context, name string, inputJSON json.RawMessage, repoRoot string, timeout time.Duration) (string, bool) {
	var input map[string]any
	if err := json.Unmarshal(inputJSON, &input); err != nil {
		return fmt.Sprintf("error parsing input: %s", err), true
	}

	toolCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	type toolResult struct {
		output  string
		isError bool
	}
	ch := make(chan toolResult, 1)

	go func() {
		var out string
		var isErr bool
		switch name {
		case "read_file":
			out, isErr = toolReadFile(getString(input, "path"), getInt(input, "start_line"), getInt(input, "end_line"), repoRoot)
		case "glob":
			out, isErr = toolGlob(getString(input, "pattern"), repoRoot)
		case "grep":
			out, isErr = toolGrep(getString(input, "pattern"), getString(input, "glob"), repoRoot)
		case "list_dir":
			out, isErr = toolListDir(getString(input, "path"), repoRoot)
		case "tree":
			out, isErr = toolTree(getString(input, "path"), getInt(input, "depth"), repoRoot)
		default:
			out, isErr = fmt.Sprintf("unknown tool: %s", name), true
		}
		ch <- toolResult{out, isErr}
	}()

	select {
	case r := <-ch:
		return r.output, r.isError
	case <-toolCtx.Done():
		// TODO: thread toolCtx into walk-heavy tools (toolGrep, toolTree) so
		// they can bail early on cancellation. For now the goroutine continues
		// until its I/O completes then writes to the buffered channel and is GC'd.
		return fmt.Sprintf("tool %q timed out", name), true
	}
}

func getString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func getInt(m map[string]any, key string) int {
	v, _ := m[key].(float64) // JSON numbers decode as float64
	return int(v)
}

func safePath(rel, root string) (string, error) {
	if rel == "" {
		return filepath.Abs(root)
	}
	abs := filepath.Join(root, rel)
	abs, err := filepath.Abs(abs)
	if err != nil {
		return "", err
	}
	rootAbs, _ := filepath.Abs(root)
	// Use separator suffix to avoid /foo matching /foobar
	if abs != rootAbs && !strings.HasPrefix(abs, rootAbs+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the repository root", rel)
	}
	return abs, nil
}

func toolReadFile(path string, startLine, endLine int, root string) (string, bool) {
	abs, err := safePath(path, root)
	if err != nil {
		return err.Error(), true
	}

	data, err := os.ReadFile(abs)
	if err != nil {
		return fmt.Sprintf("error reading file: %s", err), true
	}

	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)

	// Apply line range (1-based, inclusive)
	start := 1
	end := totalLines
	if startLine > 0 {
		start = startLine
	}
	if endLine > 0 {
		end = endLine
	}
	if start > totalLines {
		return fmt.Sprintf("start_line %d is beyond end of file (%d lines)", start, totalLines), true
	}
	if end > totalLines {
		end = totalLines
	}

	var b strings.Builder
	if start > 1 || end < totalLines {
		fmt.Fprintf(&b, "[lines %d-%d of %d]\n", start, end, totalLines)
	}
	for i := start - 1; i < end; i++ {
		fmt.Fprintf(&b, "%4d | %s\n", i+1, lines[i])
	}

	return truncate(b.String()), false
}

func toolGlob(pattern, root string) (string, bool) {
	// Validate the non-wildcard prefix using the shared safePath function.
	// This rejects absolute paths, .. traversal, and anything outside the repo root.
	prefix := pattern
	if idx := strings.IndexAny(pattern, "*?["); idx != -1 {
		prefix = filepath.Dir(pattern[:idx])
		if prefix == "." {
			prefix = ""
		}
	}
	if _, err := safePath(prefix, root); err != nil {
		return fmt.Sprintf("invalid pattern %q: %s", pattern, err), true
	}

	var matches []string

	// Use WalkDir for ** support
	rootAbs, _ := filepath.Abs(root)
	filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs (like .git)
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != rootAbs {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}

		if glob.Match(pattern, rel) {
			matches = append(matches, rel)
		}
		return nil
	})

	if len(matches) == 0 {
		return "no files matched", false
	}

	return truncate(strings.Join(matches, "\n")), false
}

func toolGrep(pattern, globFilter, root string) (string, bool) {
	// The glob filter is a filename-only pattern (e.g. "*.go"); reject path traversal
	if strings.Contains(globFilter, "..") || strings.HasPrefix(globFilter, "/") {
		return fmt.Sprintf("invalid glob filter %q: must be a filename pattern, not a path", globFilter), true
	}

	rootAbs, err := safePath("", root)
	if err != nil {
		return err.Error(), true
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Sprintf("invalid regex pattern: %s", err), true
	}

	var b strings.Builder
	filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Skip hidden dirs (like .git)
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") && path != rootAbs {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(rootAbs, path)
		if err != nil {
			return nil
		}

		// Apply glob filter against relative path (supports directory scoping like "internal/**/*.go")
		if globFilter != "" && !glob.Match(globFilter, rel) {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if re.MatchString(line) {
				fmt.Fprintf(&b, "%s:%d:%s\n", rel, lineNum, line)
				if b.Len() > maxOutputBytes {
					return fs.SkipAll
				}
			}
		}
		return nil
	})

	if b.Len() == 0 {
		return "no matches found", false
	}

	return truncate(b.String()), false
}

func toolListDir(path, root string) (string, bool) {
	abs, err := safePath(path, root)
	if err != nil {
		return err.Error(), true
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return fmt.Sprintf("error listing directory: %s", err), true
	}

	var b strings.Builder
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue // skip hidden files
		}
		suffix := ""
		if e.IsDir() {
			suffix = "/"
		}
		fmt.Fprintf(&b, "%s%s\n", e.Name(), suffix)
	}

	return truncate(b.String()), false
}

const (
	treeMaxEntriesPerDir = 50
	treeMaxEntriesTotal  = 1000
)

func toolTree(path string, depth int, root string) (string, bool) {
	abs, err := safePath(path, root)
	if err != nil {
		return err.Error(), true
	}

	if depth < 1 {
		depth = 3
	}

	var b strings.Builder
	totalEntries := 0
	limitReached := false

	var walk func(dir string, prefix string, currentDepth int)
	walk = func(dir string, prefix string, currentDepth int) {
		if limitReached || currentDepth > depth {
			return
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}

		// Filter hidden entries
		var visible []os.DirEntry
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				visible = append(visible, e)
			}
		}

		dirCount := len(visible)
		shown := 0
		for i, e := range visible {
			if limitReached {
				return
			}
			if shown >= treeMaxEntriesPerDir {
				fmt.Fprintf(&b, "%s... (%d more entries)\n", prefix, dirCount-shown)
				break
			}

			totalEntries++
			if totalEntries > treeMaxEntriesTotal {
				limitReached = true
				fmt.Fprintf(&b, "%s... (entry limit reached)\n", prefix)
				return
			}

			isLast := i == len(visible)-1
			connector := "├── "
			childPrefix := prefix + "│   "
			if isLast || (shown == treeMaxEntriesPerDir-1 && i < len(visible)-1) {
				connector = "└── "
				childPrefix = prefix + "    "
			}

			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			fmt.Fprintf(&b, "%s%s%s\n", prefix, connector, name)
			shown++

			if e.IsDir() {
				walk(filepath.Join(dir, e.Name()), childPrefix, currentDepth+1)
			}
		}
	}

	// Print root directory name
	rootAbs, _ := filepath.Abs(root)
	rel, _ := filepath.Rel(rootAbs, abs)
	if rel == "." {
		rel = "."
	}
	fmt.Fprintf(&b, "%s/\n", rel)
	walk(abs, "", 1)

	if b.Len() == 0 {
		return "empty directory", false
	}

	return truncate(b.String()), false
}

func truncate(s string) string {
	if len(s) > maxOutputBytes {
		return s[:maxOutputBytes] + "\n... (truncated)"
	}
	return s
}

