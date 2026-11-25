package engine

import "strings"

// PruneConflictBranch removes branch, the conflict_branch a landed task
// recorded from an earlier attempt, and the worktree that has it checked
// out, once the land carries its work: every commit on branch that HEAD
// lacks has a commit on HEAD, since the two diverged, with the same author
// name, author email, author date and full message — what a cherry-pick
// keeps, whether or not it resolved a conflict. Otherwise both stay and
// reason says why, as PruneLandedWorktree does. worktree is "" when no
// worktree has the branch checked out; found is false when the branch no
// longer exists, so there is nothing to report.
func PruneConflictBranch(root, taskFile, branch string) (worktree, reason string, found bool) {
	if !SafeGitRev(branch) {
		return "", "", false
	}
	ref := "refs/heads/" + branch
	if _, err := gitOut(root, "rev-parse", "--verify", "--quiet", ref); err != nil {
		return "", "", false
	}
	wt, hasWT := findWorktreeWhere(root, func(w *landWorktree) bool { return w.branch == branch })
	if hasWT {
		worktree = wt.path
	}
	if reason = conflictBranchHold(root, taskFile, branch, ref, &wt, hasWT); reason != "" {
		return worktree, reason, true
	}
	if hasWT {
		if out, err := gitWaitingOutLock(root, "worktree", "remove", wt.path); err != nil {
			return worktree, "git worktree remove refused: " + strings.TrimSpace(string(out)), true
		}
	}
	if out, err := gitWaitingOutLock(root, "branch", "-D", branch); err != nil {
		return worktree, "the branch is kept: " + strings.TrimSpace(string(out)), true
	}
	return worktree, "", true
}

// conflictBranchHold is why the conflict branch at ref must stay: another
// task records it, it is checked out in this tree, its worktree wt (when
// hasWT) is locked or dirty, or HEAD does not carry one of its commits.
func conflictBranchHold(root, taskFile, branch, ref string, wt *landWorktree, hasWT bool) string {
	if owner := conflictBranchOwner(taskFile, branch); owner != "" {
		return "its branch " + branch + " is task " + owner + "'s conflict_branch: it holds the work that task's next agent resolves"
	}
	if hasWT {
		if wt.main || SameProjectRoot(wt.path, gitToplevel(root)) {
			return "its branch " + branch + " is checked out in this tree"
		}
		if reason := worktreeStateHold(wt); reason != "" {
			return reason
		}
	}
	return uncarriedConflictCommit(root, ref)
}

// uncarriedConflictCommit names the first commit on ref that HEAD lacks and
// no commit on HEAD since their merge base carries by authorship and
// message; "" when every one is carried. Only commits since the merge base
// are read, never the whole history.
func uncarriedConflictCommit(root, ref string) string {
	base, err := gitOut(root, "merge-base", "HEAD", ref)
	if err != nil {
		return "its branch shares no history with this tree's HEAD"
	}
	old, err := commitAuthorships(root, "HEAD.."+ref)
	if err != nil {
		return "its branch's commits cannot be read"
	}
	landed, err := commitAuthorships(root, base+"..HEAD")
	if err != nil {
		return "this tree's commits cannot be read"
	}
	carried := make(map[string]bool, len(landed))
	for _, c := range landed {
		carried[c.key] = true
	}
	for _, c := range old {
		if !carried[c.key] {
			return "its branch has commit " + c.sha + ", which no landed commit carries (same author, author date and message)"
		}
	}
	return ""
}

// commitAuthorship is one commit and the key a cherry-pick keeps: author
// name, author email, author date and full message.
type commitAuthorship struct {
	sha, key string
}

// commitAuthorships lists the commits of revRange at root with their keys.
func commitAuthorships(root, revRange string) ([]commitAuthorship, error) {
	out, err := gitOut(root, "log", "--date=raw", "--format=%H%x00%an%x00%ae%x00%ad%x00%B%x1e", revRange)
	if err != nil {
		return nil, err
	}
	var commits []commitAuthorship
	for record := range strings.SplitSeq(out, "\x1e") {
		record = strings.TrimLeft(record, "\n")
		if record == "" {
			continue
		}
		sha, key, _ := strings.Cut(record, "\x00")
		commits = append(commits, commitAuthorship{sha: sha, key: key})
	}
	return commits, nil
}
