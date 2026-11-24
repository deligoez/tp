package engine

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// Agent-bound claims (spec/backlog/agent-bound-claims.md): a brief names its
// task in a marker line, the plugin's spawn hook claims that task for the
// Agent call's tool_use_id, and the plugin's stop hook asks which of the
// spawn's tasks are still unexplained.

// TaskMarkerPrefix opens the marker line a brief carries.
const TaskMarkerPrefix = "tp-task: "

// ClaimTokenPrefix opens the line under the marker that carries the brief's
// claim token; a hook older than the token ignores the line.
const ClaimTokenPrefix = "tp-claim-token: "

// NewClaimToken mints a claim token: 16 hex characters, unique enough to
// tell two agents spawned for one task apart. It is not a secret.
func NewClaimToken() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(raw[:])
}

// claimTokenRe is the shape of a spawn or session id tp records: what Claude
// Code issues (toolu_..., a UUID) and nothing a shell or JSON would read.
var claimTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// markerIDRe and markerFileRe are the shapes the plugin's spawn hook takes as
// a marker's id and task file; a task outside them gets no marker, since the
// hook would skip it without claiming.
var (
	markerIDRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	markerFileRe = regexp.MustCompile(`^[A-Za-z0-9._/-]+\.tasks\.json$`)
)

// ValidClaimToken reports whether s may be recorded as a spawn or session id.
func ValidClaimToken(s string) bool {
	return claimTokenRe.MatchString(s)
}

// TaskMarker is the line `tp-task: <id> <task file>` for task id of the task
// file at taskFilePath, the path relative to its repository root. Outside a
// git repository it is "": the hook that reads it claims from the root. It
// is "" too for an id or path the hook would not read as a marker.
func TaskMarker(taskFilePath, id string) string {
	abs, err := filepath.Abs(taskFilePath)
	if err != nil {
		return ""
	}
	root := gitToplevel(filepath.Dir(abs))
	if root == "" {
		return ""
	}
	rel := repoRelativeTo(root, abs)
	if filepath.IsAbs(filepath.FromSlash(rel)) || !markerIDRe.MatchString(id) || !markerFileRe.MatchString(rel) {
		return ""
	}
	return TaskMarkerPrefix + id + " " + rel
}

// SpawnClaim is one task a spawn claimed, and whether it is accounted for:
// done, or waiting on something, in any worktree's copy of its task file.
type SpawnClaim struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	Accounted bool   `json:"accounted"`
}

// SpawnClaims is `tp claims --spawn`'s answer.
type SpawnClaims struct {
	Spawn       string       `json:"spawn"`
	Tasks       []SpawnClaim `json:"tasks"`
	Unaccounted []string     `json:"unaccounted"`
}

type claimKey struct{ file, id string }

// ClaimsOfSpawn lists the tasks claimed by spawn across every worktree of the
// repository holding start. A coordinator claims on one tree and its agent may
// close on another before anything merges, so a task counts as accounted for
// when any worktree's copy of its file has it done or waiting. Files are
// matched by their path relative to each worktree's root.
func ClaimsOfSpawn(start, spawn string) SpawnClaims {
	out := SpawnClaims{Spawn: spawn, Tasks: []SpawnClaim{}, Unaccounted: []string{}}
	claimed := map[claimKey]bool{}
	accounted := map[claimKey]bool{}
	for _, tree := range worktreeRoots(start) {
		scanClaims(tree, spawn, claimed, accounted)
	}
	// A worktree agent's close waits as a land request until the
	// coordinator lands it (spec/backlog/worktree-land.md): asked for, so
	// accounted.
	if common := gitCommonDir(start); common != "" {
		for k := range claimed {
			if HasLandRequest(LandDirFor(common, k.file), k.id) {
				accounted[k] = true
			}
		}
	}
	keys := make([]claimKey, 0, len(claimed))
	for k := range claimed {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].file != keys[j].file {
			return keys[i].file < keys[j].file
		}
		return keys[i].id < keys[j].id
	})
	for _, k := range keys {
		out.Tasks = append(out.Tasks, SpawnClaim{ID: k.id, File: k.file, Accounted: accounted[k]})
		if !accounted[k] {
			out.Unaccounted = append(out.Unaccounted, k.id)
		}
	}
	return out
}

// scanClaims reads every task file of one worktree into claimed and accounted.
func scanClaims(tree, spawn string, claimed, accounted map[claimKey]bool) {
	for _, path := range repoTaskFiles(tree) {
		rel, err := filepath.Rel(tree, path)
		if err != nil {
			continue
		}
		tf, err := model.ReadTaskFile(path)
		if err != nil {
			continue
		}
		for i := range tf.Tasks {
			task := &tf.Tasks[i]
			k := claimKey{file: filepath.ToSlash(rel), id: task.ID}
			if task.ClaimedBy == nil || task.ClaimedBy.Spawn != spawn {
				// A copy without this claim was cut before it, or another
				// spawn holds it: what it says is not about this claim.
				continue
			}
			claimed[k] = true
			if task.Status == model.StatusDone || (task.Status == model.StatusWIP && task.Waiting != "") {
				accounted[k] = true
			}
		}
	}
}

// ClaimHolderCopy returns the copy, in another worktree of the repository, of
// the task file at path that holds task id's spawn claim, where a write about
// that claim belongs: path's copy has id open, because the coordinator
// claimed it on its own tree after this worktree was cut, so any other copy
// with id wip under a spawn's claim holds it. It returns "" otherwise: a copy
// with id wip or done speaks for itself, so tp run's own closes and a
// coordinator's own writes never look elsewhere.
func ClaimHolderCopy(path, id string) string {
	if taskStatusAt(path, id) == model.StatusOpen {
		return SpawnHolderCopy(path, id, "")
	}
	return ""
}

// LandHolderCopy is where an agent's close in its worktree lands:
// ClaimHolderCopy, and also, when path's copy in a linked worktree has id wip
// under a spawn's claim because the claim was committed before the worktree
// was cut, the main worktree's copy when it has id wip under a spawn's claim.
// A coordinator working in a linked worktree therefore leaves its claims
// uncommitted until tp land.
func LandHolderCopy(path, id string) string {
	if holder := ClaimHolderCopy(path, id); holder != "" {
		return holder
	}
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return ""
	}
	if task, _, findErr := model.FindTask(tf, id); findErr == nil && SpawnClaimed(task) {
		return mainTreeHolderCopy(path, id)
	}
	return ""
}

// taskStatusAt is id's status in the task file at path; "" when unreadable.
func taskStatusAt(path, id string) string {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return ""
	}
	task, _, findErr := model.FindTask(tf, id)
	if findErr != nil {
		return ""
	}
	return task.Status
}

// SpawnClaimed reports whether task is wip under an agent spawn's claim: an
// agent works on it, so it is that agent's, never offered to resume.
func SpawnClaimed(task *model.Task) bool {
	return task.Status == model.StatusWIP && task.ClaimedBy != nil && task.ClaimedBy.Spawn != ""
}

// AgentHeld returns the ids of tf's tasks wip under an agent spawn's claim.
func AgentHeld(tf *model.TaskFile) []string {
	var ids []string
	for i := range tf.Tasks {
		if SpawnClaimed(&tf.Tasks[i]) {
			ids = append(ids, tf.Tasks[i].ID)
		}
	}
	return ids
}

// ResumableWIP returns tf's first wip task no agent spawn holds: an
// interrupted unit, resumed before anything new starts; nil when none.
func ResumableWIP(tf *model.TaskFile) *model.Task {
	for i := range tf.Tasks {
		if t := &tf.Tasks[i]; t.Status == model.StatusWIP && !SpawnClaimed(t) {
			return t
		}
	}
	return nil
}

// mainTreeHolderCopy is the main worktree's copy of the task file at path
// when path lies in a linked worktree and that copy has id under a spawn's
// claim; "" otherwise.
func mainTreeHolderCopy(path, id string) string {
	tree := gitToplevel(filepath.Dir(path))
	if tree == "" {
		return ""
	}
	roots := worktreeRoots(tree)
	if len(roots) < 2 || samePath(roots[0], tree) {
		return ""
	}
	copyPath := filepath.Join(roots[0], filepath.FromSlash(repoRelativeTo(tree, path)))
	copyTF, err := model.ReadTaskFile(copyPath)
	if err != nil {
		return ""
	}
	if t, _, tErr := model.FindTask(copyTF, id); tErr == nil && SpawnClaimed(t) {
		return copyPath
	}
	return ""
}

// SpawnHolderCopy returns the copy, in another worktree than path's, of the
// task file at path that has task id wip under spawn's claim (any spawn's
// when spawn is ""); "" when none does.
func SpawnHolderCopy(path, id, spawn string) string {
	tree := gitToplevel(filepath.Dir(path))
	if tree == "" {
		return ""
	}
	rel := repoRelativeTo(tree, path)
	for _, other := range worktreeRoots(tree) {
		if samePath(other, tree) {
			continue
		}
		copyPath := filepath.Join(other, filepath.FromSlash(rel))
		copyTF, readErr := model.ReadTaskFile(copyPath)
		if readErr != nil {
			continue
		}
		t, _, tErr := model.FindTask(copyTF, id)
		if tErr == nil && t.Status == model.StatusWIP && t.ClaimedBy != nil && t.ClaimedBy.Spawn != "" &&
			(spawn == "" || t.ClaimedBy.Spawn == spawn) {
			return copyPath
		}
	}
	return ""
}

// CommitOnHead reports whether sha is a commit HEAD at dir contains.
func CommitOnHead(dir, sha string) bool {
	if !SafeGitRev(sha) {
		return false
	}
	_, err := gitOut(dir, "merge-base", "--is-ancestor", sha, "HEAD")
	return err == nil
}

// worktreeRoots is every worktree of the repository holding start, from
// `git worktree list --porcelain`; start's own root alone when git lists none.
func worktreeRoots(start string) []string {
	root := gitToplevel(start)
	if root == "" {
		return nil
	}
	list, err := gitOut(root, "worktree", "list", "--porcelain")
	if err != nil {
		return []string{root}
	}
	var roots []string
	for line := range strings.SplitSeq(list, "\n") {
		if tree, ok := strings.CutPrefix(line, "worktree "); ok {
			roots = append(roots, tree)
		}
	}
	if len(roots) == 0 {
		return []string{root}
	}
	return roots
}

// RepoRelativePath is path relative to the top level of its git worktree, in
// slash form; path itself outside git.
func RepoRelativePath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	top := gitToplevel(filepath.Dir(abs))
	if top == "" {
		return path
	}
	return filepath.ToSlash(repoRelativeTo(top, abs))
}

// CommitExists reports whether sha names a commit in dir's repository.
func CommitExists(dir, sha string) bool {
	if !SafeGitRev(sha) {
		return false
	}
	_, err := gitOut(dir, "rev-parse", "--verify", "--quiet", sha+"^{commit}")
	return err == nil
}

// WorktreeWithCommit returns the root of another worktree of dir's
// repository whose HEAD contains sha; "" when none does.
func WorktreeWithCommit(dir, sha string) string {
	tree := gitToplevel(dir)
	if tree == "" || !SafeGitRev(sha) {
		return ""
	}
	for _, other := range worktreeRoots(tree) {
		if !samePath(other, tree) && CommitOnHead(other, sha) {
			return other
		}
	}
	return ""
}
