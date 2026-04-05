package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// readTPActive reads the .tp-active file in the given directory.
// Returns the trimmed path or empty string if not found.
func readTPActive(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, ".tp-active"))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// DiscoverTaskFile finds the task file in the given directory.
// Priority: --file flag > TP_FILE env var > directory scan.
// Otherwise, scans dir for *.tasks.json files, then one level of subdirectories.
func DiscoverTaskFile(dir, explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", fmt.Errorf("task file not found: %s", explicit)
		}
		return explicit, nil
	}

	if envFile := os.Getenv("TP_FILE"); envFile != "" {
		if _, err := os.Stat(envFile); err != nil {
			return "", fmt.Errorf("TP_FILE task file not found: %s", envFile)
		}
		return envFile, nil
	}

	// Check .tp-active in CWD
	if activeFile, err := readTPActive(dir); err == nil && activeFile != "" {
		// Resolve relative to the directory containing .tp-active
		resolved := filepath.Join(dir, activeFile)
		if _, statErr := os.Stat(resolved); statErr != nil {
			return "", fmt.Errorf("task file from .tp-active not found: %s. Run tp use --clear or tp use <new-file>", resolved)
		}
		return resolved, nil
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
		return "", fmt.Errorf("no task file found. Run tp init <spec.md> or set TP_FILE=<path>")
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("multiple task files: %s. Set TP_FILE=<path> or use tp --file <path> <command>", strings.Join(matches, ", "))
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

// ResolveSpecPath resolves a spec path, trying multiple strategies:
// 1. Relative to task file's directory
// 2. Relative to CWD
// 3. As absolute path
func ResolveSpecPath(taskFilePath, specField string) (string, bool) {
	// Strategy 1: relative to task file directory
	dir := filepath.Dir(taskFilePath)
	resolved := filepath.Join(dir, specField)
	if _, err := os.Stat(resolved); err == nil {
		return resolved, true
	}

	// Strategy 2: relative to CWD
	if _, err := os.Stat(specField); err == nil {
		absPath, _ := filepath.Abs(specField)
		return absPath, true
	}

	// Strategy 3: spec field might be just the filename, try same dir as task file
	base := filepath.Base(specField)
	sameDirPath := filepath.Join(dir, base)
	if sameDirPath != resolved {
		if _, err := os.Stat(sameDirPath); err == nil {
			return sameDirPath, true
		}
	}

	return resolved, false
}
