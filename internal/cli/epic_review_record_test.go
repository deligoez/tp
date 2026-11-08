package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicRecordFile writes a findings file outside .tp/ and returns its path.
func epicRecordFile(t *testing.T, dir, rows string) string {
	t.Helper()
	path := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(path, []byte(rows), 0o600))
	return path
}

// epicRoundFiles lists every recorded round file under .tp/epic-review/.
func epicRoundFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".tp", "epic-review", "*", "round-*.ndjson"))
	require.NoError(t, err)
	return files
}

// TestEpicReviewRecord_RowsOutsideTheSliceRefuseTheFile is §9 row 8: over
// --epic b, a row on epic:d and one on gate:A name units the slice does not
// hold, so the whole file is refused at exit 1 naming both lines and no round
// is written. Accepting any id, or naming only the first bad line, fails it.
func TestEpicReviewRecord_RowsOutsideTheSliceRefuseTheFile(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--epic", "b")
	path := epicRecordFile(t, dir,
		`{"severity":"low","finding":"f1","location":"epic:b","evidence":"e1"}`+"\n"+
			`{"severity":"low","finding":"f2","location":"epic:d","evidence":"e2"}`+"\n"+
			`{"severity":"low","finding":"f3","location":"gate:A","evidence":"e3"}`+"\n")

	_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--record", path)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "line 2")
	assert.Contains(t, stderr, "line 3")
	assert.NotContains(t, stderr, "line 1:")
	assert.Empty(t, epicRoundFiles(t, dir), "a refused file writes no round")
}

// TestEpicReviewRecord_UnattendedExitsTwoAndWritesNothing is §9 row 12 for
// --record: a legal one-row file over --epic b is refused under TP_UNATTENDED
// and no state directory is opened.
func TestEpicReviewRecord_UnattendedExitsTwoAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	path := epicRecordFile(t, dir, `{"severity":"low","finding":"f1","location":"epic:b","evidence":"e1"}`+"\n")

	_, stderr, code := runTPFence(t, dir, true, "epic", "review", "--epic", "b", "--record", path)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "tp escalate --decision other")
	_, err := os.Stat(filepath.Join(dir, ".tp", "epic-review"))
	assert.True(t, os.IsNotExist(err), "a refused record writes nothing")
}

