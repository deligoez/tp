package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// taskAuditUnitsRepo is a spec whose one done task closed over n files and
// owes the security role, the only role task_audit_roles lists.
func taskAuditUnitsRepo(t *testing.T, n int) (root, taskFile, spec string, tf *model.TaskFile) {
	t.Helper()
	fs := make([]string, n)
	for i := range fs {
		fs[i] = fmt.Sprintf("f%02d.go", i)
	}
	spec = presenceSpec(t, []string{"security"}, closedTask("t1", fs, "security"))
	root = filepath.Dir(spec)
	taskFile = filepath.Join(root, "spec.tasks.json")
	tf, err := model.ReadTaskFile(taskFile)
	require.NoError(t, err)
	return root, taskFile, spec, tf
}

// TestTaskAuditUnits_AOneRoleBatchRunsBeforeImplement: a full batch owing one
// role is one task-audit unit, and it runs before any implement unit.
func TestTaskAuditUnits_AOneRoleBatchRunsBeforeImplement(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, tf := taskAuditUnitsRepo(t, TaskAuditBatchFiles)
	units, round := BuildNextUnits(root, taskFile, spec, PhaseImplement, tf, nil, nil)
	assert.Equal(t, []UnitKind{UnitTaskAuditRole}, unitKinds(units))
	assert.Equal(t, []string{"security"}, unitIDs(units))
	require.NotNil(t, round)
	assert.Equal(t, 1, *round)
}

// TestTaskAuditUnits_AOneRoleAuditRunsBeforeTheReleaseAudit: in the audit
// phase the partial batch is due, and its one role unit comes before the
// release audit's roles.
func TestTaskAuditUnits_AOneRoleAuditRunsBeforeTheReleaseAudit(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, tf := taskAuditUnitsRepo(t, 1)
	units, _ := BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, nil, nil)
	assert.Equal(t, []UnitKind{UnitTaskAuditRole}, unitKinds(units))
	assert.Equal(t, []string{"security"}, unitIDs(units))
}

// TestTaskAuditUnits_AReleaseRoundInFlightGoesFirst: release audit round 1
// emitted and unrecorded holds back the task audit that is due.
func TestTaskAuditUnits_AReleaseRoundInFlightGoesFirst(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, tf := taskAuditUnitsRepo(t, 1)
	dir := ReviewStateDir(spec)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, snapshotFilename(PhaseAudit, 1)), []byte("# Spec\n"), 0o600))
	units, _ := BuildNextUnits(root, taskFile, spec, PhaseAudit, tf, nil, nil)
	assert.NotContains(t, unitKinds(units), UnitTaskAuditRole)
}

// TestTaskAuditUnits_InFlightSeqIsOnePastTheRecorded: the in-flight task audit
// is the emission one past the recorded ones, seq 1 with no state.
func TestTaskAuditUnits_InFlightSeqIsOnePastTheRecorded(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, _ := taskAuditUnitsRepo(t, 0)
	target := UnitTarget{TaskFile: taskFile, Spec: spec}
	for _, st := range []*ReviewState{nil, {TaskAudits: []TaskAuditRecord{{Seq: 1}}}} {
		want := 1
		if st != nil {
			want = 2
		}
		require.NoError(t, WriteTaskAuditEmission(spec, &TaskAuditRecord{Seq: want, Roles: []string{"security"}}))
		units, seq := inFlightTaskAuditUnits(root, target, st)
		assert.Equal(t, []string{"security"}, unitIDs(units), "seq %d", want)
		require.NotNil(t, seq)
		assert.Equal(t, want, *seq)
		require.NoError(t, os.Remove(TaskAuditEmissionPath(spec, want)))
	}
}

// taskAuditFailKey is the key of codeFail's row.
const taskAuditFailKey = "go-safety:file-go-safety-a-go.0123456789abcdef"

// taskAuditFailSpec is a spec whose one recorded task audit graded a FAIL.
func taskAuditFailSpec(t *testing.T) (spec, file string) {
	t.Helper()
	spec = auditSpec(t)
	require.NoError(t, SaveReviewState(spec, &ReviewState{TaskAudits: []TaskAuditRecord{taskAuditRec(t, spec, 1, 0, codeFail)}}))
	return spec, filepath.Join(ReviewStateDir(spec), TaskAuditFileName(1))
}

// TestTaskAuditFixFile_NamesTheTaskAuditsRoundFile: a row whose latest
// grading is a task audit is fixed in that task audit's round file.
func TestTaskAuditFixFile_NamesTheTaskAuditsRoundFile(t *testing.T) {
	t.Parallel()
	spec, file := taskAuditFailSpec(t)
	assert.Equal(t, file, TaskAuditFixFile(spec, taskAuditFailKey))
	assert.Empty(t, TaskAuditFixFile(spec, "go-safety:other"))
}

// TestResumeBriefCommand_AnAuditFixNamesTheTaskAuditFile: resume's brief of
// such a fix unit names the task audit's round file.
func TestResumeBriefCommand_AnAuditFixNamesTheTaskAuditFile(t *testing.T) {
	t.Parallel()
	spec, file := taskAuditFailSpec(t)
	u := &NextUnit{Kind: UnitAuditFix, ID: taskAuditFailKey}
	want := auditFixBrief("spec.md", repoRelative(filepath.Dir(spec), file), taskAuditFailKey)
	assert.Equal(t, want, resumeBriefCommand(filepath.Dir(spec), spec, "spec.md", u))
}

// TestOwedTaskAuditsNow_ReadsTheRecordedTaskAudits: a task audit recorded on
// disk clears the task it read.
func TestOwedTaskAuditsNow_ReadsTheRecordedTaskAudits(t *testing.T) {
	t.Parallel()
	t1 := closedTask("t1", []string{"a.go"}, "security")
	spec := presenceSpec(t, []string{"security"}, t1)
	require.Equal(t, []string{"t1"}, owedTaskAuditsNow(spec))
	require.NoError(t, os.MkdirAll(ReviewStateDir(spec), 0o755))
	require.NoError(t, SaveReviewState(spec, &ReviewState{TaskAudits: []TaskAuditRecord{read(1, &t1, "a.go")}}))
	assert.Empty(t, owedTaskAuditsNow(spec))
}

// TestTaskAuditOwedClause_ShowsAtMostFive: six owed tasks are counted whole
// and named five at a time.
func TestTaskAuditOwedClause_ShowsAtMostFive(t *testing.T) {
	t.Parallel()
	owed := []string{"t1", "t2", "t3", "t4", "t5", "t6"}
	assert.Equal(t, "6 closed task(s) owe a task audit (t1, t2, t3, t4, t5): emit it — tp audit s.md --task-audit — "+
		"run its roles, then tp audit s.md --task-audit --record-inbox", TaskAuditOwedClause(owed, "s.md"))
}

// TestExpandArgs_KeepsTheTextAfterABracedLiteral: braces that are not a
// placeholder are copied, and so is the text after them.
func TestExpandArgs_KeepsTheTextAfterABracedLiteral(t *testing.T) {
	t.Parallel()
	got, err := ExpandArgs([]string{"{a b}z"}, TemplateValues{})
	require.NoError(t, err)
	assert.Equal(t, []string{"{a b}z"}, got)
}

// TestTaskAuditFixFile_SkipsARowAReleaseRoundGraded: with a task audit on
// record, a row whose latest grading is an audit round has no task-audit
// round file to be fixed in.
func TestTaskAuditFixFile_SkipsARowAReleaseRoundGraded(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{auditFail})
	st.TaskAudits = []TaskAuditRecord{taskAuditRec(t, spec, 1, 1, codeFail)}
	require.NoError(t, SaveReviewState(spec, st))
	assert.Empty(t, TaskAuditFixFile(spec, "r:task-t1"))
	assert.NotEmpty(t, TaskAuditFixFile(spec, taskAuditFailKey))
}

// TestTaskAuditUnits_EveryRoleWrittenLeavesTheRecord: once every role of the
// in-flight task audit has its file, the one unit left is its record.
func TestTaskAuditUnits_EveryRoleWrittenLeavesTheRecord(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, _ := taskAuditUnitsRepo(t, 0)
	require.NoError(t, WriteTaskAuditEmission(spec, &TaskAuditRecord{Seq: 1, Roles: []string{"security"}}))
	dir := RoundDir(root, taskFile, PhaseTaskAudit, 1)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(RoleFindingsPath(dir, "security"), []byte(codeFail+"\n"), 0o600))
	units, seq := inFlightTaskAuditUnits(root, UnitTarget{TaskFile: taskFile, Spec: spec}, nil)
	assert.Equal(t, []UnitKind{UnitTaskAuditRecord}, unitKinds(units))
	assert.Equal(t, []string{"1"}, unitIDs(units))
	require.NotNil(t, seq)
	assert.Equal(t, 1, *seq)
}

// TestTaskAuditUnits_ADueTaskAuditDrainsAPool: with a pool running, a full
// task-audit batch is still what the oracle returns, alone, and a refill
// starts nothing beside it (spec/backlog/rolling-implement.md §2).
func TestTaskAuditUnits_ADueTaskAuditDrainsAPool(t *testing.T) {
	t.Parallel()
	root, taskFile, spec, tf := taskAuditUnitsRepo(t, TaskAuditBatchFiles)
	tf.Tasks = append(tf.Tasks,
		model.Task{ID: "w", Status: model.StatusWIP, EstimateMinutes: 5, Touches: []string{"w/"}},
		model.Task{ID: "o", Status: model.StatusOpen, EstimateMinutes: 5, Touches: []string{"o/"}})
	units, _ := BuildNextUnitsRunning(root, taskFile, spec, PhaseImplement, tf, nil, nil, []string{"w"})
	assert.Equal(t, []UnitKind{UnitTaskAuditRole}, unitKinds(units))
	mixed := append(units, NextUnit{Kind: UnitImplement, ID: "o"})
	assert.Empty(t, refillUnits(mixed, map[string]*poolSlot{"w": {}}), "the pool starts nothing beside a task audit")

	implement := []NextUnit{{Kind: UnitImplement, ID: "w"}, {Kind: UnitImplement, ID: "o"}}
	assert.Equal(t, []NextUnit{{Kind: UnitImplement, ID: "o"}}, refillUnits(implement, map[string]*poolSlot{"w": {}}), "a running unit is not started twice")
}
