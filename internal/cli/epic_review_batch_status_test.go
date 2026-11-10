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

// TestEpicReviewBatchStatus_MilestonesReadEachInTheOrderGiven is §5's
// --status --milestones: one row per listed milestone in the order given, an
// unreviewed one included with round 0 and the next_action that emits it.
func TestEpicReviewBatchStatus_MilestonesReadEachInTheOrderGiven(t *testing.T) {
	t.Parallel()
	dir := epicBatchFixture(t)

	rows := runEpicBatchStatus(t, dir, "--milestones", "L1,A")
	require.Len(t, rows, 2)
	assert.Equal(t, "L1", rows[0].Key.Value)
	assert.Equal(t, "done", rows[0].State)
	assert.Equal(t, "A", rows[1].Key.Value)
	assert.Equal(t, 0, rows[1].Round)
	assert.Equal(t, "none", rows[1].State)
	assert.Equal(t, map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}, rows[1].Open)
	assert.Equal(t, "emit round 1: tp epic review --milestone A", rows[1].NextAction)
}

// TestEpicReviewBatchStatus_CheckExitsZeroOnlyWhenEverySliceWould is §5's
// --check: the batch exits 0 only when every slice's own --status --check
// exits 0 — L1 alone passes, L1 with the capped B fails, and so does --all.
func TestEpicReviewBatchStatus_CheckExitsZeroOnlyWhenEverySliceWould(t *testing.T) {
	t.Parallel()
	dir := epicBatchFixture(t)
	require.Equal(t, 0, epicCheckCode(t, dir, "--milestone", "L1"), "L1's own check passes")
	require.Equal(t, 1, epicCheckCode(t, dir, "--milestone", "B"), "B's own check fails")

	assert.Equal(t, 0, epicCheckCode(t, dir, "--milestones", "L1"))
	assert.Equal(t, 1, epicCheckCode(t, dir, "--milestones", "L1,B"))
	assert.Equal(t, 1, epicCheckCode(t, dir, "--all"))
}

// TestEpicReviewBatchStatus_AllListsMilestoneSlicesInTrackOrder: --status
// --all lists the milestone slices in the graph's track order (A, B, L1),
// then every other slice in a stable order — the slice directory name's —
// whatever order the hashed directory names sort in.
func TestEpicReviewBatchStatus_AllListsMilestoneSlicesInTrackOrder(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicThreeMilestones)
	for _, args := range [][]string{
		{"--epic", "d"}, {"--milestone", "L1"}, {"--epics", "a,b"}, {"--milestone", "B"}, {"--milestone", "A"}, {"--epic", "c"},
	} {
		runEpicReview(t, dir, args...)
	}

	rows := runEpicBatchStatus(t, dir, "--all")
	require.Len(t, rows, 6)
	var milestones, others []string
	for _, r := range rows {
		require.NotNil(t, r.Key)
		if r.Key.Kind == "milestone" {
			require.Empty(t, others, "every milestone slice comes before the others: %v", rows)
			milestones = append(milestones, r.Key.Value)
			continue
		}
		others = append(others, r.Key.Kind+":"+r.Key.Value)
	}
	assert.Equal(t, []string{"A", "B", "L1"}, milestones, "track order")
	again := runEpicBatchStatus(t, dir, "--all")
	for i := range rows {
		assert.Equal(t, *rows[i].Key, *again[i].Key, "a stable order")
	}
	assert.Len(t, others, 3)
}
