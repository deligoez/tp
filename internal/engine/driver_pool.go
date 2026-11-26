package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// pool is the rolling implement pool in flight
// (spec/backlog/rolling-implement.md §2). The loop's goroutine owns the main
// branch and is the only one that claims, adds worktrees, integrates, records
// and spawns; each child runs in a goroutine of its own and posts its exit on
// exits. One goroutine doing every main-branch step is the serial rule: no unit
// starts while another is integrated, and no two gates run on the main tree.
type pool struct {
	setup string
	// res is the cycle read last, which a spawn's phase and round come from.
	res     *ResumeResult
	running map[string]*poolSlot
	exits   chan *unitAttempt
	asked   []*unitAttempt
	out     iterationOutcome
	err     error
	// stopping: a stop cause was seen, so nothing more is spawned, retries
	// included, and the pool drains.
	stopping bool
	// halted: an integration failed as a driver error, so the main branch is
	// in a state the driver did not choose and nothing more is picked onto
	// it; the remaining requests wait for the next run.
	halted bool
}

// poolSlot is one running unit: its worktree and the attempts it has made.
type poolSlot struct {
	tree     unitTree
	unit     NextUnit
	attempts int
}

// poolHook is called at the points a crash test kills the driver at; it does
// nothing outside those tests.
var poolHook = func(point, id string) {}

// runPool runs implement units in a rolling pool: each unit in its own
// worktree, integrated on the main branch as soon as it exits, its slot then
// refilled from the oracle with the running units named. A stop cause, an
// escalation, a spent unit or an error, drains the pool instead: nothing new
// is spawned, retries included, and every child already running is waited for
// and integrated. The pool ends when it is empty, and the loop's checkpoint
// reads what it ended as.
func (d *driver) runPool(res *ResumeResult, units []NextUnit) (iterationOutcome, error) {
	if d.opts.ClaimTasks == nil {
		return iterationOutcome{}, driverErrorf(errors.New("no claim function"), "claim an implement unit")
	}
	setup, err := worktreeSetupCmd(d.opts.Root)
	if err != nil {
		return iterationOutcome{}, err
	}
	p := &pool{
		setup:   setup,
		res:     res,
		running: make(map[string]*poolSlot, len(units)),
		// Room for every child to post without a reader, so a driver that
		// stops reading never leaves one blocked.
		exits: make(chan *unitAttempt, len(units)+max(d.opts.Workflow.RunMaxParallel, 1)),
		out:   iterationOutcome{finished: true},
	}
	d.trees = make(map[string]unitTree, len(units))
	defer func() { d.trees = nil }()
	d.fill(p, units)
	for len(p.running) > 0 {
		d.exited(p, <-p.exits)
		// Every child that exited meanwhile is taken first, so one re-read
		// of the cycle fills every slot freed.
		for drained := false; !drained; {
			select {
			case a := <-p.exits:
				d.exited(p, a)
			default:
				drained = true
			}
		}
		d.refill(p)
	}
	p.out.escalation = lowestEscalation(p.asked)
	return p.out, p.err
}

// fail keeps the pool's first error and drains it. A driver error outranks
// one that is not (a worktree setup's failure), and joins it, so the stop is
// driver-error and the setup is still named.
func (p *pool) fail(err error) {
	var driverErr *DriverError
	switch {
	case p.err == nil:
		p.err = err
	case !errors.As(p.err, &driverErr) && errors.As(err, &driverErr):
		p.err = errors.Join(err, p.err)
	}
	p.stopping = true
}

// runningIDs names the units the pool runs now, for the oracle.
func (p *pool) runningIDs() []string {
	ids := make([]string, 0, len(p.running))
	for id := range p.running {
		ids = append(ids, id)
	}
	return ids
}

// refill reads the cycle with the running units named and starts what the
// oracle returns for the free slots. A pool already empty ends instead: what
// comes next, a unit resumed or run alone or a new pool, is the loop's.
// Anything but implement units, a task audit due, starts nothing, so the pool
// drains for it.
func (d *driver) refill(p *pool) {
	if p.stopping || len(p.running) == 0 {
		return
	}
	res, err := readCycleRunning(d.opts, p.runningIDs())
	if err != nil {
		p.fail(err)
		return
	}
	p.res = &res
	d.fill(p, refillUnits(res.NextUnits, p.running))
}

// refillUnits is what a refill starts of the oracle's units: the implement
// units the pool does not run already, or none at all when any unit is of
// another kind, since that unit waits for the pool to drain.
func refillUnits(next []NextUnit, running map[string]*poolSlot) []NextUnit {
	units := make([]NextUnit, 0, len(next))
	for i := range next {
		u := next[i]
		if u.Kind != UnitImplement {
			return nil
		}
		if running[u.ID] == nil {
			units = append(units, u)
		}
	}
	return units
}

// fill starts each unit in turn while nothing stops the pool, the caps and
// the operator's signal read before every spawn.
func (d *driver) fill(p *pool, units []NextUnit) {
	for i := range units {
		if !d.mayStart(p) {
			return
		}
		d.startUnit(p, units[i])
	}
}

// mayStart reports whether another child may be spawned: no stop cause seen,
// no signal, and every cap still has room (spec/backlog/rolling-implement.md
// §2: caps are read before every spawn, so run_max_units is never passed).
func (d *driver) mayStart(p *pool) bool {
	if p.stopping {
		return false
	}
	snapshot := d.rec.Snapshot()
	if d.sig.signalled() || capStop(&snapshot, &d.opts.Workflow) != "" {
		p.stopping = true
		return false
	}
	return true
}

// startUnit claims one task, adds its worktree on a branch from the main
// branch's HEAD, runs the project's worktree_setup_cmd there
// (spec/backlog/worktree-setup.md) and spawns its first attempt. A failed add
// or setup releases that one unit, with no attempt spent, and drains the pool.
func (d *driver) startUnit(p *pool, u NextUnit) {
	tree := unitTree{
		id:       u.ID,
		worktree: filepath.Join(WorktreeBase(d.opts.Root), cycleTag(d.opts.Root, d.opts.TaskFile), d.runID, u.ID),
		branch:   unitBranch(d.opts.Root, d.opts.TaskFile, d.runID, u.ID),
	}
	if err := d.rec.JoinPool(u.ID); err != nil {
		p.fail(driverErrorf(err, "record task %q in the pool", u.ID))
		return
	}
	if err := d.opts.ClaimTasks([]string{u.ID}); err != nil {
		p.fail(errors.Join(driverErrorf(err, "claim task %q", u.ID), d.leavePool(u.ID)))
		return
	}
	if err := d.setAsideUnitBranch(tree); err != nil {
		// Released with no branch named: the earlier branch is not this
		// unit's to delete.
		p.fail(errors.Join(driverErrorf(err, "set aside the earlier branch of task %q", u.ID), d.releaseUnit(unitTree{id: tree.id, worktree: tree.worktree})))
		return
	}
	if _, err := gitOut(d.opts.Root, "worktree", "add", "-q", "-b", tree.branch, tree.worktree, "HEAD"); err != nil {
		p.fail(errors.Join(driverErrorf(err, "add the worktree for task %q", u.ID), d.releaseUnit(tree)))
		return
	}
	poolHook("added", u.ID)
	if p.setup != "" {
		if err := d.setupWorktree(p.setup, tree); err != nil {
			if relErr := d.releaseUnit(tree); relErr != nil {
				err = errors.Join(relErr, err)
			}
			p.fail(err)
			return
		}
	}
	// A setup can outlast a cap or meet the signal: both are read again
	// right before the spawn, and a stop then releases the unit unspawned.
	if !d.mayStart(p) {
		if err := d.releaseUnit(tree); err != nil {
			p.fail(err)
		}
		return
	}
	d.trees[u.ID] = tree
	p.running[u.ID] = &poolSlot{tree: tree, unit: u}
	d.spawnSlot(p, u.ID)
}

// setAsideUnitBranch renames the branch an earlier unit of the same task left
// in this run, so the task's next unit starts a fresh branch from HEAD: a
// finder tp found apply returned to open (spec/1.31.0.md §4) runs again on a
// tree that holds the fix it waited for. The earlier unit's commits stay
// reachable under tp-unit/<cycle>/<run>-<n>/<id>, kept like any unfinished
// task's branch (keptUnitBranch), and a task recording the old name as its
// conflict_branch names the new one. With no such branch it does nothing.
func (d *driver) setAsideUnitBranch(tree unitTree) error {
	if !gitBranchExists(d.opts.Root, tree.branch) {
		return nil
	}
	for n := 1; ; n++ {
		aside := unitBranch(d.opts.Root, d.opts.TaskFile, d.runID+"-"+strconv.Itoa(n), tree.id)
		if !gitBranchExists(d.opts.Root, aside) {
			if _, err := gitOut(d.opts.Root, "branch", "-m", tree.branch, aside); err != nil {
				return err
			}
			if d.opts.RenameConflictBranch == nil {
				return nil
			}
			return d.opts.RenameConflictBranch(tree.branch, aside)
		}
	}
}

// gitBranchExists reports whether root has a local branch of that name.
func gitBranchExists(root, branch string) bool {
	_, err := gitOut(root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

// spawnSlot spawns the next attempt of a running unit, in its own worktree.
func (d *driver) spawnSlot(p *pool, id string) {
	slot := p.running[id]
	attempts, err := d.prepare(p.res, []NextUnit{slot.unit}, slot.attempts+1)
	if err != nil {
		p.fail(err)
		if slot.attempts == 0 {
			delete(p.running, id)
			delete(d.trees, id)
			if relErr := d.releaseUnit(slot.tree); relErr != nil {
				p.err = errors.Join(p.err, relErr)
			}
			return
		}
		d.endSlot(p, id)
		return
	}
	slot.attempts++
	// The child's process starts here, on the loop's goroutine (the serial
	// rule); only the wait runs beside the loop.
	a := attempts[0]
	if !a.start(d.opts.Root, os.Environ()) {
		p.exits <- a
		return
	}
	if err := d.rec.SetUnitPID(a.seq, a.cmd.Process.Pid); err != nil {
		p.fail(driverErrorf(err, "record the pid of task %q", id))
	}
	go func() {
		a.wait()
		p.exits <- a
	}()
}

// exited records one child's exit and answers it: a failed attempt is retried
// in the same worktree while its budget lasts and nothing stops the pool;
// otherwise the unit's close is integrated and its slot released.
func (d *driver) exited(p *pool, a *unitAttempt) {
	id := a.unit.ID
	if err := d.record([]*unitAttempt{a}); err != nil {
		p.fail(err)
		d.endSlot(p, id)
		return
	}
	// Applied with the other running units named: the one that just exited
	// is still a pool member, and its finder must be free to reopen.
	reopened, err := d.applyFound(slices.DeleteFunc(p.runningIDs(), func(r string) bool { return r == id }))
	if err != nil {
		p.fail(err)
		d.endSlot(p, id)
		return
	}
	switch {
	case a.escalated():
		p.asked = append(p.asked, a)
		p.stopping = true
	case a.succeeded(), reopened[id]:
		// A finder apply returned to open waits on its blocking request:
		// not retried, and the attempt is not counted (spec/1.31.0.md §4).
	case d.retry(p, a):
		return
	}
	if !p.halted {
		if err := d.integrateUnit(id); err != nil {
			p.halted = true
			p.fail(err)
		}
	}
	d.endSlot(p, id)
}

// retry spawns a failed unit's next attempt and reports whether it did. A
// spent budget stops the run with unit-failure and drains the pool; a retry
// the operator's signal withholds is interrupted, not failed. A retry another
// stop withholds, a cap or an escalation, leaves finished as it is, since that
// stop is what the run records.
func (d *driver) retry(p *pool, a *unitAttempt) bool {
	slot := p.running[a.unit.ID]
	switch {
	case slot.attempts >= attemptBudget(&d.opts.Workflow):
		p.out.finished = false
		p.stopping = true
		return false
	case d.sig.signalled():
		p.out.finished = false
		p.out.interrupted = true
		p.stopping = true
		return false
	case !d.mayStart(p):
		return false
	}
	d.spawnSlot(p, a.unit.ID)
	return true
}

// integrateUnit integrates one unit's pending close, when it left one.
func (d *driver) integrateUnit(id string) error {
	if !HasPendingClose(d.opts.Root, d.opts.TaskFile, id) {
		return nil
	}
	poolHook("integrating", id)
	defer poolHook("integrated", id)
	pending, err := ReadPendingCloses(d.opts.Root, d.opts.TaskFile)
	if err != nil {
		return driverErrorf(err, "read the pending closes")
	}
	tf, err := model.ReadTaskFile(d.opts.TaskFile)
	if err != nil {
		return driverErrorf(err, "read the task file")
	}
	for i := range pending {
		if pending[i].ID == id {
			return d.integrateOne(tf, &pending[i])
		}
	}
	return nil
}

// endSlot releases a finished unit's slot: its worktree is removed, and its
// branch too unless the branch must outlive it (keptUnitBranch).
func (d *driver) endSlot(p *pool, id string) {
	slot := p.running[id]
	delete(p.running, id)
	delete(d.trees, id)
	u := slot.tree
	if keptUnitBranch(d.opts.Root, d.opts.TaskFile, u.branch) {
		// The branch outlives the worktree, so only the worktree goes; one
		// still on disk afterwards is reported, and the next run's
		// reconcileUnits sweeps it as a registered unit worktree.
		_, _ = gitOut(d.opts.Root, "worktree", "remove", "--force", u.worktree)
		if _, err := os.Stat(u.worktree); err == nil {
			output.Notice(fmt.Sprintf("the worktree of task %s could not be removed, still on disk: %s", u.id, u.worktree))
		}
		if !HasPendingClose(d.opts.Root, d.opts.TaskFile, u.id) {
			output.Notice(fmt.Sprintf("task %s did not finish; its unit's work stays on branch %s until the task is done", u.id, u.branch))
		}
	} else {
		removeUnitWorktree(d.opts.Root, u.worktree, u.branch)
	}
	if err := d.leavePool(id); err != nil {
		p.fail(err)
	}
}

// leavePool takes an ended unit out of the run state's pool.
func (d *driver) leavePool(id string) error {
	if err := d.rec.LeavePool(id); err != nil {
		return driverErrorf(err, "take task %q out of the pool", id)
	}
	return nil
}

// releaseUnit takes back a unit that never spawned: its worktree and tp-unit
// branch are removed, a failed add may have left both, and its task goes back
// to open. No attempt was spent, so it is not marked solo. A worktree still on
// disk afterwards is reported rather than left for the next run to trip over.
func (d *driver) releaseUnit(u unitTree) error {
	removeUnitWorktree(d.opts.Root, u.worktree, u.branch)
	_, _ = gitOut(d.opts.Root, "worktree", "prune")
	var errs []error
	if _, err := os.Stat(u.worktree); err == nil {
		errs = append(errs, driverErrorf(fmt.Errorf("still on disk: %s", u.worktree), "remove the worktree of task %q", u.id))
	}
	if d.opts.ReleaseTasks == nil {
		errs = append(errs, driverErrorf(errors.New("no release function"), "release task %q", u.id))
	} else if err := d.opts.ReleaseTasks([]string{u.id}); err != nil {
		errs = append(errs, driverErrorf(err, "release task %q", u.id))
	}
	if err := d.leavePool(u.id); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// readCycleRunning is readCycle with the pool's running units named, so the
// oracle fills only the free slots.
func readCycleRunning(o *DriverOptions, running []string) (ResumeResult, error) {
	tf, err := model.ReadTaskFile(o.TaskFile)
	switch {
	case errors.Is(err, os.ErrNotExist):
		tf = &model.TaskFile{Spec: o.Spec, Tasks: []model.Task{}}
	case err != nil:
		return ResumeResult{}, driverErrorf(err, "read the task file")
	}
	return AssembleResumeRunning(o.Root, o.TaskFile, o.Spec, tf, running)
}
