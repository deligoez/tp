package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// TaskFileList is tp export --list's payload (spec/1.8.0.md §16): one row per
// task file of the repository, bound or not, in phase order.
type TaskFileList struct {
	Version int           `json:"version"`
	Files   []TaskFileRow `json:"files"`
}

// TaskFileRow is one task file as tp export --list reports it: its
// repo-relative path, its spec's first heading, its epic and the epic's
// derived status, the phase tp resume reports, its tasks counted by status,
// its review and audit loops, the next step tp resume names, the escalate
// blockers that stop it, and a hash of the task file's and its spec's bytes
// that moves with an edit no other field shows. A file that cannot be read
// carries its path and Error alone.
type TaskFileRow struct {
	Path        string         `json:"path"`
	Title       string         `json:"title,omitempty"`
	Epic        *string        `json:"epic,omitempty"`
	EpicState   *string        `json:"epic_state,omitempty"`
	Phase       string         `json:"phase,omitempty"`
	Tasks       map[string]int `json:"tasks,omitempty"`
	Review      *EpicLoop      `json:"review,omitempty"`
	Audit       *EpicLoop      `json:"audit,omitempty"`
	NextStep    string         `json:"next_step,omitempty"`
	Escalations []Blocker      `json:"escalations,omitempty"`
	ContentHash string         `json:"content_hash,omitempty"`
	Error       string         `json:"error,omitempty"`

	readable bool
}

// phaseRank orders rows in the loop's order; an unreadable row sorts last.
var phaseRank = map[string]int{PhaseReview: 0, PhaseDecompose: 1, PhaseImplement: 2, PhaseAudit: 3, PhaseRelease: 4}

// TaskFileListOf lists every task file git sees under start's repository,
// tracked or untracked and not ignored, writing nothing. A file that cannot
// be read, or whose state cannot, keeps its row with the error; outside a git
// repository the list is empty. Rows are in phase order, those with an
// escalation first within a phase, then by path.
func TaskFileListOf(start string) TaskFileList {
	list := TaskFileList{Version: TaskExportVersion, Files: []TaskFileRow{}}
	root := gitToplevel(start)
	if root == "" {
		return list
	}
	bound := map[string][]string{}
	for _, path := range repoTaskFiles(root) {
		row, tf := taskFileRow(start, repoRelativeTo(root, path), path)
		if row.readable && tf.Epic != "" {
			bound[tf.Epic] = append(bound[tf.Epic], path)
		}
		list.Files = append(list.Files, row)
	}
	fillEpicStates(start, list.Files, bound)
	sort.SliceStable(list.Files, func(i, j int) bool { return taskFileRowLess(&list.Files[i], &list.Files[j]) })
	return list
}

// ListedTaskFile is whether shown is, byte for byte, the path of a row
// TaskFileListOf(start) lists, and the name that opens that row's file from
// start: shown read against the repository root, which list paths are
// relative to, whatever directory start is. It enumerates the paths alone
// and reads no task file, so asking costs one git ls-files.
func ListedTaskFile(start, shown string) (open string, listed bool) {
	root := gitToplevel(start)
	if root == "" {
		return "", false
	}
	for _, path := range repoTaskFiles(root) {
		if repoRelativeTo(root, path) == shown {
			return openFrom(start, root, shown), true
		}
	}
	return "", false
}

// openFrom spells shown, a list path relative to root, as a path that opens
// the same file from start: unchanged at the root itself, and relative to
// start's resolved directory below it. A list path that is absolute, a link
// whose target lies outside the repository, is its own name.
func openFrom(start, root, shown string) string {
	if filepath.IsAbs(shown) {
		return shown
	}
	target := filepath.Join(root, filepath.FromSlash(shown))
	dir, err := filepath.Abs(start)
	if err != nil {
		return target
	}
	if resolved, evalErr := filepath.EvalSymlinks(dir); evalErr == nil {
		dir = resolved
	}
	if rel, relErr := filepath.Rel(dir, target); relErr == nil {
		return rel
	}
	return target
}

// taskFileRow reads the row of the task file at path, shown as the list
// prints it, and returns the file it read.
func taskFileRow(start, shown, path string) (TaskFileRow, *model.TaskFile) {
	row := TaskFileRow{Path: shown}
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		row.Error = fmt.Sprintf("task file %s cannot be read: %v", row.Path, err)
		return row, nil
	}
	specPath, _ := ResolveSpecPath(path, tf.Spec)
	st, err := epicFileState(path)
	if err != nil {
		row.Error = err.Error()
		return row, nil
	}
	resume, err := AssembleResume(start, path, specPath, tf)
	if err != nil {
		row.Error = err.Error()
		return row, nil
	}
	row.readable = true
	row.Title = specTitle(specPath)
	if tf.Epic != "" {
		row.Epic = &tf.Epic
	}
	row.Phase, row.Tasks, row.Review, row.Audit = resume.Phase, st.Tasks, &st.Review, &st.Audit
	row.NextStep = resume.NextAction.Summary
	row.ContentHash = contentHash(path, specPath)
	row.Escalations = []Blocker{}
	for _, b := range resume.Blockers {
		if b.Class == ClassEscalate {
			row.Escalations = append(row.Escalations, b)
		}
	}
	return row, tf
}

// contentHash is the lowercase hex SHA-256 of the task file's bytes followed
// by its spec's, so an edit to a task's text or to the spec's moves it; a
// spec that cannot be read adds nothing.
func contentHash(taskPath, specPath string) string {
	h := sha256.New()
	for _, p := range []string{taskPath, specPath} {
		if data, err := os.ReadFile(p); err == nil { //nolint:gosec // the task file and the spec it names
			h.Write(data)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// specTitle is the spec's first heading, "" when it has none or cannot be
// read.
func specTitle(specPath string) string {
	data, err := os.ReadFile(specPath) //nolint:gosec // the spec a task file names
	if err != nil {
		return ""
	}
	for _, s := range SpecSections(data) {
		if s.Heading != "" {
			return s.Heading
		}
	}
	return ""
}

// fillEpicStates sets each bound row's epic state from the graph start's
// project holds, over the readable files bound to each epic. With no graph,
// an epic the graph does not hold, or state that cannot be derived, the
// row's epic state stays null.
func fillEpicStates(start string, rows []TaskFileRow, bound map[string][]string) {
	if len(bound) == 0 {
		return
	}
	tpDir := DiscoverTPDir(start)
	if tpDir == "" {
		return
	}
	g, _, err := LoadEpicGraph(tpDir)
	if err != nil {
		return
	}
	for id := range bound {
		slices.Sort(bound[id])
	}
	states, err := DeriveEpicStates(g, bound)
	if err != nil {
		return
	}
	for i := range rows {
		if rows[i].Epic == nil {
			continue
		}
		if st, ok := states[*rows[i].Epic]; ok {
			status := st.Status
			rows[i].EpicState = &status
		}
	}
}

// taskFileRowLess orders a before b: by phase, an escalated row first within
// a phase, then by path; an unreadable row after every readable one.
func taskFileRowLess(a, b *TaskFileRow) bool {
	if a.readable != b.readable {
		return a.readable
	}
	if ra, rb := phaseRank[a.Phase], phaseRank[b.Phase]; ra != rb {
		return ra < rb
	}
	if ea, eb := len(a.Escalations) > 0, len(b.Escalations) > 0; ea != eb {
		return ea
	}
	return a.Path < b.Path
}
