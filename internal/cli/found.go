package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// foundShapeHint names every field a request can carry, so a refused unit can
// rebuild the call without reading the help.
const foundShapeHint = "file one defect: --location <file:line> --reproduction <command or test>, then either --expected and --observed, or --question with an --alternative per choice"

// foundBlocksHint says who may set --blocks and how a caller names its task.
const foundBlocksHint = "--blocks marks a request the caller's own task cannot pass without, so it needs a wip task the caller works: an implement unit's own, or --task <id>; drop --blocks to file the request without it"

// foundStoreWriteHint answers a store that could not be written: the request
// lives under the project's .tp/, beside the cycle's run state.
const foundStoreWriteHint = "check that the project's .tp/ directory exists and is writable, then file the request again"

// foundAddFlags are tp found add's inputs: the request's fields as flags.
type foundAddFlags struct {
	location, reproduction, expected, observed, question, task string
	alternatives                                               []string
	blocks                                                     bool
}

func newFoundCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "found",
		Short: "File requests for defects found outside the caller's task",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newFoundAddCmd(), newFoundApplyCmd(), newFoundAnswerCmd())
	return cmd
}

func newFoundApplyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply the spec's recorded requests: open found tasks within found_max",
		Long: `Apply every recorded, unapplied request of the spec under the task-file lock.

A duplicate (same file and reproduction as a request still open) opens
nothing. A request whose file lies inside the touches of an open task (not
its finder) opens nothing, does not count against found_max, and is
attached to that task as a note shown by tp show and tp brief. A found task's
origin location is not one of its files. Any other Fix-shaped request becomes
a found task, blocking ones first, then by arrival, while the task file holds
fewer than found_max found tasks (0 is uncapped); past it the request becomes
an open queue entry. A request that needs a choice waits, undecided, for the
decision unit.

A non-blocking found task depends on the task that filed it (an audit row's
on nothing). A blocking request makes its finder depend on the task it ended
in; when that edge would close a cycle it adds no edge and becomes an open
queue entry naming the cycle.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFoundApply()
		},
	}
}

// runFoundApply implements tp found apply (spec/1.31.0.md §2, §8). Only the
// outcomes it produced are printed; an empty batch prints the counts alone.
func runFoundApply() error {
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file resolved, so there are no requests to apply.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	foundMax := engine.EffectiveWorkflowForTaskFile(taskFilePath).FoundMax
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	plan, err := applyFoundRequests(root, taskFilePath, foundMax)
	if err != nil {
		output.Error(ExitFile, "cannot apply the requests: "+err.Error(), foundStoreWriteHint)
		os.Exit(ExitFile)
		return nil
	}
	return output.JSON(foundPlanOutput(&plan, foundMax))
}

// foundPlanOutput is what tp found apply and tp found answer print of a plan:
// the counts, and only the outcomes the plan produced.
func foundPlanOutput(plan *engine.FoundApplyPlan, foundMax int) map[string]any {
	out := map[string]any{"found_tasks": plan.FoundTasks, "found_max": foundMax}
	if len(plan.Opened) > 0 {
		out["opened"] = plan.Opened
	}
	if len(plan.Attached) > 0 {
		out["attached"] = plan.Attached
	}
	if len(plan.Entries) > 0 {
		queued := make([]map[string]string, 0, len(plan.Entries))
		for i := range plan.Entries {
			queued = append(queued, map[string]string{"request": plan.Entries[i].Request, "entry": plan.Entries[i].ID})
		}
		out["queued"] = queued
	}
	if len(plan.Undecided) > 0 {
		out["undecided"] = plan.Undecided
	}
	if len(plan.Reopened) > 0 {
		out["reopened"] = plan.Reopened
	}
	if len(plan.Duplicates) > 0 {
		dups := make([]map[string]string, 0, len(plan.Duplicates))
		for _, d := range plan.Duplicates {
			dups = append(dups, map[string]string{"request": d.Request, "kept": d.KeptRequest})
		}
		out["duplicates"] = dups
	}
	return out
}

func newFoundAddCmd() *cobra.Command {
	var f foundAddFlags
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Record one request for a defect outside the caller's task",
		Long: `Record one request, one defect, in the spec's run state. No task changes.

A request carries a location (file:line) and a reproduction (a command or test
that shows the defect), and either all four Fix fields (--expected and
--observed too) or a --question with its --alternative choices. Anything else
exits 2 and records nothing.

--blocks marks a request the caller's task cannot pass without. It needs a wip
task the caller works: under tp run an implement unit's own task, or the one
--task names.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFoundAdd(&f)
		},
	}
	cmd.Flags().StringVar(&f.location, "location", "", "where the defect is, as file:line")
	cmd.Flags().StringVar(&f.reproduction, "reproduction", "", "a command or test that shows the defect")
	cmd.Flags().StringVar(&f.expected, "expected", "", "the expected behaviour")
	cmd.Flags().StringVar(&f.observed, "observed", "", "the observed behaviour")
	cmd.Flags().StringVar(&f.question, "question", "", "the choice the request needs, when it is not Fix-shaped")
	// StringArray, not StringSlice: an alternative is prose, and splitting it
	// on commas would turn one alternative into several.
	cmd.Flags().StringArrayVar(&f.alternatives, "alternative", nil, "one way to answer --question (repeatable)")
	cmd.Flags().BoolVar(&f.blocks, "blocks", false, "the caller's task cannot pass without this request")
	cmd.Flags().StringVar(&f.task, "task", "", "the wip task the caller works (an implement unit's own by default)")
	return cmd
}

// runFoundAdd implements tp found add (spec/1.31.0.md §1). It reads the task
// file only to name the finder; the request goes to the cycle's found store.
func runFoundAdd(f *foundAddFlags) error {
	req := &engine.FoundRequest{
		Location:     f.location,
		Reproduction: f.reproduction,
		Expected:     f.expected,
		Observed:     f.observed,
		Question:     f.question,
		Alternatives: f.alternatives,
		Blocks:       f.blocks,
	}
	if msg := engine.FoundRequestShapeError(req); msg != "" {
		return foundRefusal(msg, foundShapeHint)
	}

	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file resolved, so the request has no cycle.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	finder, err := foundFinderTask(taskFilePath, f.task)
	if err != nil {
		return foundRefusal(err.Error(), foundBlocksHint)
	}
	if f.blocks && finder == "" {
		return foundRefusal("--blocks needs a task the caller works, and none is named", foundBlocksHint)
	}
	req.Finder.Task = finder

	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	stored, err := engine.AddFoundRequest(root, taskFilePath, req)
	if err != nil {
		output.Error(ExitFile, "cannot record the request: "+err.Error(), foundStoreWriteHint)
		os.Exit(ExitFile)
		return nil
	}
	return output.JSON(map[string]any{
		"id":     stored.ID,
		"state":  stored.State,
		"blocks": stored.Blocks,
		"finder": stored.Finder,
	})
}

// applyFoundRequests applies a spec's requests under the task-file lock
// (spec/1.31.0.md §2): it plans over the requests read under the store's
// lock, so a request filed meanwhile is either in this batch or left
// unapplied for the next, then writes the task file, the queue and the
// store. It returns the plan with its queue entries numbered.
func applyFoundRequests(root, taskFilePath string, foundMax int) (engine.FoundApplyPlan, error) {
	var plan engine.FoundApplyPlan
	running := engine.FoundRunningUnits(root, taskFilePath, implementUnitTask())
	err := engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			return err
		}
		queue, err := engine.ReadFoundQueue(taskFilePath)
		if err != nil {
			return err
		}
		return engine.UpdateFoundRequests(root, taskFilePath, func(reqs []engine.FoundRequest) ([]engine.FoundRequest, error) {
			plan = engine.PlanFoundApply(reqs, tf.Tasks, queue, foundMax)
			reopen := func(tasks []model.Task, entries []engine.FoundQueueEntry) []string {
				return engine.FoundFindersToReopen(reqs, tasks, append(slices.Clone(queue), entries...), running)
			}
			return plan.Requests, writeFoundPlan(taskFilePath, tf, queue, &plan, reopen)
		})
	})
	return plan, err
}

// writeFoundPlan writes a plan's tasks, notes and edges to the task file and
// its entries to the queue, numbering the plan's entries as written. When
// reopen is not nil it names, over the placed tasks and the plan's new
// entries, the waiting finders to return to open (plan.Reopened). The caller
// holds the task-file lock.
func writeFoundPlan(taskFilePath string, tf *model.TaskFile, queue []engine.FoundQueueEntry, plan *engine.FoundApplyPlan, reopen func([]model.Task, []engine.FoundQueueEntry) []string) error {
	attachFoundNotes(tf, plan.Attached)
	addFoundEdges(tf, plan.Edges)
	tf.Tasks = append(tf.Tasks, plan.Tasks...)
	if reopen != nil {
		plan.Reopened = reopen(tf.Tasks, plan.Entries)
		reopenFoundFinders(tf, plan.Reopened)
	}
	if len(plan.Tasks) > 0 || len(plan.Attached) > 0 || len(plan.Edges) > 0 || len(plan.Reopened) > 0 {
		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			return err
		}
	}
	if len(plan.Entries) > 0 {
		queue = append(queue, plan.Entries...)
		if err := engine.WriteFoundQueue(taskFilePath, queue); err != nil {
			return err
		}
		plan.Entries = queue[len(queue)-len(plan.Entries):]
	}
	return nil
}

// attachFoundNotes adds each attached request's note to its task.
func attachFoundNotes(tf *model.TaskFile, attached []engine.FoundAttached) {
	for j := range attached {
		a := &attached[j]
		for i := range tf.Tasks {
			if tf.Tasks[i].ID == a.Task {
				tf.Tasks[i].FoundNotes = append(tf.Tasks[i].FoundNotes, a.Note)
			}
		}
	}
}

// reopenFoundFinders returns each waiting finder to open (spec/1.31.0.md
// §4), releasing its claim the way a run's release does. The attempt is
// not counted against the task's retries.
func reopenFoundFinders(tf *model.TaskFile, ids []string) {
	for _, id := range ids {
		task, _, err := model.FindTask(tf, id)
		if err != nil {
			continue
		}
		task.Status = model.StatusOpen
		task.Waiting = ""
		task.StartedAt = nil
		task.DurationSource = ""
		task.ClaimedBy = task.ClaimedBy.Released()
	}
}

// addFoundEdges adds each placement edge to its task's depends_on.
func addFoundEdges(tf *model.TaskFile, edges []engine.FoundEdge) {
	for _, e := range edges {
		for i := range tf.Tasks {
			if tf.Tasks[i].ID == e.Task {
				tf.Tasks[i].DependsOn = append(tf.Tasks[i].DependsOn, e.DependsOn)
			}
		}
	}
}

// foundFinderTask returns the task the caller works, or "" when it names none:
// --task, or an implement unit's own task under tp run. A role unit's
// TP_UNIT_ID is a role, not a task, so only the implement kind counts. The task
// must be wip in the task file, which is read and never written.
func foundFinderTask(taskFilePath, flagTask string) (string, error) {
	unitTask := implementUnitTask()
	if flagTask != "" && unitTask != "" && flagTask != unitTask {
		return "", fmt.Errorf("--task names %s, but this unit works %s", flagTask, unitTask)
	}
	id := flagTask
	if id == "" {
		id = unitTask
	}
	if id == "" {
		return "", nil
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		// task-file-hint: the task file that names the finder is unreadable.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	for i := range tf.Tasks {
		if tf.Tasks[i].ID != id {
			continue
		}
		if tf.Tasks[i].Status != model.StatusWIP {
			return "", fmt.Errorf("task %s is %s, not a task the caller works", id, tf.Tasks[i].Status)
		}
		return id, nil
	}
	return "", fmt.Errorf("task %s is not in the task file", id)
}

// implementUnitTask is the task an implement unit of tp run works, from its
// environment, or "" outside one: a role unit's TP_UNIT_ID is a role.
func implementUnitTask() string {
	if engine.UnitKind(os.Getenv(engine.EnvUnitKind)) == engine.UnitImplement {
		return os.Getenv(engine.EnvUnitID)
	}
	return ""
}

// foundRefusal emits a refusal and exits 2; see escalateRefusal for why it
// returns at all.
func foundRefusal(msg, hint string) error {
	output.Error(ExitUsage, msg, hint)
	os.Exit(ExitUsage)
	return nil
}
