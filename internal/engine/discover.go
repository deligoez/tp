package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiscoverTaskFile finds the task file in the given directory.
// Priority: --file flag > TP_FILE env var > .tp/local.json active pointer >
// auto-detect. The legacy .tp-active marker (deprecated in v0.24.0) was removed
// in v0.25.0 and is no longer read. Auto-detect scans dir for *.tasks.json
// files, then one level of subdirectories.
func DiscoverTaskFile(dir, explicit string) (string, error) {
	path, _, err := DiscoverTaskFileVia(dir, explicit)
	return path, err
}

// DiscoverTaskFileVia is DiscoverTaskFile that also reports whether the
// .tp/local.json active pointer is what chose the file. A write command needs
// the difference: the pointer outlives the spec it was set for, so a write it
// resolves may land in a task file the caller has stopped working on.
func DiscoverTaskFileVia(dir, explicit string) (path string, viaPointer bool, err error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err != nil {
			return "", false, fmt.Errorf("task file not found: %s", explicit)
		}
		return explicit, false, nil
	}

	if envFile := os.Getenv("TP_FILE"); envFile != "" {
		if _, err := os.Stat(envFile); err != nil {
			return "", false, fmt.Errorf("TP_FILE task file not found: %s", envFile)
		}
		return envFile, false, nil
	}

	// .tp/local.json active pointer (project-root-relative). A dangling pointer
	// falls through to auto-detect.
	if active := ResolveLocalActive(dir); active != "" {
		if _, statErr := os.Stat(active); statErr == nil {
			return active, true, nil
		}
		fmt.Fprintf(os.Stderr, "warning: .tp/local.json active points to a missing file %q; continuing discovery\n", active)
	}

	matches := findTaskFiles(dir)

	// If nothing in current dir, try one level of subdirectories
	if len(matches) == 0 {
		for _, sub := range childDirs(dir) {
			matches = append(matches, findTaskFiles(sub)...)
		}
	}

	switch len(matches) {
	case 0:
		return "", false, fmt.Errorf("no task file found. Run tp init <spec.md> or set TP_FILE=<path>")
	case 1:
		return matches[0], false, nil
	default:
		return "", false, fmt.Errorf("multiple task files: %s. Set TP_FILE=<path> or use tp --file <path> <command>", strings.Join(matches, ", "))
	}
}

// OtherTaskFiles returns the absolute paths of the *.tasks.json files in reach
// of dir other than path: where auto-detect looks from dir — dir itself and its
// non-hidden immediate subdirectories — plus path's own directory, so a pointer
// into a nested spec directory still sees its siblings.
func OtherTaskFiles(dir, path string) []string {
	self, err := filepath.Abs(path)
	if err != nil {
		self = path
	}
	dirs := append([]string{dir, filepath.Dir(self)}, childDirs(dir)...)

	seen := map[string]bool{self: true}
	others := make([]string, 0)
	for _, d := range dirs {
		for _, m := range findTaskFiles(d) {
			abs, absErr := filepath.Abs(m)
			if absErr != nil || seen[abs] {
				continue
			}
			seen[abs] = true
			others = append(others, abs)
		}
	}
	return others
}

// childDirs returns dir's non-hidden immediate subdirectories.
func childDirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, filepath.Join(dir, e.Name()))
		}
	}
	return dirs
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
