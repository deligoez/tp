package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// reusedGateTime returns when a unit's worktree gate passed, when the close of
// taskID may reuse that result instead of running the gate on the main branch
// (engine.ReusableGate: the same tree, a working tree that differs only in
// tp's own files, and the same gate command), and nil otherwise. The reuse is
// said on stderr, which the driver's integrate log keeps, and the task's
// gate_passed_at is the unit's gate time rather than the close's.
func reusedGateTime(taskFilePath string, tf *model.TaskFile, taskID string) *time.Time {
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	p := engine.PendingCloseFor(root, taskFilePath, taskID)
	if p == nil {
		return nil
	}
	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	gateCmd := engine.EffectiveWorkflowForTaskFile(taskFilePath).QualityGate
	if !engine.ReusableGate(root, taskFilePath, specPath, p, gateCmd) {
		return nil
	}
	at, err := time.Parse(time.RFC3339Nano, p.At)
	if err != nil {
		return nil
	}
	output.Notice(fmt.Sprintf("gate reused from the unit's worktree: tree %s passed %q there, and the main branch holds the same tree", p.GatedTree, gateCmd))
	at = at.UTC()
	return &at
}
