package engine

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// CommitRoundFiles commits the round files at paths, new or modified, alone
// with msg on the branch checked out at root, and returns the new commit's
// short sha. A path with no change (or one git ignores) is left out, and with
// none left nothing is committed and the sha is "". Anything else staged stays
// staged and out of the commit.
func CommitRoundFiles(root string, paths []string, msg string) (string, error) {
	changed := changedPaths(root, paths)
	if len(changed) == 0 {
		return "", nil
	}
	// A round file the record just wrote is untracked, and git commit with
	// a pathspec commits only what git already knows.
	args := append([]string{"add", "--"}, changed...)
	if out, err := gitWaitingOutLock(root, args...); err != nil {
		return "", fmt.Errorf("git add of the round files failed: %s", strings.TrimSpace(string(out)))
	}
	args = append([]string{"commit", "-q", "-m", msg, "--"}, changed...)
	if out, err := gitWaitingOutLock(root, args...); err != nil {
		return "", fmt.Errorf("git commit of the round files failed: %s", strings.TrimSpace(string(out)))
	}
	return gitOut(root, "rev-parse", "--short", "HEAD")
}

// CommitRoundFilesHC is CommitRoundFiles under the hc commit_strategy: one hc
// plan naming the changed round files, the rest of the tree unplanned.
func CommitRoundFilesHC(root string, paths []string, msg string) (string, error) {
	changed := changedPaths(root, paths)
	if len(changed) == 0 {
		return "", nil
	}
	return hcCommitPaths(root, changed, msg, "the round files")
}

// changedPaths keeps the paths git status reports a change for.
func changedPaths(root string, paths []string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if status, err := gitOut(root, "status", "--porcelain", "-uall", "--", p); err == nil && status != "" {
			out = append(out, p)
		}
	}
	return out
}

// taskAuditRoundFiles are the files a task-audit record unit for seq writes in
// the spec's state directory: the task audit's round file and state.json.
func taskAuditRoundFiles(spec, seq string) []string {
	n, err := strconv.Atoi(seq)
	if err != nil {
		return nil
	}
	if abs, absErr := filepath.Abs(spec); absErr == nil {
		spec = abs
	}
	dir := CanonicalPath(ReviewStateDir(spec))
	return []string{filepath.Join(dir, TaskAuditFileName(n)), filepath.Join(dir, "state.json")}
}
