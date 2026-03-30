package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverTaskFile finds the task file in the given directory.
// If explicit is non-empty, it is returned directly (--file flag).
// Otherwise, scans dir for *.tasks.json files.
func DiscoverTaskFile(dir, explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("task file not found: %s", explicit)
		}
		return explicit, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read directory: %w", err)
	}

	var matches []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tasks.json") {
			matches = append(matches, filepath.Join(dir, e.Name()))
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no task file found. Run `tp init <spec.md>` or `tp import <file>` to create one")
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple task files found. Use --file to specify: %s", strings.Join(matches, ", "))
	}
}

// ResolveSpecPath resolves a spec path relative to the task file's directory.
func ResolveSpecPath(taskFilePath, specField string) (string, bool) {
	dir := filepath.Dir(taskFilePath)
	resolved := filepath.Join(dir, specField)

	if _, err := os.Stat(resolved); err != nil {
		return resolved, false
	}
	return resolved, true
}
