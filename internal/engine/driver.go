package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// The §3.4 stop reasons are the StopReason vocabulary in stopreason.go; the
// loop names them and never composes one of its own.

// roleFindingsPartSuffix is what a role unit writes and the driver renames
// away on exit 0 (§3.3.1). The unit may only write the .part — §6.3's
// allowlist names that path — so the final name existing is proof the driver,
// not the unit, decided the unit finished.
const roleFindingsPartSuffix = ".part"

// DriverOptions is everything §3.1's loop needs to drive one cycle: the
// resolved cycle itself and the effective workflow whose caps bound the run and
// whose runner field says what to spawn.
//
// The task file and spec are resolved by the caller, the same way tp resume
// resolves them, so the driver never runs discovery of its own and a child
// spawned with TP_FILE works the target its driver worked.
type DriverOptions struct {
	// Root is the repository root. Children are spawned there (§3.1.1), and
	// every run artifact is resolved under it.
	Root string
	// TaskFile is the resolved task file — TP_FILE, and what the run lock,
	// the run state and the round directories are all named after.
	TaskFile string
	// Spec is the resolved spec the cycle works on.
	Spec string
	// Workflow is the effective workflow: §7's caps, the per-unit budget and
	// the runner field.
	Workflow model.Workflow
}

// DriverResult is what one run ended as. It is the loop's own answer, not a
// read of the run state file — §3.5 makes that file observability rather than
// truth, and a driver reading its own bookkeeping back would be the second
// source the design exists to avoid.
type DriverResult struct {
	RunID      string
	Phase      string
	StopReason StopReason
	Units      int
	// EscalationPath is the escalation record the run stopped on (§5.2), or
	// "" when it stopped for any other reason. When one iteration produced
	// several, it is the lowest seq's — the first unit that asked — and
	// every record is left on disk.
	EscalationPath string
	// Notify is what the operator's notify_cmd did (§5.2), or nil when the
	// run stopped converged or no command is configured. It is a report
	// about the notification and never about the run: nothing here changes
	// StopReason.
	Notify *NotifyOutcome
}

// driver is one run in flight: the options it was given, the identity it
// generated, the state file it owns and the attempt counter it hands out.
//
// seq counts unit *attempts* rather than units and is unique within the run
// (§3.1.1), so no two attempts share a log path. It lives here rather than
// being threaded through every call because it is the one piece of state the
// loop genuinely cannot re-derive from disk.
type driver struct {
	opts  *DriverOptions
	runID string
	rec   *RunRecorder
	seq   int
	// sig is the operator's signal, watched for the whole run (§3.4). It is
	// read at every point the loop is about to spawn something, which is
	// what makes "no new unit is spawned" a property of the code rather
	// than of how quickly the driver happens to notice.
	sig *interrupts
}

// RunDriver executes §3.1's loop and returns the reason it stopped.
//
// The loop holds nothing in memory it could re-derive. Each iteration reads
// the cycle through the same path tp resume uses, stops if the cycle is
// releasable, takes next_units, spawns a runner per unit — concurrent kinds
// together, every other kind alone — and then reads the cycle again from disk.
// That last step is the load-bearing one: a unit's result is whatever it wrote
// to disk, and the only thing the driver reads of the process itself is its
// exit code. Nothing here parses an agent's prose, so a driver that dies
// mid-run loses nothing but the attempts already in flight.
//
// The error return is for a failure the driver itself could not recover from —
// a runner that will not exec, a run directory it cannot write. It arrives as a
// DriverError, stops the run with driver-error and exits 4, and is deliberately
// not charged to the unit as a failed attempt (§3.4).
func RunDriver(o *DriverOptions) (DriverResult, error) {
	runID := NewULID()
	result := DriverResult{RunID: runID}

	// The run directory exists before the first child, because a child
	// addresses TP_RUN_DIR for its log and its escalation record and cannot
	// be asked to create the directory its driver named.
	//
	// These two failures are driver errors with no recorder to record them
	// in: the run state file is the very thing that could not be written, so
	// the classification travels in the error alone and the caller's exit 4
	// is the whole report.
	if err := os.MkdirAll(RunDir(o.Root, runID), 0o750); err != nil {
		return result, driverErrorf(err, "create run directory")
	}
	rec, err := NewRunRecorder(o.Root, o.TaskFile, runID, "")
	if err != nil {
		return result, driverErrorf(err, "create run state")
	}
	// §3.4's two signals are watched for the whole loop and released when it
	// ends. The watch starts after the two driver errors above: until there is
	// a recorder, a signal has nowhere to record the stop it causes, and the
	// default disposition is the more honest answer.
	sig := watchInterrupts()
	defer sig.stop()
	d := &driver{opts: o, runID: runID, rec: rec, sig: sig}

	// §3.2.2: a runner that meters nothing is reported here and nowhere
	// else, which is what makes "once at run start" a property of where the
	// line sits rather than of a flag somebody has to remember to clear. A
	// partial spend total — or none at all — is then never mistaken for a
	// complete one, and run_max_budget_usd being inert for those kinds is
	// said out loud rather than discovered from a cap that never trips.
	if msg := spendNotice(unmeteredKinds(o.Workflow.Runner)); msg != "" {
		output.Notice(msg)
	}

	// prev is what the previous iteration ended as, nil before the first has
	// run. It is carried into the next iteration's read rather than answered
	// where it was produced, because §3.4's precedence is a rule over the
	// conditions satisfied at one checkpoint: a unit that escalated or spent
	// its last attempt may have made the cycle releasable on its way, and
	// §5.2 says such an iteration still records converged. Answering it
	// before re-reading the cycle would decide that case without ever having
	// asked the question.
	var prev *iterationOutcome
	for {
		// 1. Read the cycle state through the same path tp resume uses.
		res, readErr := readCycle(d.opts)
		if readErr != nil {
			return d.fail(&result, readErr)
		}
		result.Phase = res.Phase
		if err := rec.SetPhase(res.Phase); err != nil {
			return d.fail(&result, driverErrorf(err, "record the run's phase"))
		}

		// 2, 3 and 6 are one checkpoint. Releasability, the operator's
		//    signal, the three caps, an empty next_units and what the
		//    previous iteration left are all read here, and §3.4's
		//    precedence — not the order this code happens to ask in —
		//    picks which of them the run records.
		if reason := d.checkpoint(&res, prev); reason != "" {
			if reason == StopEscalation {
				result.EscalationPath = prev.escalation
			}
			return d.stop(&result, reason), nil
		}

		// 4. Spawn a runner process per unit, retrying a unit that failed
		//    until its attempt budget is spent.
		out, iterErr := d.runIteration(&res)
		if iterErr != nil {
			return d.fail(&result, iterErr)
		}
		// 5. The next iteration's read is step five: the cycle is re-read
		//    from disk, never carried over from what a child claimed —
		//    including the driver's own reading of how the iteration went.
		prev = &out
	}
}

// checkpoint returns the reason the run stops at this point in the loop, or ""
// to carry on. res is the cycle just read from disk; prev is the previous
// iteration's outcome, nil before the first one has run.
//
// Every condition is evaluated and the winner picked by rank, rather than by
// the order the code tests them in. That is the difference §3.4's rule needs:
// an if-chain records whichever condition it reaches first, which is how a run
// that reached a cap on the very iteration that released the cycle came to
// record the cap — while §3.4 has converged lead, because a cycle that became
// releasable is releasable whatever else the iteration also hit.
func (d *driver) checkpoint(res *ResumeResult, prev *iterationOutcome) StopReason {
	// The four the checkpoint can hold at once: what the previous iteration
	// left, the signal, a cap and what the cycle says.
	//
	// They are collected in an order deliberately unlike §3.4's precedence,
	// converged last of all. Collecting them in precedence order would let a
	// ranking that had stopped working go on producing right answers, which
	// is the failure mode this whole function exists to remove — an ordering
	// nobody can observe is an ordering nobody can test.
	reasons := make([]StopReason, 0, 4)

	if prev != nil {
		switch {
		case prev.escalation != "":
			// A unit asked for a user-only decision (§5.2). It ends
			// the run before any failed sibling does: the operator
			// has to answer before anything here could be retried in
			// good faith.
			reasons = append(reasons, StopEscalation)
		case prev.interrupted:
			// A retry the signal cut short is not a unit that
			// exhausted its attempts — the attempts are still there,
			// unspent — which is why these two are exclusive rather
			// than ranked against each other.
			reasons = append(reasons, StopInterrupted)
		case !prev.finished:
			reasons = append(reasons, StopUnitFailure)
		}
	}
	// The signal is read before anything is spawned, which is what makes
	// §3.4's "no new unit is spawned" a property of the code rather than of
	// how quickly the driver happens to notice.
	if d.sig.signalled() {
		reasons = append(reasons, StopInterrupted)
	}
	// Caps are evaluated here and nowhere else, which is what keeps them
	// between iterations (§3.4): the totals are the recorder's, and nothing
	// this function does can move them.
	snapshot := d.rec.Snapshot()
	reasons = append(reasons, capStop(&snapshot, &d.opts.Workflow))
	// An empty next_units stops the run rather than re-polling: the oracle
	// has said the phase cannot proceed, and a driver that looped here would
	// spin against a state only a human can change.
	if len(res.NextUnits) == 0 {
		reasons = append(reasons, StopNoUnits)
	}
	// A releasable cycle is the run's only agreed ending, and §3.4 has it
	// lead every other row it can be found beside.
	if res.Phase == PhaseRelease {
		reasons = append(reasons, StopConverged)
	}
	return highestPrecedence(reasons...)
}

// readCycle reads the cycle exactly as tp resume does: the task file from disk
// on every iteration, then the oracle over it.
//
// A task file that cannot be read is treated as an empty task set rather than
// as an error, matching tp resume's own handling of a spec whose adjacent task
// file does not exist yet — a cycle before decomposition has no task file, and
// refusing to drive it would refuse the phase that creates one.
func readCycle(o *DriverOptions) (ResumeResult, error) {
	tf, err := model.ReadTaskFile(o.TaskFile)
	if err != nil {
		tf = &model.TaskFile{Spec: o.Spec, Tasks: []model.Task{}}
	}
	return AssembleResume(o.Root, o.TaskFile, o.Spec, tf)
}

// DryRunUnits reports the units `tp run` would spawn next without spawning any
// of them (§3.5), alongside the phase and the round they belong to.
//
// It reaches them through the driver's own first steps — the same readCycle
// every iteration of the loop starts with, then the same concurrentBatch that
// picks what one iteration spawns — so the listing is the driver's own answer
// rather than a second derivation that could quietly disagree with it.
//
// Nothing the loop does after that runs: no run directory, no run state, no run
// lock and no child. That is what makes the mode safe to point at a cycle
// another run is already driving, and it is why this is a function beside the
// loop rather than a flag threaded through it — a dry run sharing the loop's
// body would have a spawn to suppress, and a suppression is the kind of thing
// that can be got wrong once and never noticed again.
func DryRunUnits(o *DriverOptions) (phase string, round *int, units []NextUnit, err error) {
	res, err := readCycle(o)
	if err != nil {
		return "", nil, nil, err
	}
	// A releasable cycle is where the loop stops rather than spawns (§3.1
	// step 2) and the oracle already returns no unit for it, so naming the
	// phase with an empty listing is the whole honest report of that case.
	return res.Phase, res.Round, concurrentBatch(res.NextUnits), nil
}

// stop records the run's stop reason and returns the finished result. A
// recorder that cannot write its last state does not change what the run did,
// so the reason is reported either way — losing the answer to a bookkeeping
// failure would be strictly worse than an observability file one write behind.
func (d *driver) stop(result *DriverResult, reason StopReason) DriverResult {
	_ = d.rec.Stop(reason)
	result.StopReason = reason
	result.Units = d.rec.Snapshot().Totals.Units
	// §5.2: every non-converged stop is a report to a human, so the operator
	// is told through the command they configured. Convergence is the run's
	// own agreed ending and notifies nobody.
	//
	// It happens here, at the one sink every stop passes through, rather than
	// at each place a run can end: a reason that never reached this line
	// would be a reason with no run state written for it either, so there is
	// no second path a later stop could be added to and forget this one.
	if reason != StopConverged {
		result.Notify = InvokeNotify(
			d.opts.Root, d.opts.Workflow.NotifyCmd, reason, d.runID, result.EscalationPath)
	}
	return *result
}

// fail ends a run on an error the loop could not continue past, recording
// driver-error for the ones §3.4 gives that name.
//
// Only a DriverError takes the reason. The other error the loop can return is a
// runner value the configuration got wrong (§3.2), which is a usage error the
// caller exits 2 on; giving it a stop reason would tell a driver's caller a run
// had reached a state it never reached.
func (d *driver) fail(result *DriverResult, err error) (DriverResult, error) {
	var driverErr *DriverError
	if errors.As(err, &driverErr) {
		return d.stop(result, StopDriverError), err
	}
	return *result, err
}

// unitAttempt is one attempt at one unit: what the driver resolved before
// spawning, and what the child left behind.
type unitAttempt struct {
	unit     NextUnit
	env      UnitEnv
	target   UnitTarget
	runner   *Runner
	logPath  string
	seq      int
	exitCode int
	duration time.Duration
	err      error
	// escalationPath is the valid escalation record this attempt's unit
	// wrote (§5.2), or "" when it wrote none the driver could read.
	escalationPath string
}

// iterationOutcome is what one iteration ended as, in the terms §3.4's
// precedence is expressed in. The three are not exclusive alternatives so much
// as three separate questions the loop asks in order — an escalation outranks a
// failure, and a failure the operator's signal cut short is not a failure at
// all — which is why they travel together rather than as one enum.
type iterationOutcome struct {
	// escalation is the record an attempt's unit wrote (§5.2), or "".
	escalation string
	// finished reports that every unit of the iteration succeeded.
	finished bool
	// interrupted reports that a retry was withheld because the operator
	// signalled the driver mid-iteration (§3.4). It is only ever set with
	// finished false, and it is what keeps that case out of unit-failure:
	// the unit still has attempts nobody spent.
	interrupted bool
}

// runIteration spawns one iteration's units and retries the ones that failed,
// reporting the escalation record an attempt left (§5.2), whether every unit
// finished, and whether a signal stopped it short of the unit's last attempt.
//
// An escalation returns immediately, before the retry loop and before any
// failed sibling is carried forward: §3.4 ranks it above unit-failure, and a
// retry of anything in an iteration that has already asked the operator a
// question would be work done against an answer nobody has given yet.
//
// A unit gets `1 + run_max_unit_retries` attempts (§3.4), and only the units
// that failed are carried into the next attempt: a role sibling that already
// wrote its findings has finished, and re-spawning it would spend an attempt
// on work that is already durable. Each retry goes back through prepare, so it
// takes a fresh seq and log path and has its findings file cleared again
// (§3.1.1) — a leftover from the attempt that failed can never answer for the
// one that replaces it.
//
// A driver-side failure — a runner that will not exec — comes back from record
// as an error and is returned rather than retried: §3.4 keeps that apart from
// a failed attempt, and charging the unit for it would spend a budget the unit
// never got to use.
func (d *driver) runIteration(res *ResumeResult) (iterationOutcome, error) {
	units := concurrentBatch(res.NextUnits)
	budget := attemptBudget(&d.opts.Workflow)
	for attempt := 1; attempt <= budget; attempt++ {
		// §3.4's "no new unit is spawned" reaches inside the iteration as
		// well: a retry is another child, and one spawned after the
		// operator signalled would be work started against a decision
		// already made. The first attempt is not guarded here — the loop
		// read the signal immediately before calling this — so the check
		// costs the run a retry rather than the iteration.
		if attempt > 1 && d.sig.signalled() {
			return iterationOutcome{interrupted: true}, nil
		}
		batch, prepErr := d.prepare(res, units, attempt)
		if prepErr != nil {
			return iterationOutcome{}, prepErr
		}
		spawnAll(d.opts.Root, batch)
		if recErr := d.record(batch); recErr != nil {
			return iterationOutcome{}, recErr
		}
		if asked := lowestEscalation(batch); asked != "" {
			return iterationOutcome{escalation: asked}, nil
		}
		units = failedUnits(batch)
		if len(units) == 0 {
			return iterationOutcome{finished: true}, nil
		}
	}
	return iterationOutcome{}, nil
}

// prepare resolves everything one attempt at a set of units needs and returns
// the attempts to spawn. attempt is the 1-based attempt number the run state's
// rows carry (§3.5).
//
// Every command line is fully resolved before anything is spawned, which is
// how §3.2.1's "raised before any child is spawned" is kept structurally: a
// bad template or an unresolvable placeholder returns here, with the run
// having done nothing to recover from.
func (d *driver) prepare(res *ResumeResult, units []NextUnit, attempt int) ([]*unitAttempt, error) {
	attempts := make([]*unitAttempt, 0, len(units))
	for _, u := range units {
		d.seq++
		a := &unitAttempt{unit: u, seq: d.seq}
		a.env = UnitEnv{
			RunID:    d.runID,
			Root:     d.opts.Root,
			TaskFile: d.opts.TaskFile,
			Phase:    res.Phase,
			Round:    res.Round,
			Kind:     u.Kind,
			ID:       u.ID,
			Seq:      d.seq,
		}
		a.target = d.unitTarget(&a.env)
		a.logPath = unitLogPath(RunDir(d.opts.Root, d.runID), d.seq, u.Kind, u.ID)

		runner, err := ResolveUnitRunner(d.opts.Workflow.Runner, TemplateValues{
			Prompt:       UnitPrompt(u.Kind, a.target),
			Kind:         u.Kind,
			ID:           u.ID,
			LogPath:      a.logPath,
			MaxBudgetUSD: d.opts.Workflow.RunMaxUnitBudgetUSD,
		})
		if err != nil {
			return nil, err
		}
		a.runner = runner
		attempts = append(attempts, a)
	}

	for _, a := range attempts {
		// Both of these are the driver failing to prepare the ground it
		// spawns on rather than the unit failing, so both are driver
		// errors. The resolver's own errors above are not: a runner value
		// that is none of §3.2's three shapes is the configuration being
		// wrong, which exits 2 and keeps its own hint.
		if err := clearUnitArtifacts(a); err != nil {
			return nil, driverErrorf(err, "prepare %s unit %q", a.unit.Kind, a.unit.ID)
		}
		if err := d.rec.StartUnit(&RunUnitRow{
			Seq: a.seq, Kind: a.unit.Kind, ID: a.unit.ID, Attempt: attempt, LogPath: a.logPath,
		}); err != nil {
			return nil, driverErrorf(err, "record the start of %s unit %q", a.unit.Kind, a.unit.ID)
		}
	}
	return attempts, nil
}

// unitTarget names the artifacts this unit's durable-write predicate reads
// (§3.3). Outside a round-based phase the round fields stay zero, which is the
// same absence TP_ROUND and TP_ROUND_DIR carry for the child.
func (d *driver) unitTarget(u *UnitEnv) UnitTarget {
	t := UnitTarget{TaskFile: d.opts.TaskFile, Spec: d.opts.Spec, ID: u.ID}
	if u.Round != nil {
		t.Round = *u.Round
		t.RoundDir = RoundDir(d.opts.Root, d.opts.TaskFile, u.Phase, *u.Round)
	}
	return t
}

// record updates each attempt's run-state row now that its child has exited,
// then reports the first driver-side spawn failure.
//
// Every row is written before any error is returned, because §3.4 lets
// children already spawned run to completion: a sibling that finished normally
// must still be recorded when another could not be executed at all.
//
// An attempt whose runner never exec'd is the exception, and it is why this
// loop branches at all: its row keeps the nulls StartUnit wrote, which §3.5
// already reads as "never finished". Finishing it would stamp exit_code 0 on a
// child that does not exist, filing the driver's own failure as a unit that
// succeeded — the one reading of the run state a human triaging an unattended
// run must not be handed.
//
// The spend each row carries is read from the runner's final log line (§3.2.2)
// once its child has exited, and is null for every way of not having a number —
// a runner declaring no spend_key first among them. A row reporting 0 for an
// unread number would be a measurement nobody made.
func (d *driver) record(attempts []*unitAttempt) error {
	var firstErr error
	for _, a := range attempts {
		if a.err != nil {
			if firstErr == nil {
				firstErr = a.err
			}
			continue
		}
		d.recordLastFailure(a)
		if err := d.rec.FinishUnit(a.seq, a.exitCode, a.duration, a.spend()); err != nil && firstErr == nil {
			firstErr = driverErrorf(err, "record the result of %s unit %q", a.unit.Kind, a.unit.ID)
		}
	}
	return firstErr
}

// recordLastFailure keeps §4.2's advisory record in step with what the child
// did: a non-zero exit writes it, and a success of that same unit clears it.
//
// The two triggers are deliberately not each other's negation. §4.2 records a
// child that exited non-zero, while §3.3's success is exit 0 *and* the durable
// write — so an attempt that exited 0 having written nothing is a failed
// attempt that writes no record and clears none, which is the honest answer for
// a unit whose exit code says nothing went wrong.
//
// An escalating unit reaches neither branch. §5.2 says it sets no
// last_failure, and it clears none either: an escalation is a question rather
// than an outcome, so a record the unit's earlier failure left is still the
// last thing that failed.
//
// Every write error is dropped, because the record is advisory (§4.2): its
// absence never changes which unit runs next, and turning a bookkeeping failure
// into a driver error would stop a run over a hint.
func (d *driver) recordLastFailure(a *unitAttempt) {
	if a.escalated() {
		return
	}
	switch {
	case a.exitCode != 0:
		_ = WriteLastFailure(d.opts.Root, d.opts.TaskFile, &LastFailure{
			UnitKind: a.unit.Kind,
			UnitID:   a.unit.ID,
			Phase:    a.env.Phase,
			ExitCode: a.exitCode,
			Summary:  failureSummary(a),
		})
	case a.succeeded():
		_ = ClearLastFailure(d.opts.Root, d.opts.TaskFile, a.unit.Kind, a.unit.ID)
	}
}

// failureSummary is the driver's half of §4.2's tp-authored summary: the
// command that failed and the log it wrote.
//
// It is assembled from what the driver itself resolved, and the log is named
// rather than read — §4.2 forbids either writer from copying the child's prose,
// so the next unit is handed a pointer to the evidence instead of an agent's
// account of it.
func failureSummary(a *unitAttempt) string {
	command := strings.Join(append([]string{a.runner.Cmd}, a.runner.Args...), " ")
	return "command: " + command + "; log: " + a.logPath
}

// concurrentBatch picks the units one iteration spawns together (§3.1 step 4):
// units marked concurrent go together, every other kind goes alone.
//
// The oracle already guarantees it never returns a non-concurrent kind
// alongside another unit (§4.1), so this normally passes the whole array
// through. It is enforced here as well because the two guarantees protect
// different things: the oracle's keeps the advice coherent, and this one keeps
// two writers of the same shared file from ever being spawned at once, which
// is the failure no later check could undo.
func concurrentBatch(units []NextUnit) []NextUnit {
	for i := range units {
		if !units[i].Kind.Concurrent() {
			return units[:1]
		}
	}
	return units
}

// clearUnitArtifacts prepares the filesystem one attempt writes into.
//
// A role unit's findings file — both the final name and any stale .part — is
// deleted immediately before the unit is spawned (§3.1.1). The oracle has
// already omitted every role whose file satisfies the predicate, so a file
// still here belongs to a role being re-run, and leaving it would let a
// previous attempt's leftover answer for an attempt that wrote nothing.
func clearUnitArtifacts(a *unitAttempt) error {
	if err := os.MkdirAll(filepath.Dir(a.logPath), 0o750); err != nil {
		return fmt.Errorf("create log directory: %w", err)
	}
	if a.target.RoundDir == "" {
		return nil
	}
	if err := os.MkdirAll(a.target.RoundDir, 0o750); err != nil {
		return fmt.Errorf("create round directory: %w", err)
	}
	if !isRoleKind(a.unit.Kind) {
		return nil
	}
	findings := RoleFindingsPath(a.target.RoundDir, a.unit.ID)
	for _, path := range []string{findings, findings + roleFindingsPartSuffix} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("clear %s: %w", path, err)
		}
	}
	return nil
}

// spawnAll runs one iteration's attempts and waits for all of them. A
// non-concurrent kind reaches here as a batch of one, so the same code path
// serves both cases and there is no second place for the concurrency rule to
// be decided.
func spawnAll(root string, attempts []*unitAttempt) {
	parent := os.Environ()
	var wg sync.WaitGroup
	for _, a := range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.spawn(root, parent)
		}()
	}
	wg.Wait()
}

// spawn runs one child to completion and records what it did.
//
// The child is spawned in the repository root with the environment §3.1.1
// fixes, and with stdin closed: a backgrounded run that inherits a TTY-less
// stdin hangs silently, which is the hardest unattended failure to diagnose.
//
// Its stdout and stderr go to the unit's log (§3.5), unless the runner was
// handed the log path itself — then that process owns the file and the driver
// writes none of it. Both streams share one descriptor: a log read for
// post-hoc diagnosis is more use with the child's own interleaving intact than
// with its two streams separated.
//
// A runner that will not exec at all is a driver-side failure rather than a
// failed attempt, so it is kept apart from a non-zero exit code and reported
// up rather than charged to the unit (§3.4). A log the driver cannot open is
// the same kind of failure — the ground the child was to be spawned on, not
// the child — and takes the same path.
func (a *unitAttempt) spawn(root string, parent []string) {
	cmd := exec.Command(a.runner.Cmd, a.runner.Args...) //nolint:gosec // the runner is the operator's own configured command
	cmd.Dir = root
	cmd.Env = ChildEnv(parent, a.runner.Env, &a.env)
	cmd.Stdin = nil

	if !a.childOwnsLog() {
		log, err := os.OpenFile(a.logPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			a.err = driverErrorf(err, "open the log for %s unit %q", a.unit.Kind, a.unit.ID)
			return
		}
		defer log.Close()
		cmd.Stdout, cmd.Stderr = log, log
	}

	start := time.Now()
	err := cmd.Run()
	a.duration = time.Since(start)

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		a.exitCode = 0
	case errors.As(err, &exitErr):
		a.exitCode = exitErr.ExitCode()
	default:
		a.err = driverErrorf(err, "spawn %s unit %q", a.unit.Kind, a.unit.ID)
	}
	a.promoteRoleFindings()
	a.readEscalation()
}

// readEscalation keeps the escalation record this attempt's unit wrote, when it
// wrote a valid one (§5.2).
//
// The record is what the driver tests rather than the exit code, because the
// driver spawns a harness and not tp itself: the harness's code need not carry
// the inner command's. A record that is absent, unreadable or fails schema
// validation leaves the path empty, which sends the attempt back to its §3.3
// predicate and its exit code — the same judgement a unit that wrote nothing
// gets, and the reason the read's error is not reported anywhere.
//
// The path is derived from this attempt's own seq, so a record a previous
// attempt of the same unit left can never be read as this one's.
func (a *unitAttempt) readEscalation() {
	path := EscalationPath(RunDir(a.env.Root, a.env.RunID), strconv.Itoa(a.seq))
	if _, err := ReadEscalation(path); err != nil {
		return
	}
	a.escalationPath = path
}

// escalated reports whether this attempt's unit asked for a user-only
// decision. It is neither a success nor a failed attempt (§5.2).
func (a *unitAttempt) escalated() bool {
	return a.escalationPath != ""
}

// promoteRoleFindings renames a role unit's .part to the final name on exit 0,
// which is what completes that unit's durable write (§3.3.1).
//
// The unit itself may only write the .part, so this rename is the driver's own
// judgement that the child finished, and the predicate then reads the final
// name. A rename that fails leaves the final name absent, which the success
// test reads as an unfinished unit — the correct answer, and the reason there
// is nothing here to report separately.
func (a *unitAttempt) promoteRoleFindings() {
	if !isRoleKind(a.unit.Kind) || a.exitCode != 0 || a.target.RoundDir == "" {
		return
	}
	final := RoleFindingsPath(a.target.RoundDir, a.unit.ID)
	_ = os.Rename(final+roleFindingsPartSuffix, final)
}

// succeeded reports whether this attempt succeeded, by the kind's own two-part
// test: exit 0 and the durable write present (§3.3.1). The driver reads those
// two and never the unit's output.
//
// A driver-side spawn failure is not a success either, but it is not a failed
// attempt: runIteration returns it as an error before this is consulted.
func (a *unitAttempt) succeeded() bool {
	return a.err == nil && a.unit.Kind.Succeeded(a.exitCode, a.target)
}

// failedUnits returns the units of a spawned batch whose attempt did not
// succeed — the set a retry re-spawns, and the set that stops the run with
// unit-failure once the budget is spent.
//
// It is the one place an attempt's outcome is turned into "try again", so a
// later outcome that must not be charged as a failed attempt is added here
// rather than in the loop.
func failedUnits(attempts []*unitAttempt) []NextUnit {
	failed := make([]NextUnit, 0, len(attempts))
	for _, a := range attempts {
		if !a.succeeded() {
			failed = append(failed, a.unit)
		}
	}
	return failed
}

// lowestEscalation returns the escalation record an iteration's attempts left,
// the lowest seq's when several did (§5.2).
//
// Every record stays on disk — they are per unit, so concurrent siblings never
// clobber each other — and the lowest seq is only which one the run reports.
// It is chosen by seq rather than by position or by exit order so that the
// reported record is the first unit that asked, whichever child happened to
// finish first.
func lowestEscalation(attempts []*unitAttempt) string {
	lowest, path := 0, ""
	for _, a := range attempts {
		if !a.escalated() {
			continue
		}
		if path == "" || a.seq < lowest {
			lowest, path = a.seq, a.escalationPath
		}
	}
	return path
}

// capStop returns the cap a run has reached, or "" while it is within all of
// them (§3.4).
//
// The order is §3.4's precedence among the caps, so a checkpoint satisfying
// more than one always records the same reason. A cap of 0 is disabled: §7
// clamps the unit and wall-clock caps above zero so only the budget can
// legitimately arrive that way, and a zero cap that did arrive should stop
// nothing rather than stop everything.
func capStop(st *RunState, wf *model.Workflow) StopReason {
	switch {
	case wf.RunMaxBudgetUSD > 0 && st.Totals.SpendUSD >= wf.RunMaxBudgetUSD:
		return StopCapBudget
	case wf.RunMaxWallClockSeconds > 0 && st.Totals.WallClockSeconds >= wf.RunMaxWallClockSeconds:
		return StopCapWallClock
	case wf.RunMaxUnits > 0 && st.Totals.Units >= wf.RunMaxUnits:
		return StopCapUnits
	}
	return ""
}

// attemptBudget is how many times one unit is attempted: 1 + the workflow's
// run_max_unit_retries (§3.4), so the default of 1 gives two attempts and 0
// gives one attempt and no retry.
//
// A negative value — outside §7's 0-5 range, which the config layer clamps and
// only a caller building a workflow by hand can produce — still buys one
// attempt. A unit attempted zero times is not a unit the driver can report an
// exit code or a durable write for, so there is nothing it could honestly say
// about it.
func attemptBudget(wf *model.Workflow) int {
	if wf.RunMaxUnitRetries < 1 {
		return 1
	}
	return 1 + wf.RunMaxUnitRetries
}

// isRoleKind reports whether a kind is one of the two that write a role
// findings file — the kinds whose artifacts the driver clears before an
// attempt and renames after one.
func isRoleKind(k UnitKind) bool {
	return k == UnitReviewRole || k == UnitAuditRole
}

// unitLogPath returns one attempt's log, $TP_RUN_DIR/<seq>-<kind>-<id>.jsonl
// (§3.5). It is keyed by seq rather than by unit, so a retried attempt never
// overwrites the log of the attempt that failed.
func unitLogPath(runDir string, seq int, kind UnitKind, id string) string {
	return filepath.Join(runDir, strconv.Itoa(seq)+"-"+string(kind)+"-"+id+".jsonl")
}

// childOwnsLog reports whether this attempt's runner was handed the unit's log
// path, which is §3.5's test for which of the two processes writes that file:
// a template using {log_path} receives it and owns the file, and one that omits
// the placeholder does not and gets the driver's redirect instead.
//
// The question is asked of the resolved argv rather than of the template it
// was expanded from, so what decides is what the child actually receives. That
// is one rule covering the two built-in templates, the test seam, and an
// operator's own runner — including one naming the path without going through
// the placeholder, which owns the file for exactly the same reason.
func (a *unitAttempt) childOwnsLog() bool {
	if a.logPath == "" {
		return false
	}
	for _, arg := range a.runner.Args {
		if strings.Contains(arg, a.logPath) {
			return true
		}
	}
	return false
}

// spend is what this attempt's unit cost, read from the final line of its log
// under the key its runner declares (§3.2.2), or nil when there is no number to
// read — a runner declaring no spend_key first among the ways of that being so.
//
// It is asked of the attempt rather than of the log alone because the key and
// the path are both properties of the attempt: the runner is the one resolved
// for this unit's kind, which is what makes a per-kind map meter some kinds and
// not others without this call having to know that is what it is doing.
//
// The read happens after the child has exited, so the file is whatever the
// child left; whether the driver or the child wrote it makes no difference,
// since both write to the same path (§3.5).
func (a *unitAttempt) spend() *float64 {
	return readSpend(a.logPath, a.runner.SpendKey)
}
