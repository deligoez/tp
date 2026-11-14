package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	codeFail = `{"role":"go-safety","item_id":"file-go-safety-a-go.0123456789abcdef","status":"FAIL","severity":"error","notes":"exits"}`
	codePass = `{"role":"go-safety","item_id":"file-go-safety-a-go.0123456789abcdef","status":"PASS","severity":null}`
)

// taskAuditRec writes task audit seq's round file and returns its record,
// placed after audit round after.
func taskAuditRec(t *testing.T, specPath string, seq, after int, lines ...string) TaskAuditRecord {
	t.Helper()
	dir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	name := TaskAuditFileName(seq)
	content := strings.Join(lines, "\n")
	if content != "" {
		content += "\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
	return TaskAuditRecord{Seq: seq, AfterAuditRound: after, File: name, Files: []string{"a.go"}, Roles: []string{"go-safety"}}
}

// TestTaskAuditCarried_ATaskAuditFailBlocks: a task audit's FAIL joins the
// carried set and holds the audit open, naming the task audit it came from.
func TestTaskAuditCarried_ATaskAuditFailBlocks(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := &ReviewState{TaskAudits: []TaskAuditRecord{taskAuditRec(t, spec, 1, 0, codeFail)}}
	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, 1, rows[0].TaskAudit)
	assert.Equal(t, filepath.Join(ReviewStateDir(spec), "task-audit-1.ndjson"), rows[0].File)
}

// TestTaskAuditCarried_ALaterReleasePassClearsIt: the same (role, item_id)
// graded PASS by a release round after it clears the task audit's FAIL.
func TestTaskAuditCarried_ALaterReleasePassClearsIt(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{codePass})
	st.TaskAudits = []TaskAuditRecord{taskAuditRec(t, spec, 1, 0, codeFail)}
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}

// TestTaskAuditCarried_ALaterTaskAuditPassClearsAReleaseFail: and the other
// way round, a task audit after a release round re-grades its row.
func TestTaskAuditCarried_ALaterTaskAuditPassClearsAReleaseFail(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{codeFail})
	st.TaskAudits = []TaskAuditRecord{taskAuditRec(t, spec, 1, 1, codePass)}
	assert.Empty(t, AuditCarriedBlocking(spec, st))
}

// TestTaskAuditCarried_AnEarlierPassClearsNothing: order is by position among
// the audit rounds, so a task audit before a release round's FAIL does not
// clear it.
func TestTaskAuditCarried_AnEarlierPassClearsNothing(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := auditState(t, spec, AuditConvergeOnAll, []string{codeFail})
	st.TaskAudits = []TaskAuditRecord{taskAuditRec(t, spec, 1, 0, codePass)}
	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, 0, rows[0].TaskAudit)
}

// TestTaskAuditCarried_SameRoundTiesBreakBySeq: two task audits after the same
// release round order by their sequence.
func TestTaskAuditCarried_SameRoundTiesBreakBySeq(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := &ReviewState{TaskAudits: []TaskAuditRecord{
		taskAuditRec(t, spec, 2, 0, codePass),
		taskAuditRec(t, spec, 1, 0, codeFail),
	}}
	assert.Empty(t, AuditCarriedBlocking(spec, st), "seq 2's PASS comes after seq 1's FAIL")
}

// TestTaskAuditCarried_AnEpochCutsTaskAudits: an epoch's task-audit position
// cuts the task audits at or below it.
func TestTaskAuditCarried_AnEpochCutsTaskAudits(t *testing.T) {
	t.Parallel()
	spec := auditSpec(t)
	st := &ReviewState{
		TaskAudits: []TaskAuditRecord{taskAuditRec(t, spec, 1, 0, codeFail)},
		Epochs:     []Epoch{{AfterTaskAudit: 1}},
	}
	assert.Empty(t, AuditCarriedBlocking(spec, st))
	st.TaskAudits = append(st.TaskAudits, taskAuditRec(t, spec, 2, 0, codeFail))
	rows := AuditCarriedBlocking(spec, st)
	require.Len(t, rows, 1)
	assert.Equal(t, 2, rows[0].TaskAudit, strconv.Itoa(rows[0].TaskAudit))
}
