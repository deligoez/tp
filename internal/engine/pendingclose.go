package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PendingClose is a unit's request to close its task, written by tp done in a
// linked worktree in place of the close itself. The driver integrates it: it
// cherry-picks Commits onto the main branch, gates there, and closes the task
// with the picked shas (spec/backlog/parallel-implement.md §2.4).
type PendingClose struct {
	ID       string   `json:"id"`
	Reason   string   `json:"reason"`
	Commits  []string `json:"commits"`
	Worktree string   `json:"worktree"`
	At       string   `json:"at"`
}

// PendingCloseDir is the cycle's pending-close directory,
// .tp/runs/pending/<key> under the project root: under the ignored runs/
// directory, and keyed by cycle rather than by run so a crashed run's requests
// reach the next one.
func PendingCloseDir(root, taskFile string) string {
	return absoluteUnder(root, filepath.Join(".tp", "runs", "pending", CycleKey(root, taskFile)))
}

func pendingClosePath(root, taskFile, id string) string {
	return filepath.Join(PendingCloseDir(root, taskFile), id+".json")
}

// WritePendingClose records p, replacing an earlier request for the same task.
func WritePendingClose(root, taskFile string, p *PendingClose) error {
	record := *p
	if record.At == "" {
		record.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	path := pendingClosePath(root, taskFile, record.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// ReadPendingCloses returns the cycle's pending closes in id order; none is an
// empty slice. A file that cannot be parsed is an error: a request the driver
// cannot read is work it must not silently drop.
func ReadPendingCloses(root, taskFile string) ([]PendingClose, error) {
	dir := PendingCloseDir(root, taskFile)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []PendingClose{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make([]PendingClose, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(dir, e.Name()))
		if readErr != nil {
			return nil, readErr
		}
		var p PendingClose
		if jsonErr := json.Unmarshal(data, &p); jsonErr != nil {
			return nil, fmt.Errorf("pending close %s: %w", e.Name(), jsonErr)
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// pendingCloseIDs names the task file's pending closes, nil when there are
// none or they cannot be read: tp resume only reports them.
func pendingCloseIDs(taskFile string) []string {
	pending, err := ReadPendingCloses(CanonicalProjectRoot(filepath.Dir(taskFile)), taskFile)
	if err != nil || len(pending) == 0 {
		return nil
	}
	ids := make([]string, len(pending))
	for i := range pending {
		ids[i] = pending[i].ID
	}
	return ids
}

// pendingCloseAction is tp resume's next action while pending closes wait:
// their commits are finished work, so the next step integrates them rather
// than resuming the wip task they belong to.
func pendingCloseAction(ids []string) NextAction {
	command := "tp run"
	return NextAction{
		Command: &command,
		Summary: fmt.Sprintf("integrate the finished work of %s: tp run picks it onto the main branch and closes it before anything else", strings.Join(ids, ", ")),
		Payload: map[string]any{"pending_closes": ids},
	}
}

// HasPendingClose reports whether the cycle holds a pending close for id.
func HasPendingClose(root, taskFile, id string) bool {
	return fileExists(pendingClosePath(root, taskFile, id))
}

// RemovePendingClose deletes the request for id; an absent one is not an error.
func RemovePendingClose(root, taskFile, id string) error {
	if err := os.Remove(pendingClosePath(root, taskFile, id)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LinkedWorktreeOf reports whether dir is in a linked worktree of the
// repository whose main tree holds root: the two share a git common directory
// and have different top levels.
func LinkedWorktreeOf(dir, root string) bool {
	top := gitToplevel(dir)
	if top == "" || SameProjectRoot(top, gitToplevel(root)) {
		return false
	}
	common := gitCommonDir(dir)
	return common != "" && common == gitCommonDir(root)
}

func gitCommonDir(dir string) string {
	out, err := gitOut(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return ""
	}
	return CanonicalPath(out)
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CheckUnitCommits refuses commits a unit cannot ask to have integrated: a sha
// that is not a commit, one not on the worktree's branch, one already on the
// main branch, and one that touches a tp-owned path — the task file, the
// spec's review state or the project's .tp/ directory, which only tp writes.
func CheckUnitCommits(root, worktree, taskFile, spec string, commits []string) error {
	mainHead, err := gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	for _, sha := range commits {
		if !SafeGitRev(sha) {
			return fmt.Errorf("commit %q is not a revision", sha)
		}
		full, revErr := gitOut(worktree, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
		if revErr != nil {
			return fmt.Errorf("commit %s is not a commit of this repository", sha)
		}
		if _, ancErr := gitOut(worktree, "merge-base", "--is-ancestor", full, "HEAD"); ancErr != nil {
			return fmt.Errorf("commit %s is not on the unit's branch", sha)
		}
		if _, onMain := gitOut(root, "merge-base", "--is-ancestor", full, mainHead); onMain == nil {
			return fmt.Errorf("commit %s is already on the main branch", sha)
		}
		files, diffErr := gitOut(worktree, "diff-tree", "--no-commit-id", "--name-only", "-r", "-m", full)
		if diffErr != nil {
			return diffErr
		}
		for _, f := range strings.Split(files, "\n") {
			if f != "" && tpOwnedPath(root, taskFile, spec, f) {
				return fmt.Errorf("commit %s changes %s, which only tp writes", sha, f)
			}
		}
	}
	return nil
}

// tpOwnedPath reports whether the repo-relative path p is one only tp writes.
func tpOwnedPath(root, taskFile, spec, p string) bool {
	// All three are canonicalized so a symlinked temp root (macOS's /var)
	// cannot make the task file look like it lies outside the project.
	root, taskFile, spec = CanonicalPath(root), CanonicalPath(taskFile), CanonicalPath(spec)
	taskRel, err := NormalizeKeepPath(root, taskFile)
	if err != nil {
		taskRel = taskFile
	}
	return p == taskRel ||
		strings.HasPrefix(p, tpOwnedPrefix(root, filepath.Dir(ReviewStateDir(spec)), ".tp-review")) ||
		strings.HasPrefix(p, tpOwnedPrefix(root, ProjectConfigDir(root), ".tp"))
}
