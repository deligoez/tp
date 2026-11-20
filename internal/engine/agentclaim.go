package engine

import (
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

// claimTokenRe is the shape of a spawn or session id tp records: what Claude
// Code issues (toolu_..., a UUID) and nothing a shell or JSON would read.
var claimTokenRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// ValidClaimToken reports whether s may be recorded as a spawn or session id.
func ValidClaimToken(s string) bool {
	return claimTokenRe.MatchString(s)
}

// TaskMarker is the line `tp-task: <id> <task file>` for task id of the task
// file at taskFilePath, the path relative to its repository root. Outside a
// git repository it is "": the hook that reads it claims from the root.
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
	if filepath.IsAbs(filepath.FromSlash(rel)) {
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
			if task.ClaimedBy != nil && task.ClaimedBy.Spawn == spawn {
				claimed[k] = true
			}
			if task.Status == model.StatusDone || task.Waiting != "" {
				accounted[k] = true
			}
		}
	}
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
