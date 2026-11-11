package engine

import (
	"errors"
	"os"
	"path/filepath"
)

// DashboardSources reports what the repository discovered from start holds
// for tp dashboard to read: an epic graph file, and any task file — one git
// sees, or one task-file discovery finds from start (spec/1.8.0.md §1).
func DashboardSources(start string) (hasGraph, hasTaskFile bool) {
	if tpDir := DiscoverTPDir(start); tpDir != "" {
		if _, err := os.Stat(filepath.Join(tpDir, EpicGraphFile)); err == nil {
			hasGraph = true
		}
	}
	if root := gitToplevel(start); root != "" && len(repoTaskFiles(root)) > 0 {
		return hasGraph, true
	}
	_, err := DiscoverTaskFile(start, "")
	return hasGraph, err == nil || errors.Is(err, ErrMultipleTaskFiles)
}
