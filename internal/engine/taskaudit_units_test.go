package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestTaskAuditUnits_RoundDirIsTheTaskAuditsOwn: a task-audit unit's
// TP_ROUND_DIR, and the directory its durable write is read in, is the task
// audit's round directory keyed by its seq, whatever phase the oracle is in.
func TestTaskAuditUnits_RoundDirIsTheTaskAuditsOwn(t *testing.T) {
	root := t.TempDir()
	taskFile := filepath.Join(root, "spec.tasks.json")
	seq := 3
	want := RoundDir(root, taskFile, PhaseTaskAudit, seq)
	for _, kind := range []UnitKind{UnitTaskAuditRole, UnitTaskAuditRecord} {
		for _, phase := range []string{PhaseImplement, PhaseAudit} {
			u := &UnitEnv{RunID: "r", Root: root, TaskFile: taskFile, Phase: phase, Round: &seq, Kind: kind, ID: "go-safety"}
			assert.Equal(t, want, driverEnv(u)[EnvRoundDir], "%s in %s", kind, phase)
			d := &driver{opts: &DriverOptions{Root: root, TaskFile: taskFile}}
			assert.Equal(t, want, d.unitTarget(u).RoundDir, "%s in %s", kind, phase)
		}
	}
	// Every other kind keeps the oracle's phase.
	u := &UnitEnv{RunID: "r", Root: root, TaskFile: taskFile, Phase: PhaseAudit, Round: &seq, Kind: UnitAuditRole, ID: "go-safety"}
	assert.Equal(t, RoundDir(root, taskFile, PhaseAudit, seq), driverEnv(u)[EnvRoundDir])
}

// TestTaskAuditUnits_TheRoleIsARoleKind: the driver clears and renames a
// task-audit role's .part as it does an audit role's.
func TestTaskAuditUnits_TheRoleIsARoleKind(t *testing.T) {
	assert.True(t, isRoleKind(UnitTaskAuditRole))
	assert.False(t, isRoleKind(UnitTaskAuditRecord))
}
