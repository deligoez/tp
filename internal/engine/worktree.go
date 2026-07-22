package engine

import (
	"os/exec"
	"strings"
)

// WorktreeChanges runs `git status --porcelain=v1 -z -uall` at the repository
// root discovered from start and returns the repo-root-relative paths of every
// changed file: staged and unstaged tracked modifications, and each non-ignored
// untracked file enumerated individually (-uall never collapses a directory).
// git's --porcelain base is the repository root, so running at the root makes
// every reported path repo-root-relative — the same base the keep-list uses
// (§4.5, §7.1).
//
// A staged rename or copy yields only its destination path; the -z record's
// origin field is consumed and ignored. An unstaged rename is not a git rename
// status and surfaces as its constituent delete and untracked-add paths. When
// start is not inside a git repository — or git cannot be run — the result is an
// empty slice (git state cannot be read, so there is nothing to classify). The
// paths preserve git's emission order; the caller sorts.
func WorktreeChanges(start string) []string {
	root := gitToplevel(start)
	if root == "" {
		return []string{}
	}
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "-uall")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return []string{}
	}
	return parsePorcelainZ(string(out))
}

// gitToplevel returns the absolute repository root for start via
// `git rev-parse --show-toplevel`, or "" when start is not inside a git
// repository or git cannot be run.
func gitToplevel(start string) string {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = start
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// parsePorcelainZ parses a NUL-separated `git status --porcelain=v1 -z` stream
// into the changed file paths. Each entry is "XY<space><path>"; a staged rename
// or copy (index status R or C) is followed by an extra NUL field carrying the
// origin path, which is consumed and ignored so only the destination is
// reported. The trailing empty field after the final NUL is skipped.
func parsePorcelainZ(s string) []string {
	fields := strings.Split(s, "\x00")
	paths := make([]string, 0, len(fields))
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		// A valid record is at least "XY p" (two status chars, a space, and one
		// path char); shorter fields — including the trailing empty one — are skipped.
		if len(entry) < 4 {
			continue
		}
		index := entry[0]
		paths = append(paths, entry[3:])
		if index == 'R' || index == 'C' {
			i++ // skip the origin path that follows a staged rename/copy
		}
	}
	return paths
}
