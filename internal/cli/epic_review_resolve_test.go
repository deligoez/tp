package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicHighRow is one open high finding on b, the unit of --epic b.
const epicHighRow = `{"severity":"high","finding":"b's acceptance names no checker","location":"epic:b","evidence":"b"}` + "\n"

// TestEpicReviewResolve_AWontfixWithEvidenceMakesTheRoundClean: an open high
// finding holds round 1 open; resolving it wontfix with evidence through
// --resolve writes the disposition into the recorded round file, and --status
// then counts that round clean. A resolve that writes nowhere, or a status
// that ignores dispositions, fails it.
func TestEpicReviewResolve_AWontfixWithEvidenceMakesTheRoundClean(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	slice := []string{"--epic", "b"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, epicHighRow, slice...)
	require.Equal(t, 0, epicStatus(t, dir, slice...).ConsecutiveClean, "the open high finding holds the round open")

	_, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...),
		"--resolve", "0", "wontfix", "the checker ships in a later milestone")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	files := epicRoundFiles(t, dir)
	require.Len(t, files, 1)
	data, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Contains(t, string(data), `"status":"wontfix"`, "the disposition lands in the recorded round file")
	st := epicStatus(t, dir, slice...)
	assert.Equal(t, 1, st.ConsecutiveClean, "the resolved round is clean")
	assert.Equal(t, 1, st.Round)
}

// TestEpicReviewResolve_AnOutOfRangeIndexChangesNothing: index 1 on a
// one-row round exits non-zero and leaves the round file byte-identical.
// Clamping the index, or writing before the range check, fails it.
func TestEpicReviewResolve_AnOutOfRangeIndexChangesNothing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	slice := []string{"--epic", "b"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, epicHighRow, slice...)
	files := epicRoundFiles(t, dir)
	require.Len(t, files, 1)
	before, err := os.ReadFile(files[0])
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...),
		"--resolve", "1", "wontfix", "out of range")...)
	assert.NotEqual(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "out of range")

	after, err := os.ReadFile(files[0])
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "the round file is unchanged")
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(files[0]), "*"))
	require.NoError(t, err)
	assert.Len(t, matches, 4, "only round-1.ndjson, its snapshot, the slice's key.json and the inbox the emission made exist")
}

// TestEpicReviewResolve_SeveralIndicesAtOnce: --resolve 0,2 dispositions both
// rows and leaves row 1 open; --resolve 0,9 names an index out of range and
// changes no row at all. Resolving the valid indices before refusing the bad
// one fails the second half.
func TestEpicReviewResolve_SeveralIndicesAtOnce(t *testing.T) {
	t.Parallel()
	threeRows := `{"severity":"low","finding":"one","location":"epic:b","evidence":"b"}` + "\n" +
		`{"severity":"low","finding":"two","location":"epic:b","evidence":"b"}` + "\n" +
		`{"severity":"low","finding":"three","location":"epic:b","evidence":"b"}` + "\n"
	slice := []string{"--epic", "b"}
	resolve := func(dir string, args ...string) (string, int) {
		_, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, slice...), args...)...)
		return stderr, code
	}

	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, threeRows, slice...)
	stderr, code := resolve(dir, "--resolve", "0,2", "wontfix", "decided")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	data, err := os.ReadFile(epicRoundFiles(t, dir)[0])
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], `"wontfix"`)
	assert.NotContains(t, lines[1], `"resolved"`, "row 1 was not listed")
	assert.Contains(t, lines[2], `"wontfix"`)

	bad := epicGraphDir(t, epicChain)
	runEpicReview(t, bad, slice...)
	recordEpicRound(t, bad, threeRows, slice...)
	before, err := os.ReadFile(epicRoundFiles(t, bad)[0])
	require.NoError(t, err)
	stderr, code = resolve(bad, "--resolve", "0,9", "wontfix", "decided")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
	after, err := os.ReadFile(epicRoundFiles(t, bad)[0])
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "one bad index changes no row")
}
