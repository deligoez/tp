package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// The tests below are spec/backlog/role-presence.md §2.4: tp review
// --record-inbox records the collected round from its inbox and refuses while
// a role the round would have asked has no file.

// inboxSpec writes a one-section spec in a fresh directory and returns both.
func inboxSpec(t *testing.T) (dir, spec string) {
	t.Helper()
	dir = t.TempDir()
	spec = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	return dir, spec
}

// writeRoleFile writes a role's inbox file for round 1 with the given body.
func writeRoleFile(t *testing.T, spec, role, body string) {
	t.Helper()
	path := engine.RoleInboxPath(spec, engine.PhaseReview, 1, role)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
}

// TestReviewRecordInbox_RecordsEveryRolesFile: with a file from every role,
// empty ones included, the round records, each row stamped with its file's
// role, and the inbox leaves with it.
func TestReviewRecordInbox_RecordsEveryRolesFile(t *testing.T) {
	t.Parallel()
	dir, spec := inboxSpec(t)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	require.GreaterOrEqual(t, len(prompts), 2, "the default panel has more than one role")
	first := prompts[0]["role"].(string)
	for i, pm := range prompts {
		body := ""
		if i == 0 {
			body = `{"severity":"low","finding":"a gap","location":"§1","evidence":"line 3"}` + "\n"
		}
		writeRoleFile(t, spec, pm["role"].(string), body)
	}

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(1), out["findings"])

	body, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
	require.NoError(t, err)
	var row map[string]any
	require.NoError(t, json.Unmarshal(body, &row))
	assert.Equal(t, first, row["role"], "the row carries its file's role")
	assert.NoDirExists(t, filepath.Join(dir, ".tp-review", "spec", "review-inbox"))
}

// TestReviewRecordInbox_RefusesAMissingRole: a role of the round with no file
// refuses the record, exit 3, naming the role, and nothing is recorded.
func TestReviewRecordInbox_RefusesAMissingRole(t *testing.T) {
	t.Parallel()
	dir, spec := inboxSpec(t)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	missing := prompts[len(prompts)-1]["role"].(string)
	for _, pm := range prompts[:len(prompts)-1] {
		writeRoleFile(t, spec, pm["role"].(string), "")
	}

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record-inbox")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, missing)
	assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
}

// TestReviewRecordInbox_ARoleFilterDoesNotShrinkThePanel: an emission
// narrowed with --role asks one role, but the round still owes every role
// of the panel; the others are named as missing.
func TestReviewRecordInbox_ARoleFilterDoesNotShrinkThePanel(t *testing.T) {
	t.Parallel()
	dir, spec := inboxSpec(t)
	all := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	only := all[0]["role"].(string)
	other := all[1]["role"].(string)
	emittedPrompts(t, dir, noRoundDir(), "review", "spec.md", "--role", only)
	writeRoleFile(t, spec, only, "")

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record-inbox")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, other, "a role the filter skipped is still owed")
}

// TestReviewRecordInbox_RefusesARowOfAnotherRole: a row naming another role
// than its file's is refused, exit 1, as epic review refuses it.
func TestReviewRecordInbox_RefusesARowOfAnotherRole(t *testing.T) {
	t.Parallel()
	dir, spec := inboxSpec(t)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	for i, pm := range prompts {
		body := ""
		if i == 0 {
			body = `{"role":"someone-else","severity":"low","finding":"a gap","location":"§1","evidence":"line 3"}` + "\n"
		}
		writeRoleFile(t, spec, pm["role"].(string), body)
	}
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record-inbox")
	assert.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "someone-else")
}

// TestReviewRecordInbox_NeedsAnEmission: with no emitted round there is no
// inbox to record, exit 3; and the flag cannot sit beside --record.
func TestReviewRecordInbox_NeedsAnEmission(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record-inbox")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "not emitted")

	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record-inbox", "--record", "x.ndjson")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
}
