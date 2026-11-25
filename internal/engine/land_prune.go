package engine

import (
	"strings"

	"github.com/deligoez/tp/internal/model"
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
// has no uncommitted change, is not locked, its branch carries no commit
// beyond commits, and no task in taskFile records that branch as its
// conflict_branch; otherwise both stay and reason says why. found is false
// when git lists no such linked worktree, so there is nothing to report.
// The check reads the request's own commits, not `git cherry`: a land that
// resolved a conflict changes the patch ids.
func PruneLandedWorktree(root, taskFile, worktree string, commits []string) (branch, reason string, found bool) {
	wt, ok := findLandWorktree(root, worktree)
	if !ok || wt.main || SameProjectRoot(wt.path, gitToplevel(root)) {
		return "", "", false
	}
	if owner := conflictBranchOwner(taskFile, wt.branch); owner != "" {
		return wt.branch, "its branch " + wt.branch + " is task " + owner + "'s conflict_branch: it holds the work that task's next agent resolves", true
	}
	if reason = landWorktreeHold(&wt, commits); reason != "" {
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

// conflictBranchOwner is the task in the task file at taskFile that records
// branch as its conflict_branch, "" when none does. It is read at prune time,
// after this land's closes cleared theirs.
func conflictBranchOwner(taskFile, branch string) string {
	if branch == "" {
		return ""
	}
	tf, err := model.ReadTaskFile(taskFile)
	if err != nil {
		return ""
	}
	for i := range tf.Tasks {
		if tf.Tasks[i].ConflictBranch == branch {
			return tf.Tasks[i].ID
		}
	}
	return ""
}

// LandConflictBranch is the branch holding a land request's work once its
// commit sha conflicted: the branch checked out in the request's worktree when
// it holds sha, else the one local branch that does. "" when neither names one,
// and the task is then reopened with no branch to resolve.
func LandConflictBranch(root, worktree, sha string) string {
	if !SafeGitRev(sha) {
		return ""
	}
	if wt, ok := findLandWorktree(root, worktree); ok && !wt.main && wt.branch != "" {
		if _, err := gitOut(root, "merge-base", "--is-ancestor", sha, "refs/heads/"+wt.branch); err == nil {
			return wt.branch
		}
	}
	if branches := branchesContaining(root, sha, "refs/heads/"); len(branches) == 1 {
		return branches[0]
	}
	return ""
}

// landWorktreeHold is why the worktree wt must stay: a lock, an uncommitted
// change, or a commit on its branch the land did not pick; "" when none.
func landWorktreeHold(wt *landWorktree, commits []string) string {
	if reason := worktreeStateHold(wt); reason != "" {
		return reason
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
	// --exclude before --branches matches the branch's short name: a
	// refs/heads/ prefix excludes nothing, the tip stays among --branches
	// and every commit beyond the request's reads as reachable.
	revs, err := gitOut(wt.path, "rev-list", tip, "--not", "--exclude="+wt.branch, "--branches")
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

// worktreeStateHold is why the worktree wt must stay whatever its commits:
// a lock or an uncommitted change; "" when neither.
func worktreeStateHold(wt *landWorktree) string {
	if wt.locked {
		if wt.lockReason != "" {
			return "the worktree is locked: " + wt.lockReason
		}
		return "the worktree is locked"
	}
	if status, err := gitOut(wt.path, "status", "--porcelain"); err != nil || status != "" {
		return "the worktree has uncommitted changes"
	}
	return ""
}

// findLandWorktree is the entry of `git worktree list --porcelain` at
// root whose path is worktree.
func findLandWorktree(root, worktree string) (landWorktree, bool) {
	want := CanonicalPath(worktree)
	return findWorktreeWhere(root, func(wt *landWorktree) bool {
		return wt.path != "" && CanonicalPath(wt.path) == want
	})
}

// findWorktreeWhere is the first entry of `git worktree list --porcelain`
// at root that match accepts; main is set on the list's first entry.
func findWorktreeWhere(root string, match func(*landWorktree) bool) (landWorktree, bool) {
	list, err := gitOut(root, "worktree", "list", "--porcelain")
	if err != nil {
		return landWorktree{}, false
	}
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
		wt.main = i == 0
		if match(&wt) {
			return wt, true
		}
	}
	return landWorktree{}, false
}
