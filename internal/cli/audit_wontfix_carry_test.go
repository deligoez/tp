package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// wontfixCarryRepo is carryRepo with round 1's security FAIL on code.go under
// the id this round's checklist derives for it, accepted wontfix, and a second
// file, other.go, the round also audits. recordedAt 2021 leaves code.go
// unchanged since the round, 2019 sees its commit as a change.
func wontfixCarryRepo(t *testing.T, recordedAt string) (dir, codeID string) {
	t.Helper()
	codeID = engine.FileCheckItemID("security", "code.go")
	row := strings.Replace(carryFailRow, `"file-security-code"`, `"`+codeID+`"`, 1)
	dir = carryRepo(t, recordedAt, row, carryWontfix)
	commitFile(t, dir, "other.go", "add other")
	return dir, codeID
}

// emitSecurityPrompt emits round 2 over code.go and other.go and returns the
// security prompt and its checklist item ids.
func emitSecurityPrompt(t *testing.T, dir string) (prompt string, ids []string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "code.go", "--affected-files", "other.go")
	require.Equal(t, 0, code, "emit round 2: %s", stderr)
	sec := auditPromptsByRole(t, stdout)["security"]
	require.NotNil(t, sec, "the security role is emitted")
	items, _ := sec["checklist_items"].([]any)
	for _, it := range items {
		m, _ := it.(map[string]any)
		id, _ := m["item_id"].(string)
		ids = append(ids, id)
	}
	return sec["prompt"].(string), ids
}

// TestWontfixCarry_TheEmissionDoesNotAskAnUnchangedWontfix (decision §2.1): a
// wontfix whose cited file is unchanged since its round is carried, so the
// next round's checklist does not ask the auditor about it, and the prompt
// marks it carried instead of listing it to re-check.
func TestWontfixCarry_TheEmissionDoesNotAskAnUnchangedWontfix(t *testing.T) {
	t.Parallel()
	dir, codeID := wontfixCarryRepo(t, "2021-01-01T00:00:00Z")

	prompt, ids := emitSecurityPrompt(t, dir)
	assert.NotContains(t, ids, codeID, "the carried item is not on the checklist")
	assert.Contains(t, ids, engine.FileCheckItemID("security", "other.go"), "the other file is still asked")
	assert.Contains(t, prompt, "## Carried", "the prompt marks the carried item")
	_, prior, _ := strings.Cut(prompt, "## Prior Round")
	assert.NotContains(t, prior, codeID, "the carried item is not listed to re-check")
}

// TestWontfixCarry_AChangedFileIsAskedAgain (decision §4, non-goal): the same
// wontfix with code.go committed after its round is not carried; the item is
// back on the checklist and nothing is marked carried.
func TestWontfixCarry_AChangedFileIsAskedAgain(t *testing.T) {
	t.Parallel()
	dir, codeID := wontfixCarryRepo(t, "2019-01-01T00:00:00Z")

	prompt, ids := emitSecurityPrompt(t, dir)
	assert.Contains(t, ids, codeID, "a wontfix on a changed file is asked again")
	assert.NotContains(t, prompt, "## Carried")
}

// TestWontfixCarry_ARowCitingNoFileIsAskedAgain (decision §2.1): a wontfix
// whose row cites no file has nothing to stay unchanged, so its item is asked
// again as before.
func TestWontfixCarry_ARowCitingNoFileIsAskedAgain(t *testing.T) {
	t.Parallel()
	codeID := engine.FileCheckItemID("security", "code.go")
	row := `{"item_id":"` + codeID + `","status":"FAIL","role":"security","category":"security","severity":"error","notes":""}` + "\n"
	dir := carryRepo(t, "2021-01-01T00:00:00Z", row, carryWontfix)
	commitFile(t, dir, "other.go", "add other")

	prompt, ids := emitSecurityPrompt(t, dir)
	assert.Contains(t, ids, codeID, "a wontfix citing no file is asked again")
	assert.NotContains(t, prompt, "## Carried")
}

// TestWontfixCarry_TheRecordWritesTheCarriedRow (decision §2.1): the round's
// results hold no row for the carried item, since nobody was asked, and
// recording the round writes it with its disposition, carried_from naming the
// round it was decided in. The round stays clean.
func TestWontfixCarry_TheRecordWritesTheCarriedRow(t *testing.T) {
	t.Parallel()
	dir, codeID := wontfixCarryRepo(t, "2021-01-01T00:00:00Z")
	otherPass := `{"item_id":"` + engine.FileCheckItemID("security", "other.go") + `","status":"PASS","role":"security",` +
		`"evidence_file":"other.go","evidence_lines":"1-2","category":null,"severity":null,"notes":""}` + "\n"

	out, stderr, code := auditRecord(t, dir, otherPass)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, true, out["clean"], "the carried wontfix leaves the round clean")
	assert.Equal(t, float64(1), out["carried"])

	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "audit-round-2.ndjson"))
	require.NoError(t, err)
	var carried map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		if row["item_id"] == codeID {
			carried = row
		}
	}
	require.NotNil(t, carried, "round 2's file holds the carried row")
	resolved, _ := carried["resolved"].(map[string]any)
	assert.Equal(t, "wontfix", resolved["status"])
	assert.Equal(t, float64(1), resolved["carried_from"])
	assert.Equal(t, "FAIL", carried["status"], "the row is the one the operator accepted")
}

// TestWontfixCarry_TheCarriedItemCountsAsGraded: the carried item is not on
// the checklist, yet the round grades it by the row the record writes, so the
// role reads "graded 2 of 2", not a role that skipped a file.
func TestWontfixCarry_TheCarriedItemCountsAsGraded(t *testing.T) {
	t.Parallel()
	dir, _ := wontfixCarryRepo(t, "2021-01-01T00:00:00Z")
	emitSecurityPrompt(t, dir)
	otherPass := `{"item_id":"` + engine.FileCheckItemID("security", "other.go") + `","status":"PASS","role":"security",` +
		`"evidence_file":"other.go","evidence_lines":"1-2","category":null,"severity":null,"notes":""}` + "\n"

	_, stderr, code := auditRecord(t, dir, otherPass)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	st, err := engine.LoadReviewState(filepath.Join(dir, "spec.md"))
	require.NoError(t, err)
	var security *engine.RoleGraded
	for _, r := range st.AuditRounds {
		for i := range r.Graded {
			if r.Round == 2 && r.Graded[i].Role == "security" {
				security = &r.Graded[i]
			}
		}
	}
	require.NotNil(t, security, "round 2 grades the security role")
	assert.Equal(t, 2, security.Pool)
	assert.Equal(t, 2, security.Graded, "the carried item counts as graded")
}
