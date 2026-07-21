package engine

import (
	"os"
	"path/filepath"
)

// hasGitBoundary reports whether dir contains a .git entry (a directory in a
// normal clone, or a file in a git worktree or submodule).
func hasGitBoundary(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// DiscoverTPDir discovers the project's .tp/ directory exactly once per
// invocation by walking upward from start, testing each ancestor — including
// start itself and the git-boundary directory — and stopping at the first
// ancestor that contains a .tp/ directory.
//
// The walk halts at the repository boundary (the first ancestor containing a
// .git directory or file) or the filesystem root, whichever comes first, and
// never reads a .tp/ above that boundary. It returns the absolute path to the
// discovered .tp/ directory, or "" when none is found within the boundary.
func DiscoverTPDir(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if info, statErr := os.Stat(filepath.Join(dir, ".tp")); statErr == nil && info.IsDir() {
			return filepath.Join(dir, ".tp")
		}
		// Stop after testing the git-boundary directory itself.
		if hasGitBoundary(dir) {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "" // filesystem root reached
		}
		dir = parent
	}
}
