package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// TaskExportVersion is the version tp export's payload carries
// (spec/1.8.0.md §16).
const TaskExportVersion = 1

// The states a task of tp export carries: an open task is ready, waits on a
// task it depends on, or waits on its file's epic; a wip or done task carries
// its status.
const (
	TaskStateReady       = "ready"
	TaskStateWaitsOnTask = "waits_on_task"
	TaskStateWaitsOnEpic = "waits_on_epic"
	TaskStateWIP         = model.StatusWIP
	TaskStateDone        = model.StatusDone
)

// TaskExport is one task file's read model (spec/1.8.0.md §16): the spec in
// anchored sections, every task with its state, the ground, review and audit
// loops with their recorded rounds, the tp resume view of the file, and its
// epic and lifecycle times. Epic, BoundAt and ImportedAt are null when the
// file holds none.
type TaskExport struct {
	Version    int           `json:"version"`
	Path       string        `json:"path"`
	Spec       string        `json:"spec"`
	Epic       *string       `json:"epic"`
	BoundAt    *time.Time    `json:"bound_at"`
	ImportedAt *time.Time    `json:"imported_at"`
	Sections   []SpecSection `json:"sections"`
	Tasks      []ExportTask  `json:"tasks"`
	Loops      ExportLoops   `json:"loops"`
	Resume     ResumeResult  `json:"resume"`
}

// ExportTask is a task as the file stores it, with its state and what it
// waits on: the undone tasks it depends on, or its epic's undone
// dependencies (a group as one of {A, B}); empty when it waits on nothing.
type ExportTask struct {
	model.Task
	State   string   `json:"state"`
	WaitsOn []string `json:"waits_on"`
}

// TaskExportOf builds the read model of the task file at taskFilePath, with
// start the directory git and the keep-list are discovered from, as tp resume
// does. It writes nothing. A task file that cannot be read, or review state,
// a ground round or a graph that cannot be, is an error; an absent or
// unreadable spec has no sections.
func TaskExportOf(start, taskFilePath string) (TaskExport, error) {
	data, err := os.ReadFile(taskFilePath)
	if err != nil {
		return TaskExport{}, fmt.Errorf("task file %s: read task file: %w", taskFilePath, err)
	}
	return TaskExportOfData(start, taskFilePath, data)
}

// TaskExportOfData is TaskExportOf over data, the task file's bytes the
// caller already read from taskFilePath, so a caller that opened the file
// through a guard serves exactly what the guard let it read.
func TaskExportOfData(start, taskFilePath string, data []byte) (TaskExport, error) {
	tf, err := model.ParseTaskFile(data)
	if err != nil {
		return TaskExport{}, fmt.Errorf("task file %s: %w", taskFilePath, err)
	}
	specPath, _ := ResolveSpecPath(taskFilePath, tf.Spec)
	exp := TaskExport{Version: TaskExportVersion, Path: repoRelative(start, taskFilePath), Spec: repoRelative(start, specPath),
		BoundAt: tf.BoundAt, ImportedAt: tf.ImportedAt, Sections: []SpecSection{}}
	if tf.Epic != "" {
		exp.Epic = &tf.Epic
	}
	if specData, readErr := os.ReadFile(specPath); readErr == nil { //nolint:gosec // the spec the task file names
		exp.Sections = SpecSections(specData)
	}
	if exp.Tasks, err = exportTasks(tf, taskFilePath); err != nil {
		return TaskExport{}, err
	}
	if exp.Loops, err = exportLoops(taskFilePath, specPath); err != nil {
		return TaskExport{}, err
	}
	if exp.Resume, err = AssembleResume(start, taskFilePath, specPath, tf); err != nil {
		return TaskExport{}, err
	}
	return exp, nil
}

// repoRelative is path relative to start's git top level, slash-separated;
// path itself when there is none or path lies outside it. The top level git
// prints has its symlinks resolved, so path's are resolved too before the two
// are compared.
func repoRelative(start, path string) string {
	root := gitToplevel(start)
	abs, err := filepath.Abs(path)
	if err != nil || root == "" {
		return filepath.ToSlash(path)
	}
	if resolved, evalErr := filepath.EvalSymlinks(abs); evalErr == nil {
		abs = resolved
	}
	rel, err := filepath.Rel(root, abs)
	if slash := filepath.ToSlash(rel); err != nil || slash == ".." || strings.HasPrefix(slash, "../") {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}

// RepoRelative is path as a command prints it for the agent to run again:
// relative to the git top level of the working directory, which
// ResolveSpecArg resolves from any subdirectory; path itself outside one.
func RepoRelative(path string) string {
	return repoRelative(".", path)
}

// ResolveSpecArg resolves a spec path given on the command line: as given
// when it names a file from the working directory, else against the git top
// level, so the repo-relative paths tp resume prints run from a subdirectory
// too. An argument found by neither is returned unchanged, for the command to
// report.
func ResolveSpecArg(arg string) string {
	if arg == "" || filepath.IsAbs(arg) || fileExists(arg) {
		return arg
	}
	if root := gitToplevel("."); root != "" {
		if candidate := filepath.Join(root, arg); fileExists(candidate) {
			return candidate
		}
	}
	return arg
}

// exportTasks gives each task of tf its state. Only an open task whose own
// dependencies are done reads the epic block, which is read once.
func exportTasks(tf *model.TaskFile, taskFilePath string) ([]ExportTask, error) {
	done := doneTaskIDs(tf)
	block, err := EpicBlockFor(tf, taskFilePath)
	if err != nil {
		return nil, err
	}
	var epicWaits []string
	if block != nil && block.Blocks() {
		epicWaits = epicBlockWaits(block)
	}
	out := make([]ExportTask, 0, len(tf.Tasks))
	for i := range tf.Tasks {
		out = append(out, exportTask(&tf.Tasks[i], done, epicWaits))
	}
	return out, nil
}

// exportTask is one task with its state, given the done task ids and what
// the epic block waits on (nil when it blocks nothing).
func exportTask(t *model.Task, done map[string]bool, epicWaits []string) ExportTask {
	et := ExportTask{Task: *t, WaitsOn: []string{}}
	switch t.Status {
	case model.StatusDone, model.StatusWIP:
		et.State = t.Status
		return et
	}
	for _, dep := range t.DependsOn {
		if !done[dep] {
			et.WaitsOn = append(et.WaitsOn, dep)
		}
	}
	switch {
	case len(et.WaitsOn) > 0:
		et.State = TaskStateWaitsOnTask
	case epicWaits != nil:
		et.State = TaskStateWaitsOnEpic
		et.WaitsOn = append(et.WaitsOn, epicWaits...)
	default:
		et.State = TaskStateReady
	}
	return et
}

// epicBlockWaits names what b waits on: each plain dependency by id, then
// each group as one of {A, B}, as Describe words them.
func epicBlockWaits(b *EpicBlock) []string {
	out := append([]string{}, b.BlockedBy...)
	for _, members := range b.BlockedByAnyOf {
		out = append(out, "one of {"+strings.Join(members, ", ")+"}")
	}
	return out
}
