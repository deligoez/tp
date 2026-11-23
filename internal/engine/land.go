package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LandRequest is a worktree agent's request to close a task a hand-driven
// coordinator claimed (spec/backlog/worktree-land.md): the agent's commits
// and evidence, waiting for tp land in the coordinator's tree to pick the
// commits onto its branch and close the task there.
type LandRequest struct {
	ID       string   `json:"id"`
	Reason   string   `json:"reason"`
	Commits  []string `json:"commits"`
	Worktree string   `json:"worktree"`
	// Spawn is the claim the request was made under; only a tree whose
	// copy has the task wip under it lands the request.
	Spawn string `json:"spawn,omitempty"`
	At    string `json:"at"`
}

// LandDir is where the land requests of the task file at taskFile live: in
// the git common directory, so every worktree reads the same requests and
// removing the agent's worktree removes none, keyed by the task file's path
// relative to its worktree's top level. It is "" outside a git repository.
func LandDir(taskFile string) string {
	abs, err := filepath.Abs(taskFile)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	top, common := gitToplevel(dir), gitCommonDir(dir)
	if top == "" || common == "" {
		return ""
	}
	return LandDirFor(common, repoRelativeTo(top, abs))
}

// LandDirFor is LandDir from a git common directory and a task file's path
// relative to its worktree's top level.
func LandDirFor(common, rel string) string {
	sum := sha256.Sum256([]byte(filepath.ToSlash(rel)))
	return filepath.Join(common, "tp", "land", hex.EncodeToString(sum[:8]))
}

// WriteLandRequest records r for the task file at taskFile, replacing an
// earlier request for the same task.
func WriteLandRequest(taskFile string, r *LandRequest) error {
	dir := LandDir(taskFile)
	if dir == "" {
		return fmt.Errorf("a land request needs a git repository")
	}
	record := *r
	if record.At == "" {
		record.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return err
	}
	if err := writeFileAtomic(filepath.Join(dir, record.ID+".json"), append(data, '\n')); err != nil {
		return err
	}
	// A new request supersedes what an earlier landing of the task did.
	if err := os.Remove(filepath.Join(dir, "outcome", record.ID+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ReadLandRequests returns the land requests in dir in id order, and the
// paths of request files it could not read or parse; none is an empty slice.
// An unreadable request is not dropped: it is named, so the caller can say
// how to clear it while the readable ones still land.
func ReadLandRequests(dir string) (requests []LandRequest, unreadable []string, err error) {
	requests, unreadable = []LandRequest{}, []string{}
	if dir == "" {
		return requests, unreadable, nil
	}
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return requests, unreadable, nil
	}
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, readErr := os.ReadFile(path)
		var r LandRequest
		if readErr != nil || json.Unmarshal(data, &r) != nil || r.ID+".json" != e.Name() {
			unreadable = append(unreadable, path)
			continue
		}
		requests = append(requests, r)
	}
	sort.Slice(requests, func(i, j int) bool { return requests[i].ID < requests[j].ID })
	return requests, unreadable, nil
}

// HasLandRequest reports whether dir holds a land request for id.
func HasLandRequest(dir, id string) bool {
	return dir != "" && fileExists(filepath.Join(dir, id+".json"))
}

// LandBlockedError is a pick this tree's own state stands in the way of:
// uncommitted changes in Paths, or a pick git refused for a reason that is
// not a conflict with the commits (Cause).
type LandBlockedError struct {
	Paths []string
	Cause string
	// Abort is the git command that clears an operation in progress.
	Abort string
}

func (e *LandBlockedError) Error() string {
	if e.Cause != "" {
		return "git refused the pick: " + e.Cause
	}
	return "uncommitted changes in " + strings.Join(e.Paths, ", ") + " stand in the way of the pick"
}

// LandPick puts a land request's commits on the branch checked out at root
// and returns the HEAD it started from and the commits now carrying the work.
// Commits already on HEAD (a hand merge of the same commits, or a pick a
// crash left behind) are not picked again. A commit that no longer exists, or
// one that conflicts with the branch, is an error naming it, with HEAD
// restored. This tree's own state is a *LandBlockedError, before anything is
// picked or after it is taken back: a staged change, an unstaged one in a
// path the commits change, or a pick git refused without a conflict.
func LandPick(root string, commits []string) (pre string, landed []string, err error) {
	pre, err = gitOut(root, "rev-parse", "HEAD")
	if err != nil {
		return "", nil, err
	}
	onHead := true
	for _, sha := range commits {
		if !SafeGitRev(sha) {
			return pre, nil, fmt.Errorf("commit %q is not a revision", sha)
		}
		if _, verErr := gitOut(root, "rev-parse", "--verify", "--quiet", sha+"^{commit}"); verErr != nil {
			return pre, nil, fmt.Errorf("commit %s no longer exists", sha)
		}
		if _, ancErr := gitOut(root, "merge-base", "--is-ancestor", sha, "HEAD"); ancErr != nil {
			onHead = false
		}
	}
	if onHead {
		return pre, commits, nil
	}
	if blocked := landBlockingPaths(root, commits); len(blocked) > 0 {
		return pre, nil, &LandBlockedError{Paths: blocked}
	}
	if op, abort := gitOperationInProgress(root); op != "" {
		return pre, nil, &LandBlockedError{Cause: op + " is in progress in this tree", Abort: abort}
	}
	picked := make([]string, 0, len(commits))
	for _, sha := range commits {
		landedAs, pickErr := pickOne(root, sha)
		if pickErr != nil {
			_ = LandUndo(root, pre)
			return pre, nil, pickErr
		}
		picked = append(picked, landedAs)
	}
	return pre, picked, nil
}

// pickOne cherry-picks sha onto HEAD at root and returns the commit that now
// carries its change: the new pick, or the commit on HEAD that already
// carries the same change (a pick a crashed run left, a hand merge under
// another sha, or inside a larger commit), which is not picked again. A
// conflict or a merge commit is an error naming the commit; any other refusal
// is this tree's own state, a *LandBlockedError carrying git's message.
func pickOne(root, sha string) (string, error) {
	if parents, err := gitOut(root, "rev-list", "--parents", "-n", "1", sha); err == nil && len(strings.Fields(parents)) > 2 {
		return "", fmt.Errorf("commit %s is a merge commit: tp land picks commits with one parent", sha)
	}
	pick := exec.Command("git", "cherry-pick", "--allow-empty", sha)
	pick.Dir = root
	out, pickErr := pick.CombinedOutput()
	if pickErr == nil {
		return gitOut(root, "rev-parse", "HEAD")
	}
	conflicted, _ := gitOut(root, "diff", "--name-only", "--diff-filter=U")
	if conflicted != "" {
		_, _ = gitOut(root, "cherry-pick", "--abort")
		return "", fmt.Errorf("commit %s conflicts with this branch in %s", sha, strings.Join(strings.Fields(conflicted), ", "))
	}
	if emptyPick(root) {
		_, _ = gitOut(root, "cherry-pick", "--skip")
		if same := equivalentOnHead(root, sha); same != "" {
			return same, nil
		}
		return gitOut(root, "rev-parse", "HEAD")
	}
	_, _ = gitOut(root, "cherry-pick", "--abort")
	return "", &LandBlockedError{Cause: strings.TrimSpace(string(out))}
}

// emptyPick reports a cherry-pick stopped because its change is already on
// HEAD: one is in progress and nothing is staged against HEAD.
func emptyPick(root string) bool {
	if _, err := gitOut(root, "rev-parse", "-q", "--verify", "CHERRY_PICK_HEAD"); err != nil {
		return false
	}
	_, err := gitOut(root, "diff", "--cached", "--quiet")
	return err == nil
}

// gitOperationInProgress names a cherry-pick, merge, revert or rebase this
// tree is in the middle of; "" when none.
func gitOperationInProgress(root string) (name, abort string) {
	for _, op := range []struct{ ref, name, abort string }{
		{"CHERRY_PICK_HEAD", "a cherry-pick", "git cherry-pick --abort"},
		{"MERGE_HEAD", "a merge", "git merge --abort"},
		{"REVERT_HEAD", "a revert", "git revert --abort"},
		{"REBASE_HEAD", "a rebase", "git rebase --abort"},
	} {
		if _, err := gitOut(root, "rev-parse", "-q", "--verify", op.ref); err == nil {
			return op.name, op.abort
		}
	}
	return "", ""
}

// equivalentOnHead returns the commit between sha's merge base with HEAD and
// HEAD whose change has sha's patch id; "" when none does.
func equivalentOnHead(root, sha string) string {
	id := patchID(root, sha)
	if id == "" {
		return ""
	}
	base, err := gitOut(root, "merge-base", "HEAD", sha)
	if err != nil {
		return ""
	}
	revs, err := gitOut(root, "rev-list", "--no-merges", base+"..HEAD")
	if err != nil {
		return ""
	}
	for rev := range strings.SplitSeq(revs, "\n") {
		if rev != "" && patchID(root, rev) == id {
			return rev
		}
	}
	return ""
}

// patchID is the stable patch id of commit rev's change; "" when it has none.
func patchID(root, rev string) string {
	diff := exec.Command("git", "diff-tree", "-p", "--root", rev)
	diff.Dir = root
	patch, err := diff.Output()
	if err != nil || len(patch) == 0 {
		return ""
	}
	id := exec.Command("git", "patch-id", "--stable")
	id.Dir = root
	id.Stdin = bytes.NewReader(patch)
	out, err := id.Output()
	if err != nil {
		return ""
	}
	if fields := strings.Fields(string(out)); len(fields) > 0 {
		return fields[0]
	}
	return ""
}

// LandUndo takes a landing's pick back to pre, keeping uncommitted changes;
// a HEAD still at pre is left alone, so nothing staged is touched.
func LandUndo(root, pre string) error {
	if !SafeGitRev(pre) {
		return fmt.Errorf("%q is not a revision", pre)
	}
	if head, err := gitOut(root, "rev-parse", "HEAD"); err == nil && head == pre {
		return nil
	}
	_, err := gitOut(root, "reset", "-q", "--keep", pre)
	return err
}

// landBlockingPaths lists what at root stands in the way of picking commits:
// every staged path (git refuses a pick over a dirty index), and every
// unstaged or untracked path the commits change.
func landBlockingPaths(root string, commits []string) []string {
	// Not gitOut: its trim would eat the status column of the first line.
	cmd := exec.Command("git", "status", "--porcelain", "-z", "--untracked-files=all")
	cmd.Dir = root
	status, err := cmd.Output()
	if err != nil || len(status) == 0 {
		return nil
	}
	dirty := map[string]bool{}
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	entries := strings.Split(string(status), "\x00")
	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < 4 {
			continue
		}
		dirty[entry[3:]] = true
		if entry[0] != ' ' && entry[0] != '?' {
			add(entry[3:])
		}
		// -z writes a rename's source as the next entry, with no status.
		if entry[0] == 'R' || entry[0] == 'C' {
			i++
		}
	}
	for _, sha := range commits {
		files, diffErr := gitOut(root, "diff-tree", "--no-commit-id", "--name-only", "-r", sha)
		if diffErr != nil {
			continue
		}
		for f := range strings.SplitSeq(files, "\n") {
			if f != "" && dirty[f] {
				add(f)
			}
		}
	}
	sort.Strings(out)
	return out
}

// RemoveLandRequest deletes the request for id in dir; an absent one is not
// an error.
func RemoveLandRequest(dir, id string) error {
	if err := os.Remove(filepath.Join(dir, id+".json")); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// LandOutcome is what tp land did with a request: landed and closed, or not
// landed for Error and the task reopened. It is kept for the agent's
// worktree, whose own copy still has the task open, so a close tried there
// after the landing is told the outcome instead of closing that copy.
type LandOutcome struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
	Landed   bool   `json:"landed"`
	Error    string `json:"error,omitempty"`
	At       string `json:"at"`
}

// WriteLandOutcome records o in dir, the task file's land directory.
func WriteLandOutcome(dir string, o *LandOutcome) error {
	record := *o
	if record.At == "" {
		record.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	out := filepath.Join(dir, "outcome")
	if err := os.MkdirAll(out, 0o750); err != nil {
		return err
	}
	data, err := json.MarshalIndent(&record, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(out, record.ID+".json"), append(data, '\n'))
}

// LandOutcomeIn returns the outcome tp land recorded for task id of the task
// file at taskFile when the request came from the worktree cwd is in; nil
// otherwise.
func LandOutcomeIn(taskFile, id, cwd string) *LandOutcome {
	dir := LandDir(taskFile)
	if dir == "" {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(dir, "outcome", id+".json"))
	if err != nil {
		return nil
	}
	var o LandOutcome
	if json.Unmarshal(data, &o) != nil || o.ID != id {
		return nil
	}
	mine, theirs := gitToplevel(cwd), gitToplevel(o.Worktree)
	if mine == "" || theirs == "" || !samePath(mine, theirs) {
		return nil
	}
	return &o
}
