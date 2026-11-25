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
	// Code names what the need is about in a word that does not change with
	// the text, so a reader can word it in its own language: a run's stop
	// reason or crashed, a decision's name, a blocker's code, a cap's
	// workflow field, land-request.
	Code string `json:"code,omitempty"`
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
		res.Epics = nowEpics(".")
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
	run, runHeld, escalations := nowRunOf(root, taskFilePath)
	res.Run = run
	res.Held = nowHeldTasks(tf, runHeld)
	res.Landing = nowLandings(taskFilePath, tf)
	res.NeedsYou = append(blockerNeeds(resume.Blockers), escalations...)
	for _, l := range res.Landing {
		res.NeedsYou = append(res.NeedsYou, landNeed(l.ID))
	}
	res.Epics = nowEpics(root)
	return res, nil
}

// nowLandings lists the land requests waiting for tp land, in id order: the
// ones tp land would land, made under the claim of a task still wip.
func nowLandings(taskFilePath string, tf *model.TaskFile) []nowLand {
	out := []nowLand{}
	requests, unreadable, err := engine.ReadLandRequests(engine.LandDir(taskFilePath))
	if err != nil {
		output.Notice("warning: the land requests cannot be read: " + err.Error())
		return out
	}
	for _, path := range unreadable {
		output.Notice("warning: the land request " + path + " cannot be read")
	}
	for i := range requests {
		r := &requests[i]
		task, _, findErr := model.FindTask(tf, r.ID)
		if findErr != nil || task.Status != model.StatusWIP || !r.MadeUnder(task.ClaimedBy) {
			continue
		}
		land := nowLand{ID: r.ID, Title: task.Title}
		if r.At != "" {
			land.Since = &r.At
		}
		out = append(out, land)
	}
	return out
}

// nowRunOf is the current or last run of the task file as tp run --status
// reports it, the tasks it holds now (runHeldUnits), and what it waits on
// a person for (escalationNeeds, runNeeds); nil and none when no run was ever
// driven over it. A run state
// that cannot be read is said on stderr and reported as no run, so the rest
// of the payload still reaches the pane.
func nowRunOf(root, taskFilePath string) (run map[string]any, held map[string]*engine.RunUnitRow, escalations []nowNeed) {
	st, err := engine.ReadRunState(root, taskFilePath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			output.Notice("warning: the run state cannot be read: " + err.Error())
		}
		return nil, nil, nil
	}
	wf := engine.EffectiveWorkflowForTaskFile(taskFilePath)
	run = runStatusPayload(st, taskFilePath, &wf)
	run["last_finished"] = lastFinishedUnit(st)
	slots, _ := run["running"].([]runningSlot)
	needs := escalationNeeds(root, st)
	runState, _ := run["run_state"].(string)
	needs = append(needs, runNeeds(st, runState, needs)...)
	return run, runHeldUnits(st, slots, root, taskFilePath), needs
}

// runHeldUnits maps each task the run holds now to its latest implement
// row, nil when it has none: the implement slots an in-flight run is running,
// and every pool member engine.LiveRunningUnits names, which is the whole pool
// of an in-flight run (a member stays pooled through its integration and the
// main gate after its unit exits) and a dead driver's units still running.
func runHeldUnits(st *engine.RunState, slots []runningSlot, root, taskFilePath string) map[string]*engine.RunUnitRow {
	ids := engine.LiveRunningUnits(root, taskFilePath)
	for i := range slots {
		if slots[i].Kind == engine.UnitImplement {
			ids = append(ids, slots[i].ID)
		}
	}
	held := make(map[string]*engine.RunUnitRow, len(ids))
	for _, id := range ids {
		held[id] = latestImplementRow(st, id)
	}
	return held
}

// latestImplementRow is st's last implement row for task id, or nil.
func latestImplementRow(st *engine.RunState, id string) *engine.RunUnitRow {
	for i := len(st.Units) - 1; i >= 0; i-- {
		if u := &st.Units[i]; u.Kind == engine.UnitImplement && u.ID == id {
			return u
		}
	}
	return nil
}

// nowFinished is the latest unit attempt that exited.
type nowFinished struct {
	Kind            engine.UnitKind `json:"kind"`
	ID              string          `json:"id"`
	Attempt         int             `json:"attempt"`
	ExitCode        int             `json:"exit_code"`
	DurationSeconds *float64        `json:"duration_seconds"`
}

// lastFinishedUnit is st's exited attempt that ended last, or nil when
// none has exited: the row tp run --status's last_unit reports may still be
// running (spec/1.32.0-measurements.md §4).
func lastFinishedUnit(st *engine.RunState) *nowFinished {
	var best *engine.RunUnitRow
	for i := range st.Units {
		if u := &st.Units[i]; u.ExitCode != nil && (best == nil || finishedAfter(u, best)) {
			best = u
		}
	}
	if best == nil {
		return nil
	}
	return &nowFinished{Kind: best.Kind, ID: best.ID, Attempt: best.Attempt, ExitCode: *best.ExitCode, DurationSeconds: best.DurationSeconds}
}

// finishedAfter reports whether exited row a ended after exited row b: by
// started_at plus duration_seconds when both rows carry them, which orders
// parallel units by when they ended; else, and within a second, by seq.
func finishedAfter(a, b *engine.RunUnitRow) bool {
	aEnd, aOK := unitEnd(a)
	bEnd, bOK := unitEnd(b)
	if aOK && bOK {
		// started_at is stamped to the second, so an end is known only to
		// within one: a nearer pair keeps seq order.
		switch {
		case aEnd.Sub(bEnd) >= time.Second:
			return true
		case bEnd.Sub(aEnd) >= time.Second:
			return false
		}
	}
	return a.Seq > b.Seq
}

// unitEnd is when row u's attempt ended, and whether its row says.
func unitEnd(u *engine.RunUnitRow) (time.Time, bool) {
	if u.DurationSeconds == nil {
		return time.Time{}, false
	}
	start, err := time.Parse(time.RFC3339, u.StartedAt)
	if err != nil {
		return time.Time{}, false
	}
	return start.Add(time.Duration(*u.DurationSeconds * float64(time.Second))), true
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

// nowHeldTasks lists tf's wip tasks in file order, each with who holds it:
// the run when runHeld names it, with the worktree and attempt of its latest
// row; an agent when a spawn's claim holds it; else you.
func nowHeldTasks(tf *model.TaskFile, runHeld map[string]*engine.RunUnitRow) []nowHeld {
	out := []nowHeld{}
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusWIP {
			continue
		}
		row := nowHeld{ID: t.ID, Title: t.Title, Holder: "you", Since: rfc3339(t.StartedAt)}
		if unit, byRun := runHeld[t.ID]; byRun {
			row.Holder = "run"
			if unit != nil {
				row.Attempt = &unit.Attempt
				if unit.Worktree != "" {
					row.Worktree = &unit.Worktree
				}
				if row.Since == nil && unit.StartedAt != "" {
					row.Since = &unit.StartedAt
				}
			}
		} else if t.ClaimedBy != nil && t.ClaimedBy.Spawn != "" {
			row.Holder = "agent"
		}
		out = append(out, row)
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
