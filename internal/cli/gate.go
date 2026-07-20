package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// gateOutputTailLines is the reported output tail size on gate failure.
const gateOutputTailLines = 20

// gateSkipHint tells the agent how to proceed after a gate failure.
const gateSkipHint = "fix the gate failure and retry, or close with --skip-gate '<why>' (recorded on the task)"

// executeQualityGate runs workflow.quality_gate in the task file's directory
// with the resolved timeout and returns the result without exiting.
func executeQualityGate(tf *model.TaskFile, taskFilePath string) engine.RunResult {
	timeout := tf.Workflow.EffectiveGateTimeoutSeconds()
	return engine.RunCommand(tf.Workflow.QualityGate, filepath.Dir(taskFilePath), time.Duration(timeout)*time.Second, gateOutputTailLines)
}

// gateFailureMessage renders the top-level error string for a failed gate run.
func gateFailureMessage(tf *model.TaskFile, res engine.RunResult) string {
	if res.TimedOut {
		return fmt.Sprintf("gate timed out after %ds", tf.Workflow.EffectiveGateTimeoutSeconds())
	}
	return fmt.Sprintf("quality gate failed: %s", tf.Workflow.QualityGate)
}

// runQualityGatePreFlock executes the quality gate once per invocation, before
// the task-file flock is acquired. Returns true when the gate executed and
// passed, false when no gate is configured. On failure it emits the error
// object carrying gate_cmd, exit_code, and output_tail, then exits with
// ExitState — no task closes.
func runQualityGatePreFlock(tf *model.TaskFile, taskFilePath string) bool {
	if tf.Workflow.QualityGate == "" {
		return false
	}
	res := executeQualityGate(tf, taskFilePath)
	if res.Passed {
		return true
	}
	errOut := map[string]any{
		"error":       gateFailureMessage(tf, res),
		"code":        ExitState,
		"gate_cmd":    tf.Workflow.QualityGate,
		"exit_code":   res.ExitCode,
		"output_tail": res.OutputTail,
		"hint":        gateSkipHint,
	}
	data, _ := json.Marshal(errOut)
	fmt.Fprintln(os.Stderr, string(data))
	os.Exit(ExitState)
	return false
}

// doneCheckError is a cheap-check failure surfaced before the gate runs.
type doneCheckError struct {
	code       int
	msg        string
	hint       string
	closure    bool   // true: emit the closure-verification JSON error shape
	acceptance string // acceptance text for the closure error shape
}

// checkDoneTarget runs the cheap pre-gate checks for one close target against
// a lock-free read of the task file: task lookup, dependency check for the
// implicit claim, state transition, covered-by reference, and closure
// verification. assumeDone lists in-invocation IDs treated as already closed
// for dependency checks. It never mutates the task file.
func checkDoneTarget(tf *model.TaskFile, taskID, reason, coveredBy string, assumeDone map[string]bool) *doneCheckError {
	task, _, findErr := model.FindTask(tf, taskID)
	if findErr != nil {
		return &doneCheckError{code: ExitState, msg: findErr.Error()}
	}

	if task.Status == model.StatusOpen {
		done := make(map[string]bool)
		for i := range tf.Tasks {
			if tf.Tasks[i].Status == model.StatusDone {
				done[tf.Tasks[i].ID] = true
			}
		}
		for _, dep := range task.DependsOn {
			if !done[dep] && !assumeDone[dep] {
				return &doneCheckError{code: ExitState, msg: fmt.Sprintf("cannot done: task %s is blocked by %s", task.ID, dep)}
			}
		}
	}

	if task.Status == model.StatusDone {
		return &doneCheckError{code: ExitState, msg: fmt.Sprintf("task %s is already done", task.ID)}
	}

	if coveredBy != "" {
		ref, _, refErr := model.FindTask(tf, coveredBy)
		if refErr != nil {
			return &doneCheckError{code: ExitState, msg: fmt.Sprintf("--covered-by: %v", refErr), hint: coveredByHint(tf, coveredBy)}
		}
		if ref.Status != model.StatusDone {
			return &doneCheckError{code: ExitState, msg: fmt.Sprintf("--covered-by: task %s is %s (must be done)", ref.ID, ref.Status)}
		}
	}

	if verifyErr := engine.VerifyClosure(task.Acceptance, reason, coveredBy != ""); verifyErr != nil {
		return &doneCheckError{
			code:       ExitValidation,
			msg:        fmt.Sprintf("closure verification failed: %v", verifyErr),
			hint:       engine.ClosureHint(verifyErr, "Rewrite reason to address all acceptance criteria, then retry tp done."),
			closure:    true,
			acceptance: task.Acceptance,
		}
	}

	return nil
}

// exitDoneCheckError emits a cheap-check failure in the established error
// shapes and exits with the failure's code.
func exitDoneCheckError(ce *doneCheckError) {
	switch {
	case ce.closure:
		errOut := map[string]any{
			"error":      ce.msg,
			"code":       ce.code,
			"acceptance": ce.acceptance,
			"hint":       ce.hint,
		}
		data, _ := json.Marshal(errOut)
		fmt.Fprintln(os.Stderr, string(data))
	case ce.hint != "":
		output.Error(ce.code, ce.msg, ce.hint)
	default:
		output.Error(ce.code, ce.msg)
	}
	os.Exit(ce.code)
}

// batchNeedsGate reports whether the gate must run: at least one entry
// survives the cheap checks and does not carry a non-empty skip_gate.
func batchNeedsGate(tf *model.TaskFile, entries []batchEntry) bool {
	sorted, _, cycles := toposortBatchEntries(entries, tf)
	assume := make(map[string]bool)
	needs := false
	for i := range sorted {
		e := &sorted[i]
		if cycles[e.ID] != "" {
			continue
		}
		if e.SkipGate != nil && strings.TrimSpace(*e.SkipGate) == "" {
			continue // cheap-check failure, not a survivor
		}
		if ce := checkDoneTarget(tf, e.ID, e.Reason, e.CoveredBy, assume); ce == nil {
			assume[e.ID] = true
			if e.SkipGate == nil {
				needs = true
			}
		}
	}
	return needs
}

