package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The tests below are spec/backlog/audit-round-ergonomics.md §2.3 for a
// release audit round: tp audit <spec> --record-inbox records the emitted
// round from its roles' files, whole or in parts, merged as tp audit --merge
// merges them, so slicing a checklist by hand costs no --merge call.

// answerAuditRound emits audit round 1 over code.go and writes a PASS row for
// every checklist item of every role but skip; the role named sliced writes
// one part file per item and no whole file.
func answerAuditRound(t *testing.T, sliced, skip string) (dir string, roles []string) {
	t.Helper()
	dir, specPath, codePath := emissionFixture(t)
	for _, pm := range emittedPrompts(t, dir, noRoundDir(), "audit", specPath, "--affected-files", codePath) {
		role := pm["role"].(string)
		roles = append(roles, role)
		if role == skip {
			continue
		}
		whole := pm["output_path"].(string)
		require.NoError(t, os.MkdirAll(filepath.Dir(whole), 0o755))
		var lines []string
		for i, it := range pm["checklist_items"].([]any) {
			row, err := json.Marshal(map[string]any{
				"role": role, "item_id": it.(map[string]any)["item_id"], "status": "PASS",
				"evidence_file": "code.go", "evidence_lines": "1-2", "category": nil, "severity": nil, "notes": "",
			})
			require.NoError(t, err)
			if role == sliced {
				part := strings.TrimSuffix(whole, ".ndjson") + "." + string(rune('a'+i)) + ".ndjson"
				require.NoError(t, os.WriteFile(part, append(row, '\n'), 0o600))
				continue
			}
			lines = append(lines, string(row))
		}
		if role != sliced {
			require.NoError(t, os.WriteFile(whole, []byte(strings.Join(lines, "\n")+"\n"), 0o600))
		}
	}
	return dir, roles
}

// TestAuditRecordInbox_RecordsTheRoundFromItsRolesFiles: every role's file,
// one of them in parts, records as audit round 1, and the inbox leaves with
// it.
func TestAuditRecordInbox_RecordsTheRoundFromItsRolesFiles(t *testing.T) {
	t.Parallel()
	dir, _ := answerAuditRound(t, "security", "")

	stdout, stderr, code := runTPEnv(t, dir, noRoundDir(), "audit", "spec.md", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, true, out["clean"], "every item graded PASS across the files: %v", out)
	assert.FileExists(t, filepath.Join(dir, ".tp-review", "spec", "audit-round-1.ndjson"))
	assert.NoDirExists(t, filepath.Join(dir, ".tp-review", "spec", "audit-inbox"))
}

// TestAuditRecordInbox_RefusesAMissingRole: an emitted role with no file
// refuses the record, exit 3, naming the role, and nothing is recorded.
func TestAuditRecordInbox_RefusesAMissingRole(t *testing.T) {
	t.Parallel()
	dir, roles := answerAuditRound(t, "", "security")
	require.Contains(t, roles, "security")

	_, stderr, code := runTPEnv(t, dir, noRoundDir(), "audit", "spec.md", "--record-inbox")
	assert.Equal(t, 3, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "no results file from roles security")
	assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "audit-round-1.ndjson"))
}

// TestAuditRecordInbox_NamesTheConflicts: two parts of the security role
// grade one item PASS and FAIL. Both rows are kept, as --merge keeps them, and
// the record names the group under conflicts, as --merge does.
func TestAuditRecordInbox_NamesTheConflicts(t *testing.T) {
	t.Parallel()
	dir, _ := answerAuditRound(t, "security", "")
	inbox := filepath.Join(dir, ".tp-review", "spec", "audit-inbox")
	data, err := os.ReadFile(filepath.Join(inbox, "round-1-security.a.ndjson"))
	require.NoError(t, err)
	var row map[string]any
	require.NoError(t, json.Unmarshal(data, &row))
	row["status"], row["severity"], row["category"] = "FAIL", "error", "security"
	row["evidence_file"], row["evidence_lines"] = nil, nil
	fail, err := json.Marshal(row)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "round-1-security.z.ndjson"), append(fail, '\n'), 0o600))

	stdout, stderr, code := runTPEnv(t, dir, noRoundDir(), "audit", "spec.md", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	conflicts, ok := out["conflicts"].([]any)
	require.True(t, ok, "the disagreeing group is named: %s", stdout)
	require.Len(t, conflicts, 1)
	first, _ := conflicts[0].(map[string]any)
	assert.Equal(t, "security", first["role"])
	assert.Equal(t, row["item_id"], first["item_id"])
	assert.Equal(t, float64(2), first["rows"])
	assert.Contains(t, stderr, "disagreeing verdicts")
}
