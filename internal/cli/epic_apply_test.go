package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// applyPayload writes rows as one NDJSON payload in dir and returns its path.
func applyPayload(t *testing.T, dir string, rows ...string) string {
	t.Helper()
	path := filepath.Join(dir, "rows.ndjson")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o600))
	return path
}

// loadAppliedGraph decodes dir's .tp/epics.json.
func loadAppliedGraph(t *testing.T, dir string) model.EpicGraph {
	t.Helper()
	var g model.EpicGraph
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	return g
}

// appliedEpic returns the epic id of g, failing when g holds none.
func appliedEpic(t *testing.T, g *model.EpicGraph, id string) model.Epic {
	t.Helper()
	for i := range g.Epics {
		if g.Epics[i].ID == id {
			return g.Epics[i]
		}
	}
	require.Failf(t, "epic not found", "the graph holds no epic %q", id)
	return model.Epic{}
}

// TestEpicApply_AppliesEveryRowKindInOneWrite (spec/1.7.0.md §6): an add, a
// group link, an unlink, an epic field set, a graph key set and a remove land
// in one write, whose payload carries ancestry_changed.
func TestEpicApply_AppliesEveryRowKindInOneWrite(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	payload := applyPayload(t, dir,
		`{"op":"add","epic":{"id":"e","title":"E","milestone":"A","depends_on":[{"id":"d","why":"e needs d"}]}}`,
		`{"op":"link","from":"e","any_of":["b","c"],"why":"either feeds e"}`,
		`{"op":"unlink","from":"a","to":"b"}`,
		`{"op":"set","id":"b","field":"title","value":"Renamed"}`,
		`{"op":"set","field":"gates.A.threshold","value":"ships"}`,
		`{"op":"remove","id":"a","force":false}`,
	)

	stdout, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.InDelta(t, 4, out["epics"], 0)
	assert.Contains(t, out, "ancestry_changed")
	assert.NotContains(t, out, "rows", "only --dry-run lists the rows")

	g := loadAppliedGraph(t, dir)
	assert.Len(t, g.Epics, 4)
	assert.Equal(t, []model.EpicDep{{ID: "d", Why: "e needs d"}, {AnyOf: []string{"b", "c"}, Why: "either feeds e"}},
		appliedEpic(t, &g, "e").DependsOn)
	assert.Equal(t, "Renamed", appliedEpic(t, &g, "b").Title)
	assert.Equal(t, model.EpicGate{Members: []string{"b"}, Threshold: "ships"}, g.Gates["A"])
}

// TestEpicApply_TheLastOfAnUnlinkAndALinkOfOneEdgeStands: rows apply in file
// order, so an unlink then a link of a -> b leaves the edge with the new why,
// and a link then an unlink leaves none.
func TestEpicApply_TheLastOfAnUnlinkAndALinkOfOneEdgeStands(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	payload := applyPayload(t, dir,
		`{"op":"unlink","from":"a","to":"b"}`,
		`{"op":"link","from":"a","to":"b","why":"a needs b again"}`,
	)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, []model.EpicDep{{ID: "b", Why: "a needs b again"}}, appliedEpic(t, &g, "a").DependsOn)

	payload = applyPayload(t, dir,
		`{"op":"link","from":"d","to":"c","why":"d reads c"}`,
		`{"op":"unlink","from":"d","to":"c"}`,
	)
	_, stderr, code = runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g = loadAppliedGraph(t, dir)
	assert.Empty(t, appliedEpic(t, &g, "d").DependsOn)
}

// TestEpicApply_AForwardReferenceIsJudgedOnlyByTheFinalLint: a link to an
// epic a later row adds is a reference inside a row, so it waits for the one
// lint of the after-state, which holds the epic.
func TestEpicApply_AForwardReferenceIsJudgedOnlyByTheFinalLint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	payload := applyPayload(t, dir,
		`{"op":"link","from":"d","to":"e","why":"d reads e"}`,
		`{"op":"add","epic":{"id":"e","title":"E","milestone":"A"}}`,
	)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, []model.EpicDep{{ID: "e", Why: "d reads e"}}, appliedEpic(t, &g, "d").DependsOn)
}

// TestEpicApply_AnErrorOnlyAnIntermediateStateHoldsIsNotRefused: the link
// d -> a closes the cycle a -> b -> c -> d -> a, and the next row unlinks it;
// only the after-state is linted, and it holds no cycle.
func TestEpicApply_AnErrorOnlyAnIntermediateStateHoldsIsNotRefused(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir,
		`{"op":"link","from":"d","to":"a","why":"closes a cycle"}`,
		`{"op":"unlink","from":"d","to":"a"}`,
	)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.JSONEq(t, string(before), string(readEpicGraphFile(t, dir)))
}

