package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// unitBranchPrefix names the branches a batch's worktrees are on. A run that
// finds one at start left it behind in a crash (spec/backlog/parallel-implement.md §2.6).
const unitBranchPrefix = "tp-unit/"

// BatchCheckoutNotice is what tp run --dry-run says before an implement
// batch: its units start from a fresh checkout (spec/backlog/parallel-implement.md §3).
const BatchCheckoutNotice = "an implement batch runs each unit in a fresh worktree of HEAD, without ignored files (dependencies, .env, build output); if the units need them, keep run_max_parallel at 1"

// CloseFunc closes a task on the main branch once its commits are picked
// there: it runs the quality gate and records the close, as tp done does, and
// returns an error when the gate fails or the close is refused. logPath is
// where its output goes.
type CloseFunc func(id, reason string, commits []string, logPath string) error

// CloseRefusedError is a close the driver's own tp done refused as a usage
// error: tp calling itself wrong, which stops the run as a driver error
// rather than discarding the unit's work.
type CloseRefusedError struct {
	ID, LogPath string
}

func (e *CloseRefusedError) Error() string {
	return fmt.Sprintf("tp done %s refused its own arguments (log: %s)", e.ID, e.LogPath)
}

// batchUnit is one unit of an implement batch: its worktree and branch.
type batchUnit struct {
	id, worktree, branch string
}

// startBatch claims the batch's tasks and adds a worktree per unit, outside
// the repository, on a branch from the main branch's HEAD (§2.3).
func (d *driver) startBatch(units []NextUnit) (map[string]batchUnit, error) {
	ids := make([]string, len(units))
	for i := range units {
		ids[i] = units[i].ID
	}
	if d.opts.ClaimTasks == nil {
		return nil, driverErrorf(fmt.Errorf("no claim function"), "claim the implement batch")
	}
	if err := d.opts.ClaimTasks(ids); err != nil {
		return nil, driverErrorf(err, "claim the implement batch")
	}
	base := WorktreeBase(d.opts.Root)
	out := make(map[string]batchUnit, len(ids))
	for _, id := range ids {
		u := batchUnit{
			id:       id,
			worktree: filepath.Join(base, d.runID, id),
			branch:   unitBranchPrefix + d.runID + "/" + id,
		}
		if _, err := gitOut(d.opts.Root, "worktree", "add", "-q", "-b", u.branch, u.worktree, "HEAD"); err != nil {
			return nil, driverErrorf(err, "add the worktree for task %q", id)
		}
		out[id] = u
	}
	return out, nil
}

// prompt is the instruction this attempt's child is spawned with: a unit in a
// worktree runs the brief that names its task, any other its kind's brief.
func (a *unitAttempt) prompt() string {
	if a.dir != "" {
		return briefPrompt(a.unit.Kind, BatchBriefCommand(a.unit.ID))
	}
	return UnitPrompt(a.unit.Kind, a.target)
}

// WorktreeBase is where a project's unit worktrees live: under the system's
// temporary directory, keyed by the project root, so no tool run in the
// repository ever walks into one.
func WorktreeBase(root string) string {
	sum := sha256.Sum256([]byte(CanonicalPath(root)))
	return filepath.Join(os.TempDir(), "tp-worktrees", hex.EncodeToString(sum[:])[:16])
}

// integrateBatch integrates every pending close of the cycle, in task order,
// then removes the batch's worktrees and branches (§2.4). A close the pick or
// the gate refuses restores the main branch, reopens its task and marks it
// solo; that is the batch's answer, not a driver error.
func (d *driver) integrateBatch(units map[string]batchUnit) error {
	if err := d.integratePending(); err != nil {
		return err
	}
	for _, u := range sortedBatch(units) {
		removeUnitWorktree(d.opts.Root, u.worktree, u.branch)
	}
	return nil
}

// integratePending integrates the cycle's pending closes one at a time.
func (d *driver) integratePending() error {
	pending, err := ReadPendingCloses(d.opts.Root, d.opts.TaskFile)
	if err != nil {
		return driverErrorf(err, "read the pending closes")
	}
	tf, err := model.ReadTaskFile(d.opts.TaskFile)
	if err != nil {
		return driverErrorf(err, "read the task file")
	}
	for i := range pending {
		if err := d.integrateOne(tf, &pending[i]); err != nil {
			return err
		}
	}
	return nil
}

// integrateOne picks one unit's commits onto the main branch, gates and closes
// the task there, and on a refused pick or a failed gate restores the main
// branch and reopens the task to run alone.
func (d *driver) integrateOne(tf *model.TaskFile, p *PendingClose) error {
	// A driver that died after the close but before removing the request
	// leaves both; the task is done, so the request is only dropped.
	if task, _, err := model.FindTask(tf, p.ID); err == nil && task.Status == model.StatusDone {
		if rmErr := RemovePendingClose(d.opts.Root, d.opts.TaskFile, p.ID); rmErr != nil {
			return driverErrorf(rmErr, "remove the pending close of %q", p.ID)
		}
		return nil
	}
	pre, err := gitOut(d.opts.Root, "rev-parse", "HEAD")
	if err != nil {
		return driverErrorf(err, "read the main branch's HEAD")
	}
	logPath := filepath.Join(RunDir(d.opts.Root, d.runID), "integrate-"+p.ID+".log")
	picked, pickErr := cherryPick(d.opts.Root, p.Commits)
	if pickErr == nil {
		warnTouchesEscape(d.opts.Root, tf, p.ID, pre)
		pickErr = d.closeTask(p, picked, logPath)
	}
	var refused *CloseRefusedError
	if errors.As(pickErr, &refused) {
		_, _ = gitOut(d.opts.Root, "reset", "-q", "--keep", pre)
		return driverErrorf(pickErr, "integrate task %q", p.ID)
	}
	if pickErr != nil {
		// --keep, not --hard: the task file's uncommitted closes of the
		// batch's earlier units must survive, and a unit's commits never
		// touch it (CheckUnitCommits).
		if _, resetErr := gitOut(d.opts.Root, "reset", "-q", "--keep", pre); resetErr != nil {
			return driverErrorf(resetErr, "restore the main branch after task %q", p.ID)
		}
		output.Notice(fmt.Sprintf("task %s did not integrate (%v); it reopens to run alone", p.ID, pickErr))
		if d.opts.ReopenSolo == nil {
			return driverErrorf(fmt.Errorf("no reopen function"), "reopen task %q", p.ID)
		}
		if err := d.opts.ReopenSolo(p.ID); err != nil {
			return driverErrorf(err, "reopen task %q", p.ID)
		}
	}
	if err := RemovePendingClose(d.opts.Root, d.opts.TaskFile, p.ID); err != nil {
		return driverErrorf(err, "remove the pending close of %q", p.ID)
	}
	return nil
}

func (d *driver) closeTask(p *PendingClose, picked []string, logPath string) error {
	if d.opts.CloseTask == nil {
		return fmt.Errorf("no close function: the driver cannot gate an integration")
	}
	return d.opts.CloseTask(p.ID, p.Reason, picked, logPath)
}

// cherryPick picks commits onto the current branch in the order given and
// returns the new shas; a conflict aborts the pick.
func cherryPick(root string, commits []string) ([]string, error) {
	picked := make([]string, 0, len(commits))
	for _, sha := range commits {
		if !SafeGitRev(sha) {
			return nil, fmt.Errorf("commit %q is not a revision", sha)
		}
		if _, err := gitOut(root, "cherry-pick", "--allow-empty", sha); err != nil {
			_, _ = gitOut(root, "cherry-pick", "--abort")
			return nil, fmt.Errorf("cherry-pick %s: %w", sha, err)
		}
		head, err := gitOut(root, "rev-parse", "HEAD")
		if err != nil {
			return nil, err
		}
		picked = append(picked, head)
	}
	return picked, nil
}

// warnTouchesEscape says which picked files lie outside the task's touches.
// It warns and records nothing else: the pick and the gate are the safety,
// touches only the schedule (§2.4).
func warnTouchesEscape(root string, tf *model.TaskFile, id, pre string) {
	task, _, err := model.FindTask(tf, id)
	if err != nil || len(task.Touches) == 0 {
		return
	}
	// --relative: touches are relative to the project root, which in a
	// monorepo sits below the git top level.
	files, err := gitOut(root, "diff", "--name-only", "--relative", pre, "HEAD")
	if err != nil {
		return
	}
	var outside []string
	for _, f := range strings.Split(files, "\n") {
		if f != "" && !TouchesMatch(task.Touches, f) {
			outside = append(outside, f)
		}
	}
	if len(outside) > 0 {
		output.Notice(fmt.Sprintf("task %s changed files outside its touches: %s", id, strings.Join(outside, ", ")))
	}
}

func removeUnitWorktree(root, worktree, branch string) {
	_, _ = gitOut(root, "worktree", "remove", "--force", worktree)
	_, _ = gitOut(root, "branch", "-D", branch)
}

func sortedBatch(units map[string]batchUnit) []batchUnit {
	out := make([]batchUnit, 0, len(units))
	for _, u := range units {
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b batchUnit) int { return strings.Compare(a.id, b.id) })
	return out
}

// reconcileBatches finishes what a crashed run left (§2.6): its pending closes
// are integrated, then every unit worktree and branch is removed. A task whose
// unit never asked to close stays wip, and the oracle resumes it alone.
func (d *driver) reconcileBatches() error {
	if err := d.integratePending(); err != nil {
		return err
	}
	list, err := gitOut(d.opts.Root, "worktree", "list", "--porcelain")
	if err != nil {
		return nil //nolint:nilerr // not a git tree: nothing a batch could have left
	}
	var worktree string
	for _, line := range strings.Split(list, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			worktree = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch refs/heads/"+unitBranchPrefix):
			removeUnitWorktree(d.opts.Root, worktree, strings.TrimPrefix(line, "branch refs/heads/"))
		}
	}
	_, _ = gitOut(d.opts.Root, "worktree", "prune")
	return nil
}
