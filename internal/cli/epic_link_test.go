package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicEdges returns from's depends_on as id → why, read from the file.
func epicEdges(t *testing.T, dir, from string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)
	var g struct {
		Epics []struct {
			ID        string `json:"id"`
			DependsOn []struct {
				ID  string `json:"id"`
				Why string `json:"why"`
			} `json:"depends_on"`
		} `json:"epics"`
	}
	require.NoError(t, json.Unmarshal(data, &g))
	edges := map[string]string{}
	for _, e := range g.Epics {
		if e.ID != from {
			continue
		}
		for _, d := range e.DependsOn {
			require.NotContains(t, edges, d.ID, "%s -> %s appears twice", from, d.ID)
			edges[d.ID] = d.Why
		}
	}
	return edges
}

func epicActionOf(t *testing.T, stdout string) string {
	t.Helper()
	var out struct {
		Action string `json:"action"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out.Action
}

// TestEpicLink_AddsReplacesAndRemovesOneEdge: link adds a -> c, then
// rewording it replaces that edge's why rather than appending a second edge;
// unlink removes it. Each payload says which happened.
func TestEpicLink_AddsReplacesAndRemovesOneEdge(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	stdout, stderr, code := runTP(t, dir, "epic", "link", "a", "c", "a renders c's report")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "added", epicActionOf(t, stdout))
	assert.Equal(t, map[string]string{"b": "a needs b", "c": "a renders c's report"}, epicEdges(t, dir, "a"))

	stdout, stderr, code = runTP(t, dir, "epic", "link", "a", "c", "a prints c's totals")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "replaced", epicActionOf(t, stdout))
	assert.Equal(t, map[string]string{"b": "a needs b", "c": "a prints c's totals"}, epicEdges(t, dir, "a"))

	stdout, stderr, code = runTP(t, dir, "epic", "unlink", "a", "c")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "removed", epicActionOf(t, stdout))
	assert.Equal(t, map[string]string{"b": "a needs b"}, epicEdges(t, dir, "a"))
}

// TestEpicLink_RefusalsTakeTheirExitCodes: an absent source is the edited
// epic missing (4); an absent target and an empty why are lint errors (1);
// unlinking an edge that does not exist is 4. None writes.
func TestEpicLink_RefusalsTakeTheirExitCodes(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	before := epicFileRevision(t, dir)
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"epic", "link", "zz", "a", "zz needs a"}, 4},
		{[]string{"epic", "link", "a", "zz", "a needs zz"}, 1},
		{[]string{"epic", "link", "a", "c", "  "}, 1},
		{[]string{"epic", "unlink", "a", "c"}, 4},
	} {
		_, stderr, code := runTP(t, dir, tc.args...)
		assert.Equal(t, tc.code, code, "%v: stderr: %s", tc.args, stderr)
	}
	assert.Equal(t, before, epicFileRevision(t, dir), "no refused link writes")
}
