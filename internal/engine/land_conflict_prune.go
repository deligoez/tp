package engine

import "strings"

// BranchTip is the commit refs/heads/branch points at in the repository at
// dir, "" when the branch cannot be read. It is recorded as a task's
// conflict_sha beside its conflict_branch.
func BranchTip(dir, branch string) string {
	if !SafeGitRev(branch) {
		return ""
	}
	tip, err := gitOut(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return ""
	}
	return tip
}

// PruneConflictBranch removes branch, the conflict_branch a landed task
// recorded from an earlier attempt, and the worktree that has it checked
// out, once the land carries its work. That needs two things. The branch
// still stands at recordedTip, the conflict_sha recorded with it: a branch
// that moved since (an amend, a commit, a rebase) holds work no land
// picked, and one recorded without a tip cannot show it did not. And every
// commit on the branch that HEAD lacks is carried by its own commit among
// picked, the commits this land put on HEAD for the task: the same author
// name, author email, author date and full message, what a cherry-pick
// keeps whether or not it resolved a conflict, each picked commit carrying
// one old commit at most. Otherwise both stay and reason says why, as
// PruneLandedWorktree does. worktree is "" when no worktree has the branch
// checked out; found is false when the branch no longer exists, so there is
// nothing to report.
func PruneConflictBranch(root, taskFile, branch, recordedTip string, picked []string) (worktree, reason string, found bool) {
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
	if reason = conflictBranchHold(root, taskFile, branch, &wt, hasWT); reason == "" {
		reason = conflictBranchMoved(root, branch, recordedTip)
	}
	if reason == "" {
		reason = uncarriedConflictCommit(root, ref, picked)
	}
	if reason != "" {
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

// conflictBranchHold is why the conflict branch must stay whatever its
// commits: another task records it, it is checked out in this tree, or its
// worktree wt (when hasWT) is locked or dirty.
func conflictBranchHold(root, taskFile, branch string, wt *landWorktree, hasWT bool) string {
	if owner := conflictBranchOwner(taskFile, branch); owner != "" {
		return "its branch " + branch + " is task " + owner + "'s conflict_branch: it holds the work that task's next agent resolves"
	}
	if hasWT {
		if wt.main || SameProjectRoot(wt.path, gitToplevel(root)) {
			return "its branch " + branch + " is checked out in this tree"
		}
		return worktreeStateHold(wt)
	}
	return ""
}

// conflictBranchMoved is why branch may hold work added since its conflict
// was recorded at recordedTip: no tip was recorded, or the branch no longer
// stands there. "" when it still does.
func conflictBranchMoved(root, branch, recordedTip string) string {
	if recordedTip == "" {
		return "no tip of its branch " + branch + " was recorded with the conflict (a task file from before conflict_sha), so nothing shows the branch holds only the work this land carries: delete it by hand once you have checked"
	}
	if tip := BranchTip(root, branch); tip != recordedTip {
		return "its branch " + branch + " moved since the conflict was recorded at " + recordedTip + " and now stands at " + tip + ": it may hold work no land picked"
	}
	return ""
}

// uncarriedConflictCommit names the first commit on ref that HEAD lacks and
// no commit of picked carries by authorship and message, each picked commit
// carrying one at most; "" when every one is carried.
func uncarriedConflictCommit(root, ref string, picked []string) string {
	if _, err := gitOut(root, "merge-base", "HEAD", ref); err != nil {
		return "its branch shares no history with this tree's HEAD"
	}
	// Oldest first, so of two same-key commits the later one, the one a
	// pick of the first leaves behind, is the one named.
	old, err := commitAuthorships(root, "--reverse", "HEAD.."+ref)
	if err != nil {
		return "its branch's commits cannot be read"
	}
	carriers := map[string]int{}
	revs := []string{"--no-walk=unsorted"}
	for _, sha := range picked {
		if SafeGitRev(sha) {
			revs = append(revs, sha)
		}
	}
	if len(revs) > 1 {
		landed, err := commitAuthorships(root, revs...)
		if err != nil {
			return "the commits this land picked cannot be read"
		}
		for _, c := range landed {
			carriers[c.key]++
		}
	}
	for _, c := range old {
		if carriers[c.key] == 0 {
			return "its branch has commit " + c.sha + ", which no commit this land picked carries (same author, author date and message)"
		}
		carriers[c.key]--
	}
	return ""
}

// commitAuthorship is one commit and the key a cherry-pick keeps: author
// name, author email, author date and full message.
type commitAuthorship struct {
	sha, key string
}

// commitAuthorships lists the commits git log reads from revs at root with
// their keys.
func commitAuthorships(root string, revs ...string) ([]commitAuthorship, error) {
	args := append([]string{"log", "--date=raw", "--format=%H%x00%an%x00%ae%x00%ad%x00%B%x1e"}, revs...)
	out, err := gitOut(root, args...)
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
