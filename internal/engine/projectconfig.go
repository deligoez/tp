package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// LoadProjectConfig reads and parses tpDir/config.json into a ProjectConfig.
// A missing file returns an empty ProjectConfig, which is equivalent to an
// empty object {} and contributes no overrides. Workflow fields are
// presence-tracked (WorkflowOverride uses pointers), so an absent key stays
// distinct from an explicit zero.
func LoadProjectConfig(tpDir string) (model.ProjectConfig, error) {
	var pc model.ProjectConfig
	data, err := os.ReadFile(filepath.Join(tpDir, "config.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return pc, nil
		}
		return pc, err
	}
	if err := json.Unmarshal(data, &pc); err != nil {
		return pc, err
	}
	return pc, nil
}

// LoadLocalConfig reads and parses tpDir/local.json into a LocalConfig.
// A missing file returns an empty LocalConfig (nil active, nil defaults).
func LoadLocalConfig(tpDir string) (model.LocalConfig, error) {
	var lc model.LocalConfig
	data, err := os.ReadFile(filepath.Join(tpDir, "local.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return lc, nil
		}
		return lc, err
	}
	if err := json.Unmarshal(data, &lc); err != nil {
		return lc, err
	}
	return lc, nil
}

// EnsureTPGitignore ensures tpDir/.gitignore exists and contains a "local.json"
// entry, so .tp/local.json stays git-ignored even when the .tp/ directory was
// created by hand rather than by tp. It is idempotent: it creates the file when
// absent, appends the entry when the file exists without it, and does nothing
// when the entry is already present. It is invoked whenever tp writes any file
// under .tp/.
func EnsureTPGitignore(tpDir string) error {
	path := filepath.Join(tpDir, ".gitignore")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(path, []byte("local.json\n"), 0o600)
		}
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "local.json" {
			return nil // already ignored
		}
	}
	content := string(data)
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += "local.json\n"
	return os.WriteFile(path, []byte(content), 0o600)
}

// hasGitBoundary reports whether dir contains a .git entry (a directory in a
// normal clone, or a file in a git worktree or submodule).
func hasGitBoundary(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// FindGitBoundary walks up from start and returns the first ancestor that
// contains a .git entry (directory or file), or "" when none is found up to the
// filesystem root.
func FindGitBoundary(start string) string {
	dir, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if hasGitBoundary(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ProjectRoot returns the project root for a start directory: the directory
// containing the discovered .tp/, or — when no .tp/ exists yet — the git
// boundary directory, or the start directory itself when not inside a git
// repository. Config creation and the project-wide scan use this root.
// Discovery is anchored once at start and is never re-anchored to a --file
// located in another directory. When no .tp/ is found, callers use built-in
// defaults and per-task-file settings exactly as in v0.23.0.
func ProjectRoot(start string) string {
	if tpDir := DiscoverTPDir(start); tpDir != "" {
		return filepath.Dir(tpDir)
	}
	if boundary := FindGitBoundary(start); boundary != "" {
		return boundary
	}
	if abs, err := filepath.Abs(start); err == nil {
		return abs
	}
	return start
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
