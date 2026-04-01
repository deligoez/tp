package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoneCoveredBy(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"t1","title":"First","depends_on":[],"estimate_minutes":5,"acceptance":"First done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Second","depends_on":[],"estimate_minutes":5,"acceptance":"Second done with specific criteria","source_sections":["s1"]}`)

	// Close t1 normally
	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "First done completely with evidence")

	// Close t2 as covered-by t1 — should skip closure verification
	stdout, stderr, code := runTP(t, dir, "done", "t2", "covered by t1: test #26 covers this", "--covered-by", "t1", "--gate-passed")
	require.Equal(t, 0, code, "covered-by should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "t2", result["closed"])
}

func TestDoneCoveredByNotDone(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"t1","title":"First","depends_on":[],"estimate_minutes":5,"acceptance":"First done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Second","depends_on":[],"estimate_minutes":5,"acceptance":"Second done","source_sections":["s1"]}`)

	// t1 is not done — covered-by should fail
	_, stderr, code := runTP(t, dir, "done", "t2", "covered by t1", "--covered-by", "t1")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "must be done")
}

func TestDoneCoveredByNonExistent(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"t1","title":"First","depends_on":[],"estimate_minutes":5,"acceptance":"First done","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "covered by ghost", "--covered-by", "nonexistent")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "not found")
}

func TestDoneCoveredByForbiddenPatternAllowed(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"t1","title":"First","depends_on":[],"estimate_minutes":5,"acceptance":"First done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Second","depends_on":[],"estimate_minutes":5,"acceptance":"Second criteria","source_sections":["s1"]}`)

	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "First done completely with evidence")

	// "covered by existing" would normally be rejected, but --covered-by allows it
	_, stderr, code := runTP(t, dir, "done", "t2", "covered by existing test in t1", "--covered-by", "t1")
	require.Equal(t, 0, code, "covered-by should bypass forbidden patterns: %s", stderr)
}

func TestBatchDoneCoveredBy(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"t1","title":"First","depends_on":[],"estimate_minutes":5,"acceptance":"First done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Second","depends_on":[],"estimate_minutes":5,"acceptance":"Second done","source_sections":["s1"]}`)

	// Close t1 normally
	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "First done completely")

	// Batch close t2 with covered_by
	ndjson := `{"id":"t2","reason":"covered by t1","covered_by":"t1"}`
	ndjsonPath := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(ndjsonPath, []byte(ndjson+"\n"), 0o600))

	stdout, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, float64(1), result["closed"])
}
