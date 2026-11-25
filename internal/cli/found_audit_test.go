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

// auditFoundFixture is foundApplyFixture with audit round 1 in flight: its
// snapshot is on disk and its emission kept go-safety's asked item, the one
// file it was given.
func auditFoundFixture(t *testing.T) string {
	t.Helper()
	dir := foundApplyFixture(t, `{}`)
	markRoundEmitted(t, dir, "audit")
	require.NoError(t, engine.WriteAuditPools(filepath.Join(dir, "spec.md"), 1, []engine.AuditRolePool{
		{Role: "go-safety", Pool: 1, Items: []string{"file-internal/a.go"}},
	}))
	return dir
}

// auditRowRequest files a Fix-shaped request from an audit row.
func auditRowRequest(t *testing.T, dir, rowKey string) (stdout, stderr string, code int) {
	t.Helper()
	return foundAdd(t, dir, nil, append([]string{"--audit-row", rowKey}, fixFlags...)...)
}

// recordAuditRows records audit round rows from an NDJSON body, unattended
// when asked, and returns the payload.
func recordAuditRows(t *testing.T, dir, body string, unattended bool) map[string]any {
	t.Helper()
	rows := filepath.Join(t.TempDir(), "rows.ndjson")
	require.NoError(t, os.WriteFile(rows, []byte(body), 0o600))
	stdout, stderr, code := runTPFence(t, dir, unattended, "audit", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	return parseJSON(t, stdout)
}

// recordedAuditRows reads audit round n's recorded rows.
func recordedAuditRows(t *testing.T, dir string, n int) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", fmt.Sprintf("audit-round-%d.ndjson", n)))
	require.NoError(t, err)
	var rows []map[string]any
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		rows = append(rows, row)
	}
	return rows
}

func auditConsecutiveClean(t *testing.T, dir string) any {
	t.Helper()
	stdout, stderr, code := runTPEnv(t, dir, nil, "audit", "spec.md", "--status")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	return parseJSON(t, stdout)["consecutive_clean"]
}

// outOfSurfaceRows are a round's rows: a FAIL on an item go-safety was not
// asked, and a PASS on the one it was.
const outOfSurfaceRows = `{"role":"go-safety","item_id":"outside-1","status":"FAIL","finding":"an unchecked error outside the spec"}` + "\n" +
	`{"role":"go-safety","item_id":"file-internal/a.go","status":"PASS","finding":"ok"}` + "\n"

// spec/1.31.0.md §7: a Fix-shaped finding outside the spec's surface (an
// item the auditor was not asked) is filed as a request from its row and held
// until the round is recorded; the record names the request in the row's
// disposition, so the row does not dirty the round, and the request is
// applied only then.
func TestFoundAudit_AnOutOfSurfaceFindingFilesARequestTheRecordNames(t *testing.T) {
	t.Parallel()
	dir := auditFoundFixture(t)

	stdout, stderr, code := auditRowRequest(t, dir, "go-safety:outside-1")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	reqs := storedRequests(t, dir)
	require.Len(t, reqs, 1)
	assert.Equal(t, engine.FoundFinder{AuditRound: 1, RowKey: "go-safety:outside-1"}, reqs[0].Finder)
	assert.Nil(t, foundApply(t, dir)["opened"], "held until the audit round is recorded")

	// Unattended: tp writes the disposition from the request, so no unit
	// accepts a finding and the accept-finding fence stays out of it.
	assert.Equal(t, true, recordAuditRows(t, dir, outOfSurfaceRows, true)["clean"], "the filed row does not dirty the round")
	resolved, ok := recordedAuditRows(t, dir, 1)[0]["resolved"].(map[string]any)
	require.True(t, ok, "the filed row carries a disposition")
	assert.Equal(t, "F1", resolved["request"])
	assert.Equal(t, "wontfix", resolved["status"])
	assert.Contains(t, resolved["evidence"], "F1")

	assert.Equal(t, []any{map[string]any{"request": "F1", "task": "found-f1"}}, foundApply(t, dir)["opened"])
}

// spec/1.31.0.md §7 and interview answer 2: a found task's close does not
// reset the audit's clean-round streak.
func TestFoundAudit_AFoundTasksCloseKeepsTheCleanCount(t *testing.T) {
	t.Parallel()
	dir := auditFoundFixture(t)
	_, stderr, code := auditRowRequest(t, dir, "go-safety:outside-1")
	require.Equal(t, 0, code, stderr)
	recordAuditRows(t, dir, outOfSurfaceRows, false)
	foundApply(t, dir)

	before := auditConsecutiveClean(t, dir)
	assert.Equal(t, float64(1), before)
	stdout, stderr, code := runTPEnv(t, dir, nil, "done", "found-f1", "the reproduction now exits 0")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Equal(t, before, auditConsecutiveClean(t, dir), "a found task's close does not reset the streak")
}

// spec/1.31.0.md §7: a finding that is not a row, filed by an audit-role
// unit with a plain tp found add, is applied only after its round is recorded
// too.
func TestFoundAudit_AnAuditUnitsPlainRequestWaitsForItsRound(t *testing.T) {
	t.Parallel()
	dir := auditFoundFixture(t)
	stdout, stderr, code := foundAdd(t, dir, []string{"TP_UNIT_KIND=audit-role", "TP_UNIT_ID=go-safety"}, fixFlags...)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Equal(t, engine.FoundFinder{AuditRound: 1}, storedRequests(t, dir)[0].Finder)
	assert.Nil(t, foundApply(t, dir)["opened"], "held until the audit round is recorded")

	recordAuditRows(t, dir, `{"role":"go-safety","item_id":"file-internal/a.go","status":"PASS","finding":"ok"}`+"\n", false)
	assert.Equal(t, []any{map[string]any{"request": "F1", "task": "found-f1"}}, foundApply(t, dir)["opened"])
}

// spec/1.31.0.md §7: a row on an asked item keeps the audit-fix path, so a
// request naming it is refused and nothing is filed; its recorded row stays
// open. A request that is not Fix-shaped, a key that is not role:item_id, a
// round that kept no asked items and no audit round in flight are refused too.
func TestFoundAudit_AnAskedItemOrANonFixRequestFilesNothing(t *testing.T) {
	t.Parallel()
	dir := auditFoundFixture(t)

	_, stderr, code := auditRowRequest(t, dir, "go-safety:file-internal/a.go")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "audit-fix")

	_, stderr, code = foundAdd(t, dir, nil, "--audit-row", "go-safety:outside-1",
		"--location", "internal/x.go:12", "--reproduction", "go test ./internal", "--question", "which?")
	assert.Equal(t, 2, code, stderr)

	_, stderr, code = auditRowRequest(t, dir, "outside-1")
	assert.Equal(t, 2, code, "a row key is role:item_id: %s", stderr)
	assert.Empty(t, storedRequests(t, dir))

	recordAuditRows(t, dir, `{"role":"go-safety","item_id":"file-internal/a.go","status":"FAIL","finding":"f"}`+"\n", false)
	assert.Nil(t, recordedAuditRows(t, dir, 1)[0]["resolved"], "an asked row stays open for the audit-fix path")

	noRound := foundApplyFixture(t, `{}`)
	_, stderr, code = auditRowRequest(t, noRound, "go-safety:outside-1")
	assert.Equal(t, 2, code, "no audit round in flight: %s", stderr)

	noPools := foundApplyFixture(t, `{}`)
	markRoundEmitted(t, noPools, "audit")
	_, stderr, code = auditRowRequest(t, noPools, "go-safety:outside-1")
	assert.Equal(t, 2, code, "nothing shows what the round asked: %s", stderr)
}
