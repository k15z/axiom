package discovery

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Test struct {
	Name          string
	On            []string
	Condition     string
	Tags          []string
	SourceFile    string // relative path to the YAML file
	Model         string // per-test model override (empty = use global)
	Timeout       int    // per-test timeout in seconds (0 = use global)
	MaxIterations int    // per-test max_iterations (0 = use global)
}

type testDefinition struct {
	On            []string `yaml:"on"`
	Condition     string   `yaml:"condition"`
	Tags          []string `yaml:"tags"`
	Model         string   `yaml:"model"`
	Timeout       int      `yaml:"timeout"`
	MaxIterations int      `yaml:"max_iterations"`
}

func Discover(testDir string) ([]Test, error) {
	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("test directory %q not found — run `axiom init` to create it or set test_dir in axiom.yml", testDir)
	}

	// Walk the test directory recursively to find all YAML files
	var files []string
	err := filepath.WalkDir(testDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != testDir {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			rel, _ := filepath.Rel(testDir, path)
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("reading test directory %s: %w", testDir, err)
	}
	sort.Strings(files)

	var tests []Test
	seen := make(map[string]string) // name -> source file
	for _, file := range files {
		path := filepath.Join(testDir, file)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", path, err)
		}

		var raw yaml.Node
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("invalid YAML in %s: %w — check indentation and syntax", path, err)
		}

		if raw.Kind != yaml.DocumentNode || len(raw.Content) == 0 {
			continue
		}
		mapping := raw.Content[0]
		if mapping.Kind != yaml.MappingNode {
			continue
		}

		for i := 0; i < len(mapping.Content)-1; i += 2 {
			keyNode := mapping.Content[i]
			valNode := mapping.Content[i+1]

			var def testDefinition
			if err := valNode.Decode(&def); err != nil {
				return nil, fmt.Errorf("parsing test %q in %s (line %d): %w", keyNode.Value, path, keyNode.Line, err)
			}

			if keyNode.Value == "" {
				return nil, fmt.Errorf("test in %s (line %d): test name cannot be empty", path, keyNode.Line)
			}
			if strings.ContainsAny(keyNode.Value, "/\\") {
				return nil, fmt.Errorf("test %q in %s (line %d): test name cannot contain path separators", keyNode.Value, path, keyNode.Line)
			}

			if def.Condition == "" {
				return nil, fmt.Errorf("test %q in %s (line %d): condition is required — add a `condition:` field describing what to verify", keyNode.Value, path, keyNode.Line)
			}

			if prev, ok := seen[keyNode.Value]; ok {
				return nil, fmt.Errorf("duplicate test name %q: defined in %s and %s — rename one of them", keyNode.Value, prev, file)
			}
			seen[keyNode.Value] = file

			tests = append(tests, Test{
				Name:          keyNode.Value,
				On:            def.On,
				Condition:     def.Condition,
				Tags:          def.Tags,
				SourceFile:    file,
				Model:         def.Model,
				Timeout:       def.Timeout,
				MaxIterations: def.MaxIterations,
			})
		}
	}

	return tests, nil
}
