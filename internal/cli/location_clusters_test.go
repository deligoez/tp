package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clustersFrom parses the location_clusters array out of a JSON payload,
// requiring the key to be present (§8a.1 emits [] rather than omitting it).
func clustersFrom(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
	raw, ok := payload["location_clusters"].([]any)
	require.True(t, ok, "location_clusters present as an array in %s", stdout)
	out := make([]map[string]any, 0, len(raw))
	for _, c := range raw {
		out = append(out, c.(map[string]any))
	}
	return out
}

// TestReviewMerge_LocationClusters covers §8a.1 on the --merge surface: one entry
// per location key reported by more than one role, and — the half that matters —
// arithmetic that does not move because of it. The merged_count/duplicates_removed
// assertions are regression guards by construction: they hold on the code before
// this field existed, and their job is to fail if clustering ever feeds the
// counts.
func TestReviewMerge_LocationClusters(t *testing.T) {
	t.Run("groups one location two roles reported under different classes", func(t *testing.T) {
		dir := t.TempDir()
		// §1 carries two roles under two classes; §2 carries two findings from
		// ONE role; §3 carries one. If clusters fed the arithmetic, merged_count
		// would read 3 (one per location) instead of 5.
		f1 := writeFindingsFile(t, dir, "clusters.ndjson", []string{
			`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl at one"}`,
			`{"severity":"low","role":"tester","class":"B","location":"§1 trailing words","finding":"tester at one"}`,
			`{"severity":"medium","role":"implementer","class":"C","location":"§2","finding":"impl at two"}`,
			`{"severity":"high","role":"implementer","class":"D","location":"§2 more","finding":"impl at two again"}`,
			`{"severity":"critical","role":"architect","class":"E","location":"§3","finding":"arch at three"}`,
		})
		stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
		require.Equal(t, 0, code, "merge failed: %s", stderr)

		clusters := clustersFrom(t, stdout)
		require.Len(t, clusters, 1, "only §1 was reported by more than one role")
		assert.Equal(t, "§1", clusters[0]["location"], "the location key, not the raw phrasing")
		assert.Equal(t, []any{"implementer", "tester"}, clusters[0]["roles"], "distinct roles, sorted")
		assert.Equal(t, []any{"high", "low"}, clusters[0]["severities"], "distinct severities, most severe first")
		assert.Equal(t, float64(2), clusters[0]["count"], "count is the records at that location")

		// Regression guards: the pre-existing arithmetic is untouched (§8a.1).
		var summary map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
		assert.Equal(t, float64(5), summary["merged_count"], "clustering by location must not collapse records")
		assert.Equal(t, float64(0), summary["duplicates_removed"])
		assert.Equal(t, map[string]any{
			"critical": float64(1), "high": float64(2), "low": float64(1), "medium": float64(1),
		}, summary["by_severity"], "severity breakdown counts records, not clusters")
		assert.Len(t, parseNDJSON(t, mergeNDJSON(t, dir, f1)), 5, "the merged record set is unchanged")
	})

	t.Run("empty array when every location has a single role", func(t *testing.T) {
		dir := t.TempDir()
		f1 := writeFindingsFile(t, dir, "single-role.ndjson", []string{
			`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"only role"}`,
			`{"severity":"low","role":"implementer","class":"B","location":"§1 again","finding":"same role again"}`,
		})
		stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
		require.Equal(t, 0, code, "merge failed: %s", stderr)

		var summary map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
		assert.Equal(t, []any{}, summary["location_clusters"], "present and empty, never null")
	})

	t.Run("a collapsed multi-role record still reads as a multi-role location", func(t *testing.T) {
		dir := t.TempDir()
		// Same location key AND class: merge collapses these into one record
		// carrying found_by_roles, so the location is multi-role with count 1.
		f1 := writeFindingsFile(t, dir, "collapsed.ndjson", []string{
			`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"same class one"}`,
			`{"severity":"low","role":"tester","class":"A","location":"§1 words","finding":"same class two"}`,
		})
		stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1)
		require.Equal(t, 0, code, "merge failed: %s", stderr)

		clusters := clustersFrom(t, stdout)
		require.Len(t, clusters, 1)
		assert.Equal(t, []any{"implementer", "tester"}, clusters[0]["roles"], "found_by_roles supplies the roles")
		assert.Equal(t, []any{"high"}, clusters[0]["severities"], "the surviving record's severity")
		assert.Equal(t, float64(1), clusters[0]["count"])

		var summary map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
		assert.Equal(t, float64(1), summary["merged_count"], "the (location, class) dedup is unchanged")
		assert.Equal(t, float64(1), summary["duplicates_removed"])
	})

	t.Run("omitted under --compact while overlap_report stays", func(t *testing.T) {
		dir := t.TempDir()
		f1 := writeFindingsFile(t, dir, "compact.ndjson", []string{
			`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl"}`,
			`{"severity":"low","role":"tester","class":"B","location":"§1 words","finding":"tester"}`,
		})
		stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", "--compact", f1)
		require.Equal(t, 0, code, "merge failed: %s", stderr)
		var summary map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
		_, has := summary["location_clusters"]
		assert.False(t, has, "reporting-only, so §8.4 strips it under --compact")
		_, hasOverlap := summary["overlap_report"]
		assert.True(t, hasOverlap, "the decision-critical overlap_report still survives")
		assert.Equal(t, float64(2), summary["merged_count"], "regression guard: --compact arithmetic unchanged")
	})
}

// mergeNDJSON re-runs the merge without --json to capture the raw NDJSON record
// stream, so a test can assert the record set separately from the summary.
func mergeNDJSON(t *testing.T, dir, path string) string {
	t.Helper()
	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", path)
	require.Equal(t, 0, code, "merge failed: %s", stderr)
	return stdout
}

// TestReviewStatus_LocationClusters covers §8a.1 on the --status surface: the
// clusters are recomputed at read time from the latest recorded round, and the
// recorded finding count, the clean flag and the --status --check exit code are
// unchanged. Those three are regression guards by construction — they hold on
// the code before location_clusters existed. The fixture puts three roles on one
// location precisely so that a clustering bug feeding the arithmetic would read
// 1 finding instead of 3.
func TestReviewStatus_LocationClusters(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	merged := writeFindingsFile(t, dir, "merged.ndjson", []string{
		`{"severity":"high","role":"implementer","class":"A","location":"§1","finding":"impl"}`,
		`{"severity":"medium","role":"tester","class":"B","location":"§1 words","finding":"tester"}`,
		`{"severity":"low","role":"architect","class":"C","location":"§1 more","finding":"arch"}`,
	})
	recordOut, _, code := runTP(t, dir, "review", "spec.md", "--record", merged)
	require.Equal(t, 0, code)
	var rec map[string]any
	require.NoError(t, json.Unmarshal([]byte(recordOut), &rec))
	assert.Equal(t, float64(3), rec["findings"], "regression guard: --record still counts every record")
	assert.Equal(t, false, rec["clean"], "regression guard: the stored clean flag is unchanged")

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	clusters := clustersFrom(t, stdout)
	require.Len(t, clusters, 1, "one entry for §1")
	assert.Equal(t, "§1", clusters[0]["location"])
	assert.Equal(t, []any{"architect", "implementer", "tester"}, clusters[0]["roles"])
	assert.Equal(t, []any{"high", "medium", "low"}, clusters[0]["severities"], "critical → high → medium → low")
	assert.Equal(t, float64(3), clusters[0]["count"])

	var status map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &status))
	rounds := status["review_rounds"].([]any)
	require.Len(t, rounds, 1)
	assert.Equal(t, float64(3), rounds[0].(map[string]any)["findings"], "regression guard: recorded finding count")
	assert.Equal(t, false, rounds[0].(map[string]any)["clean"], "regression guard: live clean flag")
	assert.Equal(t, false, status["converged"])

	// Regression guard: --status --check still exits 1 on an unconverged spec.
	_, _, checkCode := runTP(t, dir, "review", "spec.md", "--status", "--check")
	assert.Equal(t, 1, checkCode, "regression guard: --status --check exit code")

	// §8.4: reporting-only, so --compact strips it; overlap_report stays.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	var compact map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &compact))
	_, hasClusters := compact["location_clusters"]
	assert.False(t, hasClusters, "location_clusters omitted under --compact")
	_, hasOverlap := compact["overlap_report"]
	assert.True(t, hasOverlap)
}

