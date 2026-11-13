package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/engine"
)

// Universe notice kinds (decision 5 of
// spec/backlog/checklist-covers-what-changed.md).
const (
	noticeMergeOnlyFiles = "merge-only-files"
	noticeTasksDiffer    = "tasks-differ"
)

// universeNoticeNames caps how many paths a universe notice names on stderr.
const universeNoticeNames = 5

// universeNoticeFiles caps how many paths a universe notice's files carries
// per side; the notice's counts carry the full sizes.
const universeNoticeFiles = 20

// universeNotices says when the audited universe itself looks wrong: under
// auto-detection, files that reach the committed range only through a merge;
// under --affected-from-tasks, a difference between the files the plan's tasks
// touched and the auto-detected diff. A list named with --affected-files is
// the operator's own and draws neither.
func universeNotices(specPath string, files []string, base string, named, fromTasks bool) []auditNotice {
	dir := filepath.Dir(specPath)
	switch {
	case fromTasks:
		return tasksDifferNotice(dir, files)
	case named:
		return nil
	default:
		return mergeOnlyNotice(dir, base, files)
	}
}

// committedRev is the revision the audit's committed range starts at: --base,
// or else the latest tag; "" when there is none or it is unsafe as a git
// argument. The callers concatenate it into a range, so this is the sink.
func committedRev(dir, base string) string {
	rev := base
	if rev == "" {
		rev = latestGitTag(dir)
	}
	if rev == "" || !engine.SafeGitRev(rev) {
		return ""
	}
	return rev
}

// mergeOnlyNotice names the universe files the committed range holds only
// through a merge: in `<rev>...HEAD`, and changed by no first-parent, non-merge
// commit of `<rev>..HEAD`, so `git log --first-parent` sees fewer files than
// the audit. It compares the committed range alone; a working-tree edit to
// such a file does not change how it reached the range.
func mergeOnlyNotice(dir, base string, universe []string) []auditNotice {
	rev := committedRev(dir, base)
	if rev == "" {
		return nil
	}
	inRange := pathSet(execGitPaths(dir, "diff --name-only",
		"-c", "diff.external=", "diff", "--no-ext-diff", "--name-only", rev+"...HEAD"))
	firstParent := pathSet(execGitPaths(dir, "log --first-parent",
		"-c", "diff.external=", "log", "--first-parent", "--no-merges", "--name-only", "--format=", rev+"..HEAD"))
	var merged []string
	for _, f := range universe {
		if inRange[f] && !firstParent[f] {
			merged = append(merged, f)
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return []auditNotice{(&auditNotice{
		Kind: noticeMergeOnlyFiles,
		Message: fmt.Sprintf("%s %s...HEAD only through a merge, so --first-parent sees fewer: %s",
			countNoun(len(merged), "file reaches", "files reach"), rev, namedPaths(merged)),
		Files:     capPaths(merged),
		MergeOnly: len(merged),
	}).emit()}
}

// tasksDifferNotice compares the files --affected-from-tasks derived from the
// plan's task commits with the auto-detected diff, and names the files in one
// and not the other: a changed file no task touched is otherwise dropped
// without a word. The "outside the diff" side is reported only when a
// committed range exists: with no tag the auto-detected diff is the working
// tree alone, which every committed task file lies outside by construction.
func tasksDifferNotice(dir string, taskFiles []string) []auditNotice {
	if !gitExists(dir) {
		return nil
	}
	detected, _, _ := autoDetectedFiles(dir, "")
	tasked, inDiff := pathSet(taskFiles), pathSet(detected)
	var untasked, outside []string
	for _, f := range detected {
		if !tasked[f] {
			untasked = append(untasked, f)
		}
	}
	ranged := committedRev(dir, "") != ""
	if ranged {
		for _, f := range taskFiles {
			if !inDiff[f] {
				outside = append(outside, f)
			}
		}
	}
	if len(untasked)+len(outside) == 0 {
		return nil
	}
	n := auditNotice{Kind: noticeTasksDiffer, Untasked: intPtr(len(untasked))}
	n.Message = fmt.Sprintf("--affected-from-tasks audits %d files from task commits; %s to no task", len(taskFiles),
		countNoun(len(untasked), "changed file belongs", "changed files belong"))
	if len(untasked) > 0 {
		n.Message += " (" + namedPaths(untasked) + ")"
	}
	if ranged {
		n.Outside = intPtr(len(outside))
		n.Message += fmt.Sprintf(", and %s outside the auto-detected diff", countNoun(len(outside), "task file lies", "task files lie"))
		if len(outside) > 0 {
			n.Message += " (" + namedPaths(outside) + ")"
		}
	}
	n.Files = slices.Concat(capPaths(untasked), capPaths(outside))
	return []auditNotice{n.emit()}
}

// capPaths keeps the first universeNoticeFiles paths.
func capPaths(paths []string) []string {
	return paths[:min(len(paths), universeNoticeFiles)]
}

// intPtr returns a pointer to n, for a count that is present at zero.
func intPtr(n int) *int { return &n }

// pathSet turns a path list into a set.
func pathSet(paths []string) map[string]bool {
	set := make(map[string]bool, len(paths))
	for _, p := range paths {
		set[p] = true
	}
	return set
}

// countNoun renders "1 file reaches" or "3 files reach".
func countNoun(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// namedPaths names up to universeNoticeNames paths, then how many more.
func namedPaths(paths []string) string {
	if len(paths) <= universeNoticeNames {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:universeNoticeNames], ", ") + fmt.Sprintf(" and %d more", len(paths)-universeNoticeNames)
}
