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

// presenceSpec writes spec.md, the files the tasks closed over, and a task
// file whose workflow lists roles, returning the spec path. "security" is a
// built-in auditor, so the panel holds it with no corpus on disk.
func presenceSpec(t *testing.T, roles []string, tasks ...model.Task) string {
	t.Helper()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))
	for i := range tasks {
		for _, f := range tasks[i].CommitFiles {
			require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte("package x\n"), 0o600))
		}
	}
	tf := &model.TaskFile{Version: 1, Spec: "spec.md", Tasks: tasks, Workflow: model.WorkflowOverride{TaskAuditRoles: &roles}}
	require.NoError(t, model.WriteTaskFile(filepath.Join(dir, "spec.tasks.json"), tf))
	return spec
}

func closedTask(id string, files []string, owed ...string) model.Task {
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	return model.Task{ID: id, Title: id, Status: model.StatusDone, ClosedAt: &at, TaskAuditOwed: owed,
		CommitFiles: files, Acceptance: id + " done", SourceSections: []string{"s"}}
}

// read is a recorded task audit whose member t read files.
func read(seq int, t *model.Task, files ...string) TaskAuditRecord {
	return TaskAuditRecord{Seq: seq, Members: []TaskAuditMember{{ID: t.ID, ClosedAt: TaskAuditClosedAt(t), Files: files}}}
}

// TestTaskAuditPresence_AStampedTaskWithoutARecordIsOwed: a done task owing a
// role holds the audit open until a recorded task audit read its files.
func TestTaskAuditPresence_AStampedTaskWithoutARecordIsOwed(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", []string{"a.go"}, "security")
	spec := presenceSpec(t, []string{"security"}, t1)
	st := &ReviewState{}
	assert.Equal(t, []string{"t1"}, TaskAuditOwedTasks(spec, st))
	d := AuditLoopDone(spec, st, 2, 0, "")
	assert.False(t, d.Done)
	assert.Contains(t, AuditNextAction(spec, d, true, 0, ""), "--task-audit")

	st.TaskAudits = []TaskAuditRecord{read(1, &t1, "a.go")}
	assert.Empty(t, TaskAuditOwedTasks(spec, st))
}

// TestTaskAuditPresence_OneOwedTaskHoldsAConvergedAudit: two clean audit
// rounds converge the loop, and a single task still owing holds it open.
func TestTaskAuditPresence_OneOwedTaskHoldsAConvergedAudit(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", []string{"a.go"}, "security")
	spec := presenceSpec(t, []string{"security"}, t1)
	st := auditState(t, spec, AuditConvergeOnAll, []string{codePass}, []string{codePass})
	for i := range st.AuditRounds {
		st.AuditRounds[i].Clean = true
		st.AuditRounds[i].HashScheme = HashSchemeSnapshot
	}
	hash := st.AuditRounds[0].SpecHash
	assert.False(t, AuditLoopDone(spec, st, 2, 0, hash).Done, "one owed task holds the loop")
	st.TaskAudits = []TaskAuditRecord{read(1, &t1, "a.go")}
	assert.True(t, AuditLoopDone(spec, st, 2, 0, hash).Done, "read, the loop converges")
}

// TestTaskAuditPresence_ARoleOutsideThePanelIsNotOwed: a role the spec's
// audit panel does not hold is never asked, so it is never owed; it is
// reported instead.
func TestTaskAuditPresence_ARoleOutsideThePanelIsNotOwed(t *testing.T) {
	t.Parallel()
	spec := presenceSpec(t, []string{"ghost-role"}, closedTask("t2", []string{"a.go"}, "ghost-role"))
	assert.Empty(t, TaskAuditOwedTasks(spec, &ReviewState{}))
	assert.Equal(t, []string{"ghost-role"}, TaskAuditRolesOutsidePanel(spec))
}

// TestTaskAuditPresence_AnUnstampedTaskOwesNothing: a task closed before the
// field was set carries no stamp.
func TestTaskAuditPresence_AnUnstampedTaskOwesNothing(t *testing.T) {
	t.Parallel()
	spec := presenceSpec(t, []string{"security"}, closedTask("t1", []string{"a.go"}))
	assert.Empty(t, TaskAuditOwedTasks(spec, &ReviewState{}))
}

// TestTaskAuditPresence_ACloseWithNothingToReadOwesNothing: a close with no
// auditable file left in the tree has nothing to audit.
func TestTaskAuditPresence_ACloseWithNothingToReadOwesNothing(t *testing.T) {
	t.Parallel()
	gone := closedTask("t1", nil, "security")
	gone.CommitFiles = []string{"deleted.go"}
	spec := presenceSpec(t, []string{"security"}, closedTask("t0", nil, "security"))
	tf, err := model.ReadTaskFile(filepath.Join(filepath.Dir(spec), "spec.tasks.json"))
	require.NoError(t, err)
	tf.Tasks = append(tf.Tasks, gone)
	require.NoError(t, model.WriteTaskFile(filepath.Join(filepath.Dir(spec), "spec.tasks.json"), tf))
	assert.Empty(t, TaskAuditOwedTasks(spec, &ReviewState{}), "no commit files, and a file gone from the tree, owe nothing")
}

// TestTaskAuditPresence_EveryFileMustBeRead: a task over ten files owes until
// every file is read, whatever slices the reads came in.
func TestTaskAuditPresence_EveryFileMustBeRead(t *testing.T) {
	t.Parallel()
	var many []string
	for i := range 12 {
		many = append(many, filepath.Base(filepath.Join("x", string(rune('a'+i))+".go")))
	}
	t1 := closedTask("t1", many, "security")
	spec := presenceSpec(t, []string{"security"}, t1)
	st := &ReviewState{TaskAudits: []TaskAuditRecord{read(1, &t1, many[:10]...)}}
	assert.Equal(t, []string{"t1"}, TaskAuditOwedTasks(spec, st))
	st.TaskAudits = append(st.TaskAudits, read(2, &t1, many[10:]...))
	assert.Empty(t, TaskAuditOwedTasks(spec, st))
}

// TestTaskAuditPresence_AnUnreadableTaskFileHoldsTheAuditAndNamesTheRepair:
// presence fails closed, and next_action names the file to repair rather
// than a task audit no emission could run.
func TestTaskAuditPresence_AnUnreadableTaskFileHoldsTheAuditAndNamesTheRepair(t *testing.T) {
	t.Parallel()
	spec := presenceSpec(t, []string{"security"}, closedTask("t1", []string{"a.go"}, "security"))
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(spec), "spec.tasks.json"), []byte("{"), 0o600))
	require.Len(t, TaskAuditOwedTasks(spec, &ReviewState{}), 1)
	d := AuditLoopDone(spec, &ReviewState{}, 2, 0, "")
	assert.False(t, d.Done)
	na := AuditNextAction(spec, d, true, 0, "")
	assert.Contains(t, na, "cannot be read")
	assert.NotContains(t, na, "--task-audit")
}

// TestTaskAuditPresence_AQuotedCommitFileIsStillOwed: commit_files keeps the
// C-quoted form git prints for a name outside plain ASCII; the file is read
// as the name it stands for, not dropped as absent.
func TestTaskAuditPresence_AQuotedCommitFileIsStillOwed(t *testing.T) {
	t.Parallel()
	spec := presenceSpec(t, []string{"security"}, closedTask("t0", nil))
	dir := filepath.Dir(spec)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ü.go"), []byte("package x\n"), 0o600))
	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	tf.Tasks = append(tf.Tasks, closedTask("t1", []string{`"\303\274.go"`}, "security"))
	require.NoError(t, model.WriteTaskFile(filepath.Join(dir, "spec.tasks.json"), tf))
	cands := TaskAuditCandidates(spec, tf.Tasks, []string{"security"})
	require.Len(t, cands, 1, "t0 carries no stamp")
	assert.Equal(t, []string{"ü.go"}, cands[0].Files)
}

// TestTaskAuditPresence_AReClosedTaskOwesAgain: a record of an earlier close
// does not answer for a later one.
func TestTaskAuditPresence_AReClosedTaskOwesAgain(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", []string{"a.go"}, "security")
	old := read(1, &t1, "a.go")
	later := t1.ClosedAt.Add(time.Hour)
	t1.ClosedAt = &later
	spec := presenceSpec(t, []string{"security"}, t1)
	assert.Equal(t, []string{"t1"}, TaskAuditOwedTasks(spec, &ReviewState{TaskAudits: []TaskAuditRecord{old}}))
}
