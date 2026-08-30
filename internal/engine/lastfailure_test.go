package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lastFailureRoot is a bare repository root with a task file path in it. The
// file itself is never created: the last-failure store is keyed by the task
// file's <base> and never reads the task file.
func lastFailureRoot(t *testing.T) (root, taskFile string) {
	t.Helper()
	root = t.TempDir()
	return root, filepath.Join(root, "s.tasks.json")
}

// §4.2: the record is .tp/last_failure-<base>.json under the repository root,
// named per task file so two cycles never collide.
func TestLastFailurePath_IsNamedPerCycleUnderTheProjectDir(t *testing.T) {
	root, taskFile := lastFailureRoot(t)

	assert.Equal(t, filepath.Join(root, ".tp", "last_failure-s.json"), LastFailurePath(root, taskFile))
	assert.NotEqual(t, LastFailurePath(root, taskFile),
		LastFailurePath(root, filepath.Join(root, "other.tasks.json")),
		"two cycles in one repository get two records")
}

// §4.2: the file carries exactly the six documented fields, and `at` is stamped
// by the store when the writer left it empty.
func TestWriteLastFailure_WritesTheDocumentedFields(t *testing.T) {
	root, taskFile := lastFailureRoot(t)

	require.NoError(t, WriteLastFailure(root, taskFile, &LastFailure{
		UnitKind: UnitReviewRole,
		UnitID:   "implementer",
		Phase:    PhaseReview,
		ExitCode: 3,
		Summary:  "command: fake; log: /tmp/1.jsonl",
	}))

	data, err := os.ReadFile(LastFailurePath(root, taskFile))
	require.NoError(t, err)
	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))

	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, []string{"unit_kind", "unit_id", "phase", "exit_code", "summary", "at"}, keys)
	assert.Equal(t, "review-role", raw["unit_kind"])
	assert.Equal(t, "implementer", raw["unit_id"])
	assert.Equal(t, PhaseReview, raw["phase"])
	assert.Equal(t, float64(3), raw["exit_code"])
	assert.NotEmpty(t, raw["at"], "the store stamps the clock when the writer left it empty")

	got := ReadLastFailure(root, taskFile)
	require.NotNil(t, got)
	assert.Equal(t, UnitReviewRole, got.UnitKind)
	assert.Equal(t, 3, got.ExitCode)
}

// §4.2: the file holds at most one object — a second failure overwrites the
// first rather than accumulating a log.
func TestWriteLastFailure_SecondFailureOverwritesTheFirst(t *testing.T) {
	root, taskFile := lastFailureRoot(t)

	require.NoError(t, WriteLastFailure(root, taskFile,
		&LastFailure{UnitKind: UnitImplement, UnitID: "alpha", ExitCode: 1, Summary: "first"}))
	require.NoError(t, WriteLastFailure(root, taskFile,
		&LastFailure{UnitKind: UnitImplement, UnitID: "beta", ExitCode: 2, Summary: "second"}))

	got := ReadLastFailure(root, taskFile)
	require.NotNil(t, got)
	assert.Equal(t, "beta", got.UnitID)
	assert.Equal(t, 2, got.ExitCode)
	assert.Equal(t, "second", got.Summary)
}

// §4.2: a success clears the record only when (unit_kind, unit_id) matches. Id
// alone collides — both record kinds are identified by a round number — so the
// pair is the key.
func TestClearLastFailure_ClearsOnlyOnTheMatchingKindAndID(t *testing.T) {
	root, taskFile := lastFailureRoot(t)
	write := func() {
		require.NoError(t, WriteLastFailure(root, taskFile,
			&LastFailure{UnitKind: UnitReviewRecord, UnitID: "2", ExitCode: 1, Summary: "s"}))
	}

	write()
	require.NoError(t, ClearLastFailure(root, taskFile, UnitAuditRecord, "2"))
	assert.NotNil(t, ReadLastFailure(root, taskFile),
		"a success of a different kind with the same id does not clear it")

	require.NoError(t, ClearLastFailure(root, taskFile, UnitReviewRecord, "3"))
	assert.NotNil(t, ReadLastFailure(root, taskFile),
		"a success of the same kind with a different id does not clear it")

	require.NoError(t, ClearLastFailure(root, taskFile, UnitReviewRecord, "2"))
	assert.Nil(t, ReadLastFailure(root, taskFile), "the matching pair clears it")
}

// Clearing a cycle that never failed is a no-op, not an error: the record is
// advisory and every success calls this.
func TestClearLastFailure_IsANoOpWithNoRecord(t *testing.T) {
	root, taskFile := lastFailureRoot(t)

	require.NoError(t, ClearLastFailure(root, taskFile, UnitImplement, "alpha"))
	assert.Nil(t, ReadLastFailure(root, taskFile))
}

// §4.2: the record is advisory, so an unreadable or unparseable one reads as
// absent rather than as an error a surface could refuse to report over.
func TestReadLastFailure_TreatsAnUnparseableRecordAsAbsent(t *testing.T) {
	root, taskFile := lastFailureRoot(t)
	path := LastFailurePath(root, taskFile)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{not json"), 0o600))

	assert.Nil(t, ReadLastFailure(root, taskFile))
}
