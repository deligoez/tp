package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// importLoop is the review loop tp import enforces, as read from disk.
type importLoop struct {
	stateSpec  string
	rounds     []engine.ReviewRound
	wf         model.Workflow
	checksFile string // the task file the registered checks run from
	specHash   string
	done       engine.LoopDone
}

// specHashError is loadImportLoop failing to hash the spec, which import
// reports as a file error rather than as a state error.
type specHashError struct {
	path string
	err  error
}

func (e *specHashError) Error() string { return e.err.Error() }

// loadImportLoop reads the review loop of the import target, read-only. The
// spec path is pinned to the target's directory, matching how workflow
// resolution reads the spec field after the write, and the workflow is the
// resolved (project-layered) one, so a thinned task file inherits the project
// requirement rather than reading the raw task-file block alone.
//
// It returns nil and no error when no round is recorded. A directory holding
// only snapshots recorded nothing, so it reads as no recorded rounds rather
// than as corruption — the same window every other state reader accepts. Lost
// history is an error.
func loadImportLoop(targetPath string, tf *model.TaskFile) (*importLoop, error) {
	stateSpec := filepath.Join(filepath.Dir(targetPath), filepath.Base(tf.Spec))
	st, err := engine.LoadReviewState(stateSpec)
	if err != nil && !engine.IsRebuildableStateIndex(err) {
		return nil, err
	}
	if err != nil || st == nil || len(st.ReviewRounds) == 0 {
		return nil, nil
	}
	wf, checksFile := engine.ResolveWorkflow(stateSpec, flagFile)
	specHash, err := engine.SpecHash(stateSpec)
	if err != nil {
		return nil, &specHashError{path: stateSpec, err: err}
	}
	return &importLoop{
		stateSpec:  stateSpec,
		rounds:     st.ReviewRounds,
		wf:         wf,
		checksFile: checksFile,
		specHash:   specHash,
		done:       engine.ReviewLoopDone(stateSpec, st.ReviewRounds, wf.ReviewCleanRounds, wf.ReviewMaxRounds, specHash, wf.ReviewConvergeOn),
	}, nil
}

// enforceImportConvergence blocks an import whose spec has recorded review
// rounds and whose review loop has not ended. It reads engine.ReviewLoopDone,
// the verdict --status --check, next_action and tp resume read: converged, or
// the round cap reached with every finding of the latest round dispositioned.
// It runs under the task-file lock. Where the loop lets the import go ahead,
// the registered checks decide last, as they do for --status --check, from the
// verdict runImportChecks took before the lock (importCheckGate.enforce).
func enforceImportConvergence(targetPath string, tf *model.TaskFile, gate *importCheckGate) *engine.LoopDone {
	loop, err := loadImportLoop(targetPath, tf)
	if hashErr := (*specHashError)(nil); errors.As(err, &hashErr) {
		output.Error(ExitFile, fmt.Sprintf("cannot hash spec: %s", hashErr.path), hashErr.Error())
		os.Exit(ExitFile)
		return nil
	}
	if err != nil {
		exitStateError(err)
		return nil
	}
	if loop == nil {
		output.Info("review convergence not verified (no recorded rounds)")
		return nil
	}
	lastRound := loop.rounds[len(loop.rounds)-1].Round

	if settledByLoopVerdict(loop.done, loop.wf.ReviewMaxRounds, lastRound) {
		gate.enforce(loop)
		return &loop.done
	}

	required := loop.wf.ReviewCleanRounds
	hint := "record the remaining clean rounds with tp review --record, or import with user-approved --force"
	// Review-convergence enforcement uses the live severity-aware predicate so a
	// blocking-policy round whose only survivors are medium/low counts clean,
	// consistent with tp review --status/--record.
	consecutive := engine.ReviewConsecutiveClean(loop.stateSpec, loop.rounds, loop.wf.ReviewConvergeOn)
	if consecutive < required {
		output.Error(ExitValidation, fmt.Sprintf("review not converged: %d consecutive clean rounds, %d required", consecutive, required), hint)
		os.Exit(ExitValidation)
		return nil
	}
	if engine.StateStale(loop.rounds, loop.specHash) {
		output.Error(ExitValidation, fmt.Sprintf("spec changed since round %d was recorded", lastRound), hint)
		os.Exit(ExitValidation)
		return nil
	}
	gate.enforce(loop)
	return &loop.done
}

// importCheckGate is the registered checks' verdict for one import, taken
// before the task-file lock: a check may itself run a tp write against that
// task file, and under import's write lock it would wait on the lock import
// holds until it timed out. done and checks record what the verdict was taken
// over, so the locked read can confirm it still applies.
type importCheckGate struct {
	done    engine.LoopDone
	checks  []model.Check
	verdict engine.CheckVerdict
}

// runImportChecks reads the loop verdict read-only and runs the registered
// checks only when it could need them: the loop is done, the one state that
// lets an import go ahead, and a check is registered. It returns nil when it
// ran none, and reports nothing — the locked read reports every refusal.
func runImportChecks(targetPath string, tf *model.TaskFile) *importCheckGate {
	loop, err := loadImportLoop(targetPath, tf)
	if err != nil || loop == nil || !loop.done.Done || len(loop.wf.Checks) == 0 {
		return nil
	}
	results, _ := runMechanicalChecks(&loop.wf, loop.checksFile)
	return &importCheckGate{done: loop.done, checks: loop.wf.Checks, verdict: checkVerdict(loop.wf.Checks, results, loop.checksFile)}
}

// enforce refuses the import, under the lock, when a registered check did not
// pass, as `tp review --status --check` exits 1 on it: exit 1, naming the first
// such check and what it did. With no check registered it passes.
//
// The verdict was taken before the lock, so it applies only while the loop
// verdict and the registered checks it was taken over still hold. If either
// moved in between — a round recorded, a disposition, a check registered —
// the import is refused with exit 4 and a retry, rather than decided on a
// verdict about another state. --force never reaches it: the caller skips
// every convergence check.
func (g *importCheckGate) enforce(loop *importLoop) {
	if len(loop.wf.Checks) == 0 {
		return
	}
	if g == nil || g.done != loop.done || !slices.Equal(g.checks, loop.wf.Checks) {
		output.Error(ExitState, "the review loop or its registered checks changed while tp import ran the checks",
			"nothing was imported; run the same tp import again")
		os.Exit(ExitState)
		return
	}
	failing := g.verdict.Failing
	if len(failing) == 0 {
		return
	}
	msg := "review checks do not pass: " + failing[0].FixClause()
	if more := len(failing) - 1; more > 0 {
		msg += fmt.Sprintf(" (and %d more)", more)
	}
	hint := "tp review " + loop.stateSpec + " --status --check shows each check's output and exits 0 once every registered check passes; or import with user-approved --force"
	output.Error(ExitValidation, msg, hint)
	os.Exit(ExitValidation)
}

// settledByLoopVerdict handles the states the loop verdict decides on its own
// and reports whether it did: a converged loop imports; a loop the cap ended
// imports with a note naming what the cap waived, since no round verified it;
// a loop at the cap with a finding still open is refused with the way out.
// Anything else falls through to the clean-rounds and staleness checks.
// The verdict is returned to the caller for the import payload.
func settledByLoopVerdict(done engine.LoopDone, maxRounds, lastRound int) bool {
	switch {
	case done.Done && done.By == engine.DoneByCap:
		note := fmt.Sprintf("importing at the review round cap: every finding of round %d carries a disposition", lastRound)
		if done.FixedAtCap > 0 {
			note += fmt.Sprintf("; %d dispositioned fixed, which no round re-read", done.FixedAtCap)
		}
		if done.StaleWaived {
			note += fmt.Sprintf("; spec changed since round %d, not reviewed", lastRound)
		}
		output.Notice(note)
		return true
	case done.Done:
		return true
	case done.CapReached:
		msg := fmt.Sprintf("review reached its %d-round cap with %d finding(s) of round %d carrying no disposition", maxRounds, done.Undisposed, lastRound)
		if done.BlockingFixedAtCap > 0 {
			msg += fmt.Sprintf(" and %d blocking finding(s) marked fixed that no round re-read: the operator accepts them with evidence or raises the cap by one for a verification round", done.BlockingFixedAtCap)
		}
		output.Error(ExitValidation, msg, budgetEscalationHint)
		os.Exit(ExitValidation)
		return true
	}
	return false
}
