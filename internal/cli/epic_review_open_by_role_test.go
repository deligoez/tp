package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicOpenByRoleRound is one recorded round of the --epic b slice: two
// findings from why-auditor, one of them resolved wontfix, and one from
// phase-economist, so two stay open, one under each role.
const epicOpenByRoleRound = `{"role":"why-auditor","severity":"high","finding":"f1","location":"epic:b","evidence":"b"}
{"role":"why-auditor","severity":"medium","finding":"f2","location":"epic:b","evidence":"b","resolved":{"status":"wontfix","evidence":"the why is the operator's"}}
{"role":"phase-economist","severity":"low","finding":"f3","location":"epic:b","evidence":"b"}
`

// epicOpenByRole is the part of --status, and of each batch row, these
// tests read.
type epicOpenByRole struct {
	Open       map[string]int `json:"open"`
	OpenByRole map[string]int `json:"open_by_role"`
}

// sumCounts adds a count map's values.
func sumCounts(m map[string]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// TestEpicReviewStatus_CountsOpenFindingsByRole is spec/1.8.0.md §5's open
// findings by role: the last round's undisposed findings counted per role,
// summing to the open total, and the wontfix finding under no role. A count
// that takes the disposed finding too, or a missing open_by_role, fails it.
func TestEpicReviewStatus_CountsOpenFindingsByRole(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	runEpicReview(t, dir, "--epic", "b")
	recordEpicRound(t, dir, epicOpenByRoleRound, "--epic", "b")

	stdout, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var st epicOpenByRole
	require.NoError(t, json.Unmarshal([]byte(stdout), &st), "stdout: %s", stdout)
	assert.Equal(t, map[string]int{"why-auditor": 1, "phase-economist": 1}, st.OpenByRole)
	assert.Equal(t, sumCounts(st.Open), sumCounts(st.OpenByRole), "the by-role counts sum to the open total")

	stdout, stderr, code = runTP(t, dir, "epic", "review", "--all", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var batch struct {
		Slices []epicOpenByRole `json:"slices"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &batch), "stdout: %s", stdout)
	require.Len(t, batch.Slices, 1)
	assert.Equal(t, st.OpenByRole, batch.Slices[0].OpenByRole, "each batch row carries the slice's own counts")
}
