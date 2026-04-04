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

func writeRoundFile(t *testing.T, dir, name string, lines []string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReviewReportThreeFiles(t *testing.T) {
	dir := t.TempDir()

	r1 := writeRoundFile(t, dir, "r1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing auth endpoint","suggestion":"add auth"}`,
		`{"severity":"medium","category":"ambiguity","location":"## Models","finding":"unclear field type for status","suggestion":"specify enum"}`,
		`{"severity":"low","category":"consistency","location":"## Tests","finding":"test naming inconsistent","suggestion":"use convention"}`,
	})

	// R2: one resolved (test naming gone), one new
	r2 := writeRoundFile(t, dir, "r2.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing auth endpoint","suggestion":"add auth"}`,
		`{"severity":"medium","category":"ambiguity","location":"## Models","finding":"unclear field type for status","suggestion":"specify enum"}`,
		`{"severity":"medium","category":"completeness","location":"## Docs","finding":"missing migration guide","suggestion":"add guide"}`,
	})

	// R3: two resolved, one remains
	r3 := writeRoundFile(t, dir, "r3.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing auth endpoint","suggestion":"add auth"}`,
	})

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2, r3)
	require.Equal(t, 0, code, "report should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Check rounds array
	rounds, ok := result["rounds"].([]any)
	require.True(t, ok, "should have rounds array")
	assert.Len(t, rounds, 3)

	// R1: 3 in_file, 3 new, 0 resolved, 3 unresolved
	r1Stats := rounds[0].(map[string]any)
	assert.Equal(t, float64(3), r1Stats["in_file"])
	assert.Equal(t, float64(3), r1Stats["new"])
	assert.Equal(t, float64(0), r1Stats["resolved"])
	assert.Equal(t, float64(3), r1Stats["unresolved"])
	assert.Nil(t, r1Stats["delta_percent"], "R1 delta_percent should be null")

	// R2: 3 in_file, 1 new (migration guide), 1 resolved (test naming), 3 unresolved (3+1-1=3)
	r2Stats := rounds[1].(map[string]any)
	assert.Equal(t, float64(3), r2Stats["in_file"])
	assert.Equal(t, float64(1), r2Stats["new"])
	assert.Equal(t, float64(1), r2Stats["resolved"])
	assert.Equal(t, float64(3), r2Stats["unresolved"])

	// R3: 1 in_file, 0 new, 2 resolved (ambiguity + migration guide), 1 unresolved (3+0-2=1)
	r3Stats := rounds[2].(map[string]any)
	assert.Equal(t, float64(1), r3Stats["in_file"])
	assert.Equal(t, float64(0), r3Stats["new"])
	assert.Equal(t, float64(2), r3Stats["resolved"])
	assert.Equal(t, float64(1), r3Stats["unresolved"])
}

func TestReviewReportFromDirectory(t *testing.T) {
	dir := t.TempDir()
	roundsDir := filepath.Join(dir, "rounds")
	require.NoError(t, os.MkdirAll(roundsDir, 0o755))

	// Write files with names that sort alphabetically (b before c)
	writeRoundFile(t, roundsDir, "b-round2.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing endpoint","suggestion":"add"}`,
	})
	writeRoundFile(t, roundsDir, "a-round1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing endpoint","suggestion":"add"}`,
		`{"severity":"low","category":"style","location":"## Code","finding":"formatting issue","suggestion":"fix"}`,
	})

	stdout, stderr, code := runTP(t, dir, "review", "--report", roundsDir)
	require.Equal(t, 0, code, "report from dir should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	rounds := result["rounds"].([]any)
	assert.Len(t, rounds, 2)

	// First round should be a-round1 (alphabetical), second b-round2
	r1 := rounds[0].(map[string]any)
	assert.Equal(t, "a-round1.ndjson", r1["file"])
	assert.Equal(t, float64(2), r1["in_file"])

	r2 := rounds[1].(map[string]any)
	assert.Equal(t, "b-round2.ndjson", r2["file"])
	assert.Equal(t, float64(1), r2["in_file"])
}

func TestReviewReportConverged(t *testing.T) {
	dir := t.TempDir()

	r1 := writeRoundFile(t, dir, "r1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing endpoint","suggestion":"add"}`,
	})
	// R2 and R3 are empty (0 findings)
	r2 := writeRoundFile(t, dir, "r2.ndjson", []string{})
	r3 := writeRoundFile(t, dir, "r3.ndjson", []string{})

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2, r3)
	require.Equal(t, 0, code, "report should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	conv := result["convergence"].(map[string]any)
	assert.Equal(t, true, conv["converged"])
	assert.Equal(t, float64(3), conv["total_rounds"])
}

func TestReviewReportNotConverged(t *testing.T) {
	dir := t.TempDir()

	r1 := writeRoundFile(t, dir, "r1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing endpoint","suggestion":"add"}`,
	})
	r2 := writeRoundFile(t, dir, "r2.ndjson", []string{
		`{"severity":"medium","category":"style","location":"## Code","finding":"naming issue","suggestion":"rename"}`,
	})

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2)
	require.Equal(t, 0, code, "report should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	conv := result["convergence"].(map[string]any)
	assert.Equal(t, false, conv["converged"])
}

func TestReviewReportBySeverityAndCategory(t *testing.T) {
	dir := t.TempDir()

	r1 := writeRoundFile(t, dir, "r1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing auth","suggestion":"add"}`,
		`{"severity":"medium","category":"ambiguity","location":"## Models","finding":"unclear type","suggestion":"specify"}`,
		`{"severity":"low","category":"consistency","location":"## Tests","finding":"naming issue","suggestion":"fix"}`,
	})
	// R2: only high remains
	r2 := writeRoundFile(t, dir, "r2.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing auth","suggestion":"add"}`,
	})

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2)
	require.Equal(t, 0, code, "report should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// by_severity: high=1 remaining, medium=1 fixed, low=1 fixed (no resolved field -> inferred)
	bySev := result["by_severity"].(map[string]any)

	highSev := bySev["high"].(map[string]any)
	assert.Equal(t, float64(1), highSev["remaining"])
	assert.Equal(t, float64(0), highSev["fixed"])

	medSev := bySev["medium"].(map[string]any)
	assert.Equal(t, float64(1), medSev["fixed"])
	assert.Equal(t, float64(0), medSev["remaining"])

	lowSev := bySev["low"].(map[string]any)
	assert.Equal(t, float64(1), lowSev["fixed"])
	assert.Equal(t, float64(0), lowSev["remaining"])

	// by_category
	byCat := result["by_category"].(map[string]any)
	assert.Equal(t, float64(1), byCat["completeness"])
	assert.Equal(t, float64(1), byCat["ambiguity"])
	assert.Equal(t, float64(1), byCat["consistency"])
}

func TestReviewReportDeltaPercentNullForR1(t *testing.T) {
	dir := t.TempDir()

	r1 := writeRoundFile(t, dir, "r1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing endpoint","suggestion":"add"}`,
	})

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1)
	require.Equal(t, 0, code, "report should succeed: %s", stderr)

	// Parse raw JSON to check null value
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	var rounds []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["rounds"], &rounds))

	assert.Equal(t, "null", string(rounds[0]["delta_percent"]))
}

func TestReviewReportNoFiles(t *testing.T) {
	dir := t.TempDir()

	_, _, code := runTP(t, dir, "review", "--report")
	assert.Equal(t, 2, code, "0 files should exit 2")
}

func TestReviewReportEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	emptyDir := filepath.Join(dir, "empty")
	require.NoError(t, os.MkdirAll(emptyDir, 0o755))

	_, _, code := runTP(t, dir, "review", "--report", emptyDir)
	assert.Equal(t, 2, code, "empty directory should exit 2")
}

func TestReviewReportWithResolvedField(t *testing.T) {
	dir := t.TempDir()

	r1 := writeRoundFile(t, dir, "r1.ndjson", []string{
		`{"severity":"high","category":"completeness","location":"## API","finding":"missing auth","suggestion":"add","resolved":"fixed"}`,
		`{"severity":"medium","category":"ambiguity","location":"## Models","finding":"unclear type","suggestion":"specify","resolved":"wontfix"}`,
		`{"severity":"low","category":"style","location":"## Code","finding":"naming","suggestion":"rename","resolved":"duplicate"}`,
		`{"severity":"high","category":"security","location":"## Auth","finding":"no rate limit","suggestion":"add"}`,
	})

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1)
	require.Equal(t, 0, code, "report should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	bySev := result["by_severity"].(map[string]any)

	// high: 1 fixed + 1 remaining (no rate limit has no resolved field)
	highSev := bySev["high"].(map[string]any)
	assert.Equal(t, float64(1), highSev["fixed"])
	assert.Equal(t, float64(1), highSev["remaining"])

	// medium: 1 wontfix
	medSev := bySev["medium"].(map[string]any)
	assert.Equal(t, float64(1), medSev["wontfix"])

	// low: 1 duplicate
	lowSev := bySev["low"].(map[string]any)
	assert.Equal(t, float64(1), lowSev["duplicate"])
}
