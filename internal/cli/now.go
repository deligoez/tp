package cli

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// nowResult is tp now's payload (spec/backlog/panel-dashboard.md): the one
// read the plugin's pane polls, carrying exactly what it draws. Every list is
// present, empty when there is nothing; task_file, tasks, run and epics are
// null when there is none.
type nowResult struct {
	Project    string     `json:"project"`
	TaskFile   *string    `json:"task_file"`
	Phase      string     `json:"phase"`
	NextAction string     `json:"next_action"`
	NeedsYou   []nowNeed  `json:"needs_you"`
	Tasks      *nowCounts `json:"tasks"`
	Held       []nowHeld  `json:"held"`
	Landing    []nowLand  `json:"landing"`
	Ready      []nowTask  `json:"ready"`
	// Run is the current or last tp run as tp run --status reports it,
	// with last_finished, the latest unit attempt that exited.
	Run   map[string]any `json:"run"`
	Epics []nowEpic      `json:"epics"`
}

// nowNeed is one thing waiting for a person, in a plain sentence that names
// no tp command or flag: the pane shows it as it is (CLAUDE.md rule 14).
type nowNeed struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	ID   string `json:"id,omitempty"`
}

// nowCounts is tp status's counts of the task file.
type nowCounts struct {
	Total   int `json:"total"`
	Done    int `json:"done"`
	WIP     int `json:"wip"`
	Open    int `json:"open"`
	Ready   int `json:"ready"`
	Blocked int `json:"blocked"`
	// Held counts open tasks on a waiting hold: open is ready + blocked +
	// held.
	Held int `json:"held"`
}

// nowHeld is one wip task and who holds it.
type nowHeld struct {
	ID       string  `json:"id"`
	Title    string  `json:"title"`
	Holder   string  `json:"holder"`
	Since    *string `json:"since"`
	Worktree *string `json:"worktree"`
	Attempt  *int    `json:"attempt"`
}

// nowLand is one land request waiting for tp land.
type nowLand struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Since *string `json:"since"`
}

// nowTask is a task named by id and title.
type nowTask struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// nowEpic is one epic's progress over the tasks of its bound files.
type nowEpic struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
	State string `json:"state"`
}

func newNowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "now",
		Short: "What is happening now, what needs you, and how far along the work is, in one read-only payload",
		Long: `The one payload the plugin's pane polls: the phase and next action tp resume
reports, what waits for a person (needs_you), task counts, the wip tasks and who
holds each, land requests waiting, the first ready tasks, the current or last
tp run, and each epic's progress. Read-only: it writes no file.

Exit 0 whenever a project is found, even with no task file; 3 outside one.`,
		Args: cobra.NoArgs,
		RunE: runNow,
	}
	addEpicFileFlag(cmd)
	return cmd
}

func runNow(_ *cobra.Command, _ []string) error {
	applyEpicFileFlag()
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		if errors.Is(err, engine.ErrMultipleTaskFiles) || errors.Is(err, engine.ErrExplicitTaskFileMissing) {
			exitNow(err.Error(), "pass --file <task file>, or set the active one with tp use <file>")
		}
		if engine.DiscoverTPDir(".") == "" {
			exitNow("no tp project found: no task file and no .tp/ directory up to the git boundary", "run tp init <spec> in the project")
		}
		res := emptyNow(engine.ProjectRoot("."))
		return output.JSON(res)
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		exitNow(err.Error(), "")
	}
	res, err := buildNow(taskFilePath, tf)
	if err != nil {
		var sce *engine.StateCorruptError
		if errors.As(err, &sce) {
			exitNow(sce.Error(), sce.Hint())
		}
		exitNow(err.Error(), "")
	}
	return output.JSON(res)
}

// exitNow ends tp now with exit 3 and the standard error object. With no
// hint the error is about the task file tp now resolved, so the default
// task-file advice is the way out.
func exitNow(msg, hint string) {
	if hint == "" {
		// task-file-hint: an unreadable task file or its state, which tp use / tp init address
		output.Error(ExitFile, msg)
	} else {
		output.Error(ExitFile, msg, hint)
	}
	os.Exit(ExitFile)
}

// emptyNow is the payload of a project at root with nothing in it yet.
func emptyNow(root string) *nowResult {
	return &nowResult{
		Project:  filepath.Base(root),
		NeedsYou: []nowNeed{},
		Held:     []nowHeld{},
		Landing:  []nowLand{},
		Ready:    []nowTask{},
	}
}

// buildNow assembles the payload of the task file at taskFilePath.
func buildNow(taskFilePath string, tf *model.TaskFile) (*nowResult, error) {
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	res := emptyNow(root)
	rel := taskFileUnderRoot(root, taskFilePath)
	res.TaskFile = &rel

	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	resume, err := engine.AssembleResumeLive(resumeStart(taskFilePath), taskFilePath, specPath, tf)
	if err != nil {
		return nil, err
	}
	res.Phase = resume.Phase
	res.NextAction = resume.NextAction.Summary
	res.Tasks = nowTaskCounts(tf)
	res.Ready = nowReady(tf)
	res.Held = nowHeldTasks(tf)
	return res, nil
}

// nowReadyLimit is how many ready tasks tp now names.
const nowReadyLimit = 5

// nowTaskCounts is tp status's counts of tf.
func nowTaskCounts(tf *model.TaskFile) *nowCounts {
	c := countTasks(tf)
	return &nowCounts{Total: c.Total, Done: c.Done, WIP: c.WIP, Open: c.Open, Ready: c.Ready, Blocked: c.Blocked, Held: c.Held}
}

// nowReady names the first ready tasks of tf in tp ready's order.
func nowReady(tf *model.TaskFile) []nowTask {
	out := []nowTask{}
	for _, t := range engine.ReadyTasks(tf) {
		if len(out) == nowReadyLimit {
			break
		}
		out = append(out, nowTask{ID: t.ID, Title: t.Title})
	}
	return out
}

// nowHeldTasks lists tf's wip tasks in file order, each with who holds it.
func nowHeldTasks(tf *model.TaskFile) []nowHeld {
	out := []nowHeld{}
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusWIP {
			continue
		}
		out = append(out, nowHeld{ID: t.ID, Title: t.Title, Holder: "you", Since: rfc3339(t.StartedAt)})
	}
	return out
}

// rfc3339 formats at in RFC 3339, or nil when it is unset.
func rfc3339(at *time.Time) *string {
	if at == nil {
		return nil
	}
	s := at.UTC().Format(time.RFC3339)
	return &s
}
