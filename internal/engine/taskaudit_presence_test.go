package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// presenceSpec writes spec.md and a task file whose workflow lists roles and
// whose tasks are as given, returning the spec path.
func presenceSpec(t *testing.T, roles []string, tasks ...model.Task) string {
	t.Helper()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))
	tf := &model.TaskFile{Version: 1, Spec: "spec.md", Tasks: tasks, Workflow: model.WorkflowOverride{TaskAuditRoles: &roles}}
	require.NoError(t, model.WriteTaskFile(filepath.Join(dir, "spec.tasks.json"), tf))
	return spec
}

func closedTask(id string, owed ...string) model.Task {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return model.Task{ID: id, Title: id, Status: model.StatusDone, ClosedAt: &at, TaskAuditOwed: owed, Acceptance: id + " done", SourceSections: []string{"s"}}
}

func memberOf(t *model.Task, piece, pieces int) TaskAuditMember {
	return TaskAuditMember{ID: t.ID, ClosedAt: TaskAuditClosedAt(t), Piece: piece, Pieces: pieces}
}

// TestTaskAuditPresence_AStampedTaskWithoutARecordIsOwed: a done task owing a
// configured role holds the audit open until a recorded task audit holds it.
func TestTaskAuditPresence_AStampedTaskWithoutARecordIsOwed(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", "go-safety")
	spec := presenceSpec(t, []string{"go-safety"}, t1)
	st := &ReviewState{}
	assert.Equal(t, []string{"t1"}, TaskAuditOwedTasks(spec, st))
	d := AuditLoopDone(spec, st, 2, 0, "")
	assert.False(t, d.Done)
	assert.Contains(t, AuditNextAction(spec, d, true, 0, ""), "--task-audit")

	st.TaskAudits = []TaskAuditRecord{{Seq: 1, Members: []TaskAuditMember{memberOf(&t1, 0, 1)}}}
	assert.Empty(t, TaskAuditOwedTasks(spec, st))
}

// TestTaskAuditPresence_ARoleNoLongerConfiguredIsNotOwed: the debt is the
// stamp met with the roles the field lists now.
func TestTaskAuditPresence_ARoleNoLongerConfiguredIsNotOwed(t *testing.T) {
	t.Parallel()
	spec := presenceSpec(t, []string{"other"}, closedTask("t2", "go-safety"))
	assert.Empty(t, TaskAuditOwedTasks(spec, &ReviewState{}))
}

// TestTaskAuditPresence_AnUnstampedTaskOwesNothing: a task closed before the
// field was set carries no stamp.
func TestTaskAuditPresence_AnUnstampedTaskOwesNothing(t *testing.T) {
	t.Parallel()
	spec := presenceSpec(t, []string{"go-safety"}, closedTask("t1"))
	assert.Empty(t, TaskAuditOwedTasks(spec, &ReviewState{}))
}

// TestTaskAuditPresence_EverySliceMustBeRecorded: a task cut into slices owes
// until every slice is recorded.
func TestTaskAuditPresence_EverySliceMustBeRecorded(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", "go-safety")
	spec := presenceSpec(t, []string{"go-safety"}, t1)
	st := &ReviewState{TaskAudits: []TaskAuditRecord{{Seq: 1, Members: []TaskAuditMember{memberOf(&t1, 0, 2)}}}}
	assert.Equal(t, []string{"t1"}, TaskAuditOwedTasks(spec, st))
	st.TaskAudits = append(st.TaskAudits, TaskAuditRecord{Seq: 2, Members: []TaskAuditMember{memberOf(&t1, 1, 2)}})
	assert.Empty(t, TaskAuditOwedTasks(spec, st))
}

// TestTaskAuditPresence_AReClosedTaskOwesAgain: a record of an earlier close
// does not answer for a later one.
func TestTaskAuditPresence_AReClosedTaskOwesAgain(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", "go-safety")
	old := memberOf(&t1, 0, 1)
	later := t1.ClosedAt.Add(time.Hour)
	t1.ClosedAt = &later
	spec := presenceSpec(t, []string{"go-safety"}, t1)
	st := &ReviewState{TaskAudits: []TaskAuditRecord{{Seq: 1, Members: []TaskAuditMember{old}}}}
	assert.Equal(t, []string{"t1"}, TaskAuditOwedTasks(spec, st))
}
