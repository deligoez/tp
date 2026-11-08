package cli_test

import (
	"os"
	"path/filepath"
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

