package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverTaskFile finds the task file in the given directory.
// If explicit is non-empty, it is returned directly (--file flag).
// Otherwise, scans dir for *.tasks.json files, then one level of subdirectories.
func DiscoverTaskFile(dir, explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("task file not found: %s", explicit)
		}
		return explicit, nil
	}

	matches := findTaskFiles(dir)

	// If nothing in current dir, try one level of subdirectories
	if len(matches) == 0 {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					subMatches := findTaskFiles(filepath.Join(dir, e.Name()))
					matches = append(matches, subMatches...)
				}
			}
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

// findTaskFiles returns *.tasks.json files in a single directory (non-recursive).
func findTaskFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".tasks.json") {
			matches = append(matches, filepath.Join(dir, e.Name()))
		}
	}
	return matches
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
