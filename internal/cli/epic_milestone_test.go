package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func epicTracks(t *testing.T, dir string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)
	var g struct {
		Milestones [][]string `json:"milestones"`
	}
	require.NoError(t, json.Unmarshal(data, &g))
	return g.Milestones
}

// TestEpicMilestoneAdd_InsertsWhereAsked: --after A puts X right after A in
// A's track, --before B right before B, and --new-track opens its own track.
// An insertion before A instead of after it fails the first step.
func TestEpicMilestoneAdd_InsertsWhereAsked(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	_, stderr, code := runTP(t, dir, "epic", "milestone", "add", "X", "--after", "A")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, [][]string{{"A", "X", "B"}, {"L1"}}, epicTracks(t, dir))

	_, stderr, code = runTP(t, dir, "epic", "milestone", "add", "Y", "--before", "B")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, [][]string{{"A", "X", "Y", "B"}, {"L1"}}, epicTracks(t, dir))

	stdout, stderr, code := runTP(t, dir, "epic", "milestone", "add", "Z", "--new-track")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, [][]string{{"A", "X", "Y", "B"}, {"L1"}, {"Z"}}, epicTracks(t, dir))
	assert.Equal(t, epicFileRevision(t, dir), epicRevisionOf(t, stdout), "it prints the writers' payload")
}

// TestEpicMilestoneAdd_RefusalsTakeTheirExitCodes: no position flag or two
// are usage errors (2); an absent anchor is 4; a name the file already holds
// is 1, refused by its own check. None writes.
func TestEpicMilestoneAdd_RefusalsTakeTheirExitCodes(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	before := epicFileRevision(t, dir)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"epic", "milestone", "add", "X"}, 2},
		{[]string{"epic", "milestone", "add", "X", "--after", "A", "--new-track"}, 2},
		{[]string{"epic", "milestone", "add", "X", "--after", "Q"}, 4},
		{[]string{"epic", "milestone", "add", "B", "--new-track"}, 1},
	} {
		_, stderr, code := runTP(t, dir, tc.args...)
		assert.Equal(t, tc.code, code, "%v: stderr: %s", tc.args, stderr)
	}
	assert.Equal(t, before, epicFileRevision(t, dir), "no refused insert writes")
}
