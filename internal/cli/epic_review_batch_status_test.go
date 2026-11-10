package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicBatchStatusRow is one row of a batch --status.
type epicBatchStatusRow struct {
	Key        *epicSliceKeyPayload `json:"key"`
	Round      int                  `json:"round"`
	State      string               `json:"state"`
	Open       map[string]int       `json:"open"`
	Grantable  *bool                `json:"grantable"`
	NextAction string               `json:"next_action"`
}

// runEpicBatchStatus runs tp epic review <args> --status and decodes its rows
// on exit 0.
func runEpicBatchStatus(t *testing.T, dir string, args ...string) []epicBatchStatusRow {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, args...), "--status")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Slices []epicBatchStatusRow `json:"slices"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out.Slices
}

// epicRowsByValue maps each row to its key's value.
func epicRowsByValue(t *testing.T, rows []epicBatchStatusRow) map[string]epicBatchStatusRow {
	t.Helper()
	byValue := map[string]epicBatchStatusRow{}
	for _, r := range rows {
		require.NotNil(t, r.Key, "every row names its key")
		byValue[r.Key.Value] = r
	}
	return byValue
}

// TestEpicReviewBatchStatus_AllReadsEverySliceDirectory is §5's --status
// --all: one row per slice directory under .tp/epic-review/ — the capped B,
// the done L1 and the in-flight --epic d, never the unreviewed A — each with
// its key, round, state, open counts, grantable and next_action. Listing the
// tracked milestones instead, or only milestone slices, fails it.
func TestEpicReviewBatchStatus_AllReadsEverySliceDirectory(t *testing.T) {
	t.Parallel()
	dir := epicBatchFixture(t)
	runEpicReview(t, dir, "--epic", "d")

	rows := epicRowsByValue(t, runEpicBatchStatus(t, dir, "--all"))
	require.Len(t, rows, 3, "rows: %v", rows)
	b := rows["B"]
	assert.Equal(t, "milestone", b.Key.Kind)
	assert.Equal(t, 1, b.Round)
	assert.Equal(t, "cap_reached", b.State)
	assert.Equal(t, 1, b.Open["high"])
	require.NotNil(t, b.Grantable)
	assert.False(t, *b.Grantable)
	assert.Contains(t, b.NextAction, "round cap is reached")
	assert.Equal(t, "done", rows["L1"].State)
	assert.Contains(t, rows["L1"].NextAction, "done by converged")
	d := rows["d"]
	assert.Equal(t, "epic", d.Key.Kind)
	assert.Equal(t, "in_flight", d.State)
	assert.Contains(t, d.NextAction, "record round 1: tp epic review --epic d --record")
}

