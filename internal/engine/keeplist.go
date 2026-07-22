package engine

import (
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// NormalizeKeepPath converts a user-supplied path — relative to start's working
// directory, or absolute — to a clean, slash-separated repo-root-relative path.
// The base is ProjectRoot, computed with Go filepath just like the absolute path
// itself, so the two share one symlink treatment and the result matches git
// status's own repo-root-relative output that tp resume classifies against
// (§7.1). Glob metacharacters (* ? [ ]) pass through unchanged, so a pattern
// given from a subdirectory is stored with that subdirectory prefixed. An error
// is returned only when the working directory or the relative path cannot be
// resolved.
func NormalizeKeepPath(start, path string) (string, error) {
	abs := path
	if !filepath.IsAbs(abs) {
		wd, err := filepath.Abs(start)
		if err != nil {
			return "", err
		}
		abs = filepath.Join(wd, path)
	}
	rel, err := filepath.Rel(ProjectRoot(start), abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// LoadKeepList returns the keep_uncommitted entries in .tp/local.json discovered
// from start, or an empty slice when there is no .tp/, no local.json, no
// keep_uncommitted key, or the file is unreadable. It never returns nil.
func LoadKeepList(start string) []model.KeepEntry {
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return []model.KeepEntry{}
	}
	lc, _, err := LoadLocalConfig(tpDir)
	if err != nil || lc.KeepUncommitted == nil {
		return []model.KeepEntry{}
	}
	return *lc.KeepUncommitted
}

// UpdateKeepList reads .tp/local.json under flock, applies mutate to the current
// keep-list (never nil), and writes the result back, preserving the active
// pointer and flag defaults. The .tp/ directory is created at the project root
// when absent (with its .gitignore, keeping local.json ignored). After this call
// keep_uncommitted is always present — an explicit empty list marshals as [] —
// so the caller controls presence through mutate's return.
func UpdateKeepList(start string, mutate func([]model.KeepEntry) []model.KeepEntry) error {
	tpDir := ProjectConfigDir(start)
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		return err
	}
	localPath := filepath.Join(tpDir, "local.json")
	return WithFileLock(localPath, func() error {
		lc, _, err := LoadLocalConfig(tpDir)
		if err != nil {
			return err
		}
		cur := []model.KeepEntry{}
		if lc.KeepUncommitted != nil {
			cur = *lc.KeepUncommitted
		}
		next := mutate(cur)
		if next == nil {
			next = []model.KeepEntry{}
		}
		lc.KeepUncommitted = &next
		return WriteLocalConfig(tpDir, lc)
	})
}
