package engine

import (
	"strings"
)

// landWorktree is one entry of `git worktree list --porcelain`.
type landWorktree struct {
	path, head, branch string
	locked             bool
	lockReason         string
	main               bool
}

// PruneLandedWorktree removes the worktree at worktree and its branch once
// tp land has picked every one of commits from it
// (spec/backlog/land-lifecycle.md §2.2). It is safe only when the worktree
// has no uncommitted change, is not locked, and its branch carries no commit
// beyond commits; otherwise both stay and reason says why. found is false
// when git lists no such linked worktree, so there is nothing to report.
// The check reads the request's own commits, not `git cherry`: a land that
// resolved a conflict changes the patch ids.
func PruneLandedWorktree(root, worktree string, commits []string) (branch, reason string, found bool) {
	wt, ok := findLandWorktree(root, worktree)
	if !ok || wt.main || SameProjectRoot(wt.path, gitToplevel(root)) {
		return "", "", false
	}
	if reason = landWorktreeHold(wt, commits); reason != "" {
		return wt.branch, reason, true
	}
	if out, err := gitWaitingOutLock(root, "worktree", "remove", wt.path); err != nil {
		return wt.branch, "git worktree remove refused: " + strings.TrimSpace(string(out)), true
	}
	if wt.branch != "" {
		if out, err := gitWaitingOutLock(root, "branch", "-D", wt.branch); err != nil {
			return wt.branch, "the worktree is removed, its branch is kept: " + strings.TrimSpace(string(out)), true
		}
	}
	return wt.branch, "", true
}

// landWorktreeHold is why the worktree wt must stay: a lock, an uncommitted
// change, or a commit on its branch the land did not pick; "" when none.
func landWorktreeHold(wt landWorktree, commits []string) string {
	if wt.locked {
		if wt.lockReason != "" {
			return "the worktree is locked: " + wt.lockReason
		}
		return "the worktree is locked"
	}
	if status, err := gitOut(wt.path, "status", "--porcelain"); err != nil || status != "" {
		return "the worktree has uncommitted changes"
	}
	picked := map[string]bool{}
	for _, sha := range commits {
		if !SafeGitRev(sha) {
			continue
		}
		if full, err := gitOut(wt.path, "rev-parse", "--verify", "--quiet", sha+"^{commit}"); err == nil {
			picked[full] = true
		}
	}
	tip := wt.head
	if wt.branch != "" {
		tip = "refs/heads/" + wt.branch
	}
	if tip == "" {
		return "the worktree's HEAD cannot be read"
	}
	revs, err := gitOut(wt.path, "rev-list", tip, "--not", "--exclude=refs/heads/"+wt.branch, "--branches")
	if err != nil {
		return "the worktree's commits cannot be read"
	}
	for rev := range strings.FieldsSeq(revs) {
		if !picked[rev] {
			return "its branch has commit " + rev + ", which the land did not pick"
		}
	}
	return ""
}

// findLandWorktree is the entry of `git worktree list --porcelain` at
// root whose path is worktree.
func findLandWorktree(root, worktree string) (landWorktree, bool) {
	list, err := gitOut(root, "worktree", "list", "--porcelain")
	if err != nil {
		return landWorktree{}, false
	}
	want := CanonicalPath(worktree)
	for i, block := range strings.Split(list, "\n\n") {
		var wt landWorktree
		for line := range strings.SplitSeq(block, "\n") {
			key, value, _ := strings.Cut(line, " ")
			switch key {
			case "worktree":
				wt.path = value
			case "HEAD":
				wt.head = value
			case "branch":
				wt.branch = strings.TrimPrefix(value, "refs/heads/")
			case "locked":
				wt.locked, wt.lockReason = true, value
			}
		}
		if wt.path != "" && CanonicalPath(wt.path) == want {
			wt.main = i == 0
			return wt, true
		}
	}
	return landWorktree{}, false
}
