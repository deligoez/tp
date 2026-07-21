package engine

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// skipScanDirs are directory names the project-wide task-file scan never
// descends into.
var skipScanDirs = map[string]bool{
	".git": true, ".tp": true, "node_modules": true, "vendor": true,
}

// ScanProjectTaskFiles walks root recursively and returns every *.tasks.json
// path in sorted order — those directly in root and in its subdirectories. It
// skips the directories in skipScanDirs and does not descend into a nested
// submodule (a subdirectory, other than root, that contains its own .git).
func ScanProjectTaskFiles(root string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if skipScanDirs[d.Name()] || hasGitBoundary(path) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".tasks.json") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}
