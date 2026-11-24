package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// tenFiles names ten source files under prefix, a full task-audit batch.
func tenFiles(prefix string) []string {
	out := make([]string, 0, engine.TaskAuditBatchFiles)
	for i := range engine.TaskAuditBatchFiles {
		out = append(out, fmt.Sprintf("%s%d.go", prefix, i))
	}
	return out
}

// unitKindsAndIDs flattens next_units into "kind:id" strings, in order.
func unitKindsAndIDs(units []map[string]any) []string {
	out := make([]string, 0, len(units))
	for _, u := range units {
		out = append(out, u["kind"].(string)+" "+u["id"].(string))
	}
	return out
}

// emitTaskAuditRole runs the brief a task-audit-role unit carries and returns
// the task audit it emitted.
func emitTaskAuditRole(t *testing.T, dir, role string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--role", role)
	require.Equal(t, 0, code, "task-audit emit: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	ta, _ := res["task_audit"].(map[string]any)
	require.NotNil(t, ta, "the emission names a task audit: %v", res)
	return ta
}

// assertOracleAgreesWithEmission is the agreement rule: the task audit the
// oracle schedules role units for is the one the role's brief emits.
func assertOracleAgreesWithEmission(t *testing.T, dir string, res map[string]any) map[string]any {
	t.Helper()
	units := nextUnitsOf(t, res)
	require.NotEmpty(t, units)
	roles := make([]any, 0, len(units))
	for _, u := range units {
		require.Equal(t, "task-audit-role", u["kind"], "%v", units)
		roles = append(roles, u["id"])
		assert.Equal(t, "tp audit spec.md --task-audit --role "+u["id"].(string), u["brief_command"])
	}
	ta := emitTaskAuditRole(t, dir, roles[0].(string))
	assert.EqualValues(t, res["round"], ta["seq"], "the round the oracle reports is the task audit's seq")
	assert.Equal(t, roles, ta["roles"], "the oracle's role units are the task audit's roles")
	return ta
}

// TestTaskAuditOracle_AFullBatchRunsBeforeImplementUnits: mid-phase, a full
// batch's role units come before the next implement unit, and the emission
// the brief runs is that batch.
func TestTaskAuditOracle_AFullBatchRunsBeforeImplementUnits(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1", "t2")
	closeWith(t, dir, "t1", commitFiles(t, dir, tenFiles("a")...))
	res := resumeResult(t, dir)
	assert.Equal(t, []string{"task-audit-role go-safety"}, unitKindsAndIDs(nextUnitsOf(t, res)))
	ta := assertOracleAgreesWithEmission(t, dir, res)
	assert.EqualValues(t, 1, ta["seq"])
	assert.Nil(t, ta["regrade"], "a batch is no re-grade")
}

// TestTaskAuditOracle_NoTaskAuditUnitMidPhaseBelowTenFiles: a partial batch
// waits for the end of the phase; the implement unit runs.
func TestTaskAuditOracle_NoTaskAuditUnitMidPhaseBelowTenFiles(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1", "t2")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go", "b.go", "c.go"))
	res := resumeResult(t, dir)
	assert.Equal(t, []string{"implement t2"}, unitKindsAndIDs(nextUnitsOf(t, res)))
}

// TestTaskAuditOracle_InFlightRunsItsPendingRolesThenItsRecord: an emitted
// task audit keeps its role units until each role's file is in its round
// directory, then owes its record unit, keyed by its seq.
func TestTaskAuditOracle_InFlightRunsItsPendingRolesThenItsRecord(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1", "t2")
	closeWith(t, dir, "t1", commitFiles(t, dir, tenFiles("a")...))
	emitTaskAudit(t, dir)

	res := resumeResult(t, dir)
	assert.Equal(t, []string{"task-audit-role go-safety"}, unitKindsAndIDs(nextUnitsOf(t, res)))
	assert.EqualValues(t, 1, res["round"])

	roundDir := engine.RoundDir(dir, filepath.Join(dir, "spec.tasks.json"), engine.PhaseTaskAudit, 1)
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	require.NoError(t, os.WriteFile(engine.RoleFindingsPath(roundDir, "go-safety"), nil, 0o600))

	res = resumeResult(t, dir)
	units := nextUnitsOf(t, res)
	assert.Equal(t, []string{"task-audit-record 1"}, unitKindsAndIDs(units))
	assert.Equal(t, "tp audit spec.md --task-audit --record-inbox", units[0]["brief_command"])
	assert.EqualValues(t, 1, res["round"])
}

// TestTaskAuditOracle_AuditPhaseOrder walks the audit phase: the owed partial
// batch, the same task audit in flight, a fix unit for its undisposed FAIL
// naming the task-audit round file, the re-grade the fix claim made due, and
// only then the release audit's roles.
func TestTaskAuditOracle_AuditPhaseOrder(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))

	res := resumeResult(t, dir)
	assert.Equal(t, "audit", res["phase"])
	assert.Equal(t, []string{"task-audit-role go-safety"}, unitKindsAndIDs(nextUnitsOf(t, res)), "the owed partial batch")
	ta := assertOracleAgreesWithEmission(t, dir, res)
	assert.EqualValues(t, 1, ta["seq"])

	res = resumeResult(t, dir)
	assert.Equal(t, []string{"task-audit-role go-safety"}, unitKindsAndIDs(nextUnitsOf(t, res)), "in flight")

	var item string
	answerTaskAudit(t, dir, emitTaskAudit(t, dir), func(id string) string { item = id; return "FAIL" })
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--task-audit", "--record-inbox")
	require.Equal(t, 0, code, "record: %s", stderr)

	key := "go-safety:" + item
	res = resumeResult(t, dir)
	units := nextUnitsOf(t, res)
	assert.Equal(t, []string{"audit-fix " + key}, unitKindsAndIDs(units), "the undisposed FAIL is fixed first")
	file := filepath.Join(".tp-review", "spec", "task-audit-1.ndjson")
	assert.Contains(t, units[0]["brief_command"], "tp audit "+file+" --resolve "+key+" fixed")

	spec := filepath.Join(dir, "spec.md")
	target := engine.UnitTarget{Spec: spec, ID: key, RoundDir: filepath.Join(dir, "no-round")}
	assert.False(t, engine.UnitAuditFix.DurableWrite(target), "the row is undisposed")
	claimFixed(t, dir, 1, item)
	assert.True(t, engine.UnitAuditFix.DurableWrite(target), "the fix claim in the task-audit file is the write")

	res = resumeResult(t, dir)
	assert.Equal(t, []string{"task-audit-role go-safety"}, unitKindsAndIDs(nextUnitsOf(t, res)), "the fix claim made a re-grade due")
	ta = assertOracleAgreesWithEmission(t, dir, res)
	assert.EqualValues(t, 2, ta["seq"])
	assert.Equal(t, true, ta["regrade"])
	recordTaskAuditWith(t, dir, emitTaskAudit(t, dir), "PASS")

	res = resumeResult(t, dir)
	units = nextUnitsOf(t, res)
	require.NotEmpty(t, units)
	for _, u := range units {
		assert.Equal(t, "audit-role", u["kind"], "only now the release audit: %v", units)
	}
}

// TestTaskAuditOracle_AnOwedBatchRunsBeforeAFix: a closed task owing its
// batch is audited before an earlier task audit's FAIL is fixed.
func TestTaskAuditOracle_AnOwedBatchRunsBeforeAFix(t *testing.T) {
	t.Parallel()
	dir, _ := failedTaskAudit(t)
	_, stderr, code := runTP(t, dir, "add", `{"id":"t2","title":"Task t2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 acceptance met","source_sections":["# Spec"]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	closeWith(t, dir, "t2", commitFiles(t, dir, "b.go"))

	res := resumeResult(t, dir)
	assert.Equal(t, []string{"task-audit-role go-safety"}, unitKindsAndIDs(nextUnitsOf(t, res)))
	ta := assertOracleAgreesWithEmission(t, dir, res)
	assert.Equal(t, []any{"b.go"}, ta["files"])
}

// TestTaskAuditOracle_AFixRunsBeforeAReGradeAChangeMadeDue: a changed file
// makes a re-grade due, but the undisposed row is fixed first.
func TestTaskAuditOracle_AFixRunsBeforeAReGradeAChangeMadeDue(t *testing.T) {
	t.Parallel()
	dir, item := failedTaskAudit(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n\n// changed\n"), 0o600))
	commitFiles(t, dir, "a.go")
	units := nextUnitsOf(t, resumeResult(t, dir))
	assert.Equal(t, []string{"audit-fix go-safety:" + item}, unitKindsAndIDs(units))
}

// TestTaskAuditOracle_NoFixUnitOnceResolved: a row resolved wontfix with
// evidence owes no fix unit and no re-grade; the release audit runs.
func TestTaskAuditOracle_NoFixUnitOnceResolved(t *testing.T) {
	t.Parallel()
	dir, item := failedTaskAudit(t)
	file := filepath.Join(".tp-review", "spec", "task-audit-1.ndjson")
	_, stderr, code := runTP(t, dir, "audit", file, "--resolve", "go-safety:"+item, "wontfix", "the exit is the documented contract")
	require.Equal(t, 0, code, "resolve: %s", stderr)
	units := nextUnitsOf(t, resumeResult(t, dir))
	require.NotEmpty(t, units)
	for _, u := range units {
		assert.False(t, strings.HasPrefix(u["kind"].(string), "task-audit") || u["kind"] == "audit-fix", "%v", units)
	}
}
