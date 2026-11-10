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

// TestEpicApply_AFinalStateErrorIsRefusedNamingItsPayloadLines: an edge to
// an epic no row adds is a dangling-reference of the after-state; the
// refusal names the lines of both rows naming e, and nothing is written.
func TestEpicApply_AFinalStateErrorIsRefusedNamingItsPayloadLines(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir,
		`{"op":"set","id":"a","field":"title","value":"A2"}`,
		`{"op":"add","epic":{"id":"e","title":"E","milestone":"A"}}`,
		`{"op":"link","from":"e","to":"ghost","why":"e reads ghost"}`,
	)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "dangling-reference")
	assert.Contains(t, stderr, "payload lines 2, 3", "the error names e (lines 2 and 3) and ghost (line 3)")
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused apply writes nothing")
}

// TestEpicApply_ARemoveWithoutForceLeavesItsDependentsToTheLint: removing b,
// which a depends on, fails no row; the edge a -> b it leaves dangling
// refuses the write, unless a later row unlinks it.
func TestEpicApply_ARemoveWithoutForceLeavesItsDependentsToTheLint(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	_, stderr, code := runTP(t, dir, "epic", "apply", applyPayload(t, dir, `{"op":"remove","id":"b"}`))
	require.Equal(t, 1, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "dangling-reference [a, b]")
	assert.Contains(t, stderr, "payload line 1")
	assert.Equal(t, before, readEpicGraphFile(t, dir))

	payload := applyPayload(t, dir, `{"op":"remove","id":"b"}`, `{"op":"unlink","from":"a","to":"b"}`)
	_, stderr, code = runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Len(t, g.Epics, 3)
	assert.Equal(t, []string{"a"}, g.Gates["A"].Members, "the removal drops b's gate membership")
}

// TestEpicApply_AnAbsentSubjectOrUnlinkedEdgeFailsAtItsRow: the subject of a
// link, unlink, set or remove, and the edge an unlink removes, must exist at
// that row; the refusal names the row's line, exits 4 and writes nothing. A
// subject a later row adds does not count (a remove and a later add of one
// id are refused before any row runs, so remove names an id no row adds).
func TestEpicApply_AnAbsentSubjectOrUnlinkedEdgeFailsAtItsRow(t *testing.T) {
	t.Parallel()
	add := `{"op":"add","epic":{"id":"e","title":"E","milestone":"A"}}`
	cases := map[string]struct {
		row, want string
	}{
		"link":   {`{"op":"link","from":"e","to":"a","why":"e reads a"}`, `epic "e" not found`},
		"unlink": {`{"op":"unlink","from":"e","to":"a"}`, `epic "e" not found`},
		"set":    {`{"op":"set","id":"e","field":"title","value":"E2"}`, `epic "e" not found`},
		"remove": {`{"op":"remove","id":"f"}`, `epic "f" not found`},
		"edge":   {`{"op":"unlink","from":"d","to":"a"}`, `epic "d" has no edge to "a"`},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := epicGraphDir(t, epicChainWithGate)
			before := readEpicGraphFile(t, dir)
			_, stderr, code := runTP(t, dir, "epic", "apply", applyPayload(t, dir, `{"op":"set","id":"a","field":"title","value":"A2"}`, c.row, add))
			require.Equal(t, 4, code, "stderr: %s", stderr)
			var refusal struct {
				Error string `json:"error"`
			}
			require.NoError(t, json.Unmarshal([]byte(stderr), &refusal))
			assert.Equal(t, "payload line 2: "+c.want, refusal.Error)
			assert.Equal(t, before, readEpicGraphFile(t, dir))
		})
	}
}

// TestEpicApply_ARemoveAndALaterAddOfOneIDAreRefused: add already replaces
// by id, so a payload removing d and adding it again is refused before the
// graph is read, naming both lines.
func TestEpicApply_ARemoveAndALaterAddOfOneIDAreRefused(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir,
		`{"op":"remove","id":"d","force":true}`,
		`{"op":"add","epic":{"id":"d","title":"D again","milestone":"A"}}`,
	)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "payload line 2")
	assert.Contains(t, stderr, "line 1")
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

// TestEpicApply_AnAddRowCarryingARecordIsRefused: records are not rows, so an
// add naming done is refused as --bulk refuses it, naming the line.
func TestEpicApply_AnAddRowCarryingARecordIsRefused(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir,
		`{"op":"set","id":"a","field":"title","value":"A2"}`,
		`{"op":"add","epic":{"id":"e","title":"E","milestone":"A","done":{"at":"2026-01-01T00:00:00Z","evidence":"x"}}}`,
	)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "payload line 2")
	assert.Contains(t, stderr, "names done")
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

// TestEpicApply_DryRunPrintsEachRowsActionAndWritesNothing: --dry-run prints
// the payload a write would, the file's current revision, and each row's
// action, and leaves the file byte-identical.
func TestEpicApply_DryRunPrintsEachRowsActionAndWritesNothing(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir,
		`{"op":"add","epic":{"id":"e","title":"E","milestone":"A"}}`,
		`{"op":"add","epic":{"id":"d","title":"D2","milestone":"A"}}`,
		`{"op":"link","from":"e","to":"d","why":"e needs d"}`,
		`{"op":"link","from":"a","to":"b","why":"a needs b, reworded"}`,
		`{"op":"unlink","from":"c","to":"d"}`,
		`{"op":"set","field":"milestones","value":[["A", "B"], ["L1"]]}`,
		`{"op":"remove","id":"e","force":true}`,
	)
	stdout, stderr, code := runTP(t, dir, "epic", "apply", payload, "--dry-run")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, before, readEpicGraphFile(t, dir), "--dry-run writes nothing")
	var out struct {
		Epics    int    `json:"epics"`
		Revision string `json:"revision"`
		Rows     []struct {
			Line   int    `json:"line"`
			Op     string `json:"op"`
			Action string `json:"action"`
		} `json:"rows"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, 4, out.Epics)
	lintOut, _, _ := runTP(t, dir, "epic", "lint")
	require.NotEmpty(t, out.Revision)
	assert.Contains(t, lintOut, out.Revision, "the revision is the unchanged file's")
	var actions []string
	for _, r := range out.Rows {
		actions = append(actions, r.Op+":"+r.Action)
	}
	assert.Equal(t, []string{"add:added", "add:replaced", "link:added", "link:replaced", "unlink:removed", "set:set", "remove:removed"}, actions)
	assert.Equal(t, 7, out.Rows[6].Line)
}

// TestEpicApply_DryRunPrintsTheWritersPayloadAndEachRow is spec/1.7.0.md §6's
// "--dry-run prints it and each row's action": every key the write itself
// prints, ancestry_changed included, beside rows — and the flag's help says
// so, rather than a "payload" a reader takes for the input rows.
func TestEpicApply_DryRunPrintsTheWritersPayloadAndEachRow(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	payload := applyPayload(t, dir, `{"op":"set","id":"a","field":"title","value":"A2"}`)
	decode := func(args ...string) map[string]json.RawMessage {
		stdout, stderr, code := runTP(t, dir, append([]string{"epic", "apply", payload}, args...)...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var out map[string]json.RawMessage
		require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
		return out
	}
	dry := decode("--dry-run")
	assert.Contains(t, dry, "rows")
	for key := range decode() {
		assert.Contains(t, dry, key, "--dry-run prints the write's %s", key)
	}
	help, _, code := runTP(t, dir, "epic", "apply", "--help")
	require.Equal(t, 0, code)
	assert.Contains(t, help, "print the write's payload (file, epics, dropped_keys, revision, ancestry_changed) and each row's action")
}

// TestEpicApply_IfRevisionRefusesAMovedGraph: a stale --if-revision exits 4
// and writes nothing.
func TestEpicApply_IfRevisionRefusesAMovedGraph(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir, `{"op":"set","id":"a","field":"title","value":"A2"}`)
	_, stderr, code := runTP(t, dir, "epic", "apply", payload, "--if-revision", "0000000000000000")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "revision")
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

// TestEpicApply_UnattendedEscalatesAsOther: under TP_UNATTENDED the apply is
// fenced like every graph writer, and the same payload applies attended.
func TestEpicApply_UnattendedEscalatesAsOther(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainWithGate)
	before := readEpicGraphFile(t, dir)
	payload := applyPayload(t, dir, `{"op":"set","id":"a","field":"title","value":"A2"}`)
	_, stderr, code := runTPFence(t, dir, true, "epic", "apply", payload)
	require.Equal(t, 2, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "tp escalate --decision other")
	assert.Equal(t, before, readEpicGraphFile(t, dir))

	_, stderr, code = runTPFence(t, dir, false, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}
