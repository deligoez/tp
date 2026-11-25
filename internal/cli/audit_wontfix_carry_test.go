package cli_test

import (
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
