package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicGateFieldGraph is track [A, B] with a1 and a2 in A, b in B, and gates
// as written (an empty gates leaves the key out).
func epicGateFieldGraph(gates string) string {
	g := `{"version": 1, "milestones": [["A", "B"]],`
	if gates != "" {
		g += ` "gates": ` + gates + `,`
	}
	return g + ` "epics": [
    {"id": "a1", "title": "A1", "milestone": "A", "root": true},
    {"id": "a2", "title": "A2", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "B", "root": true}
  ]}`
}

// epicGateMet is a met record judged against "p95 under 200ms".
const epicGateMet = `{"at": "2026-01-02T03:04:05Z", "evidence": "dashboard", "threshold": "p95 under 200ms"}`

// gateOnFile is one gate as the graph file holds it; Members is a pointer so
// an absent or null list tells apart from [].
type gateOnFile struct {
	Members    *[]string       `json:"members"`
	Deferrable []string        `json:"deferrable"`
	Threshold  string          `json:"threshold"`
	Met        json.RawMessage `json:"met"`
}

// gatesOnFile reads the graph file's gates.
func gatesOnFile(t *testing.T, dir string) map[string]gateOnFile {
	t.Helper()
	var g struct {
		Gates map[string]gateOnFile `json:"gates"`
	}
	require.NoError(t, json.Unmarshal(readEpicGraphFile(t, dir), &g))
	return g.Gates
}

// setGateField runs tp epic set with arg and requires exit 0.
func setGateField(t *testing.T, dir string, arg ...string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, append([]string{"epic", "set"}, arg...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestEpicSet_GateThresholdEditsOneGateAndKeepsItsMet: the threshold changes,
// the met record and the members stay, and the other gate is untouched.
func TestEpicSet_GateThresholdEditsOneGateAndKeepsItsMet(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicGateFieldGraph(`{"A": {"members": ["a1"], "threshold": "p95 under 200ms", "met": `+epicGateMet+`}, "B": {"members": [], "threshold": "ships"}}`))
	setGateField(t, dir, "gates.A.threshold=p95 under 150ms")
	gates := gatesOnFile(t, dir)
	assert.Equal(t, "p95 under 150ms", gates["A"].Threshold)
	assert.JSONEq(t, epicGateMet, string(gates["A"].Met), "met is carried, its threshold copy shows the edit")
	require.NotNil(t, gates["A"].Members)
	assert.Equal(t, []string{"a1"}, *gates["A"].Members)
	assert.Equal(t, "ships", gates["B"].Threshold)
}

// TestEpicSet_GateDeferrableCreatesTheEntryAndAnEmptyValueClearsIt: a gate
// with no entry gains one holding only the deferrable list; an empty value
// removes the list again.
func TestEpicSet_GateDeferrableCreatesTheEntryAndAnEmptyValueClearsIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicGateFieldGraph(""))
	setGateField(t, dir, "gates.A.deferrable=a2,a1")
	gate := gatesOnFile(t, dir)["A"]
	assert.Equal(t, []string{"a2", "a1"}, gate.Deferrable)
	require.NotNil(t, gate.Members, "a created entry writes members: [], the implicit list")
	assert.Empty(t, *gate.Members)

	setGateField(t, dir, "gates.A.deferrable=")
	assert.Empty(t, gatesOnFile(t, dir)["A"].Deferrable)
}

// TestEpicSet_GateMembersSetsAnExplicitListAndAnEmptyValueMakesItImplicit:
// members names the gate's epics; an empty value writes [], the milestone's
// work epics, and keeps the deferrable list.
func TestEpicSet_GateMembersSetsAnExplicitListAndAnEmptyValueMakesItImplicit(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicGateFieldGraph(`{"A": {"members": [], "deferrable": ["a2"]}}`))
	setGateField(t, dir, "gates.A.members=a1")
	gate := gatesOnFile(t, dir)["A"]
	require.NotNil(t, gate.Members)
	assert.Equal(t, []string{"a1"}, *gate.Members)
	assert.Equal(t, []string{"a2"}, gate.Deferrable)

	setGateField(t, dir, "gates.A.members=")
	gate = gatesOnFile(t, dir)["A"]
	require.NotNil(t, gate.Members)
	assert.Empty(t, *gate.Members)
	assert.Equal(t, []string{"a2"}, gate.Deferrable)
}

// TestEpicSet_GateFieldOfAnUnknownMilestoneExits4: Z is in no track, so the
// set exits 4 as other milestone refusals do and writes nothing.
func TestEpicSet_GateFieldOfAnUnknownMilestoneExits4(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicGateFieldGraph(""))
	before := readEpicGraphFile(t, dir)
	_, stderr, code := runTP(t, dir, "epic", "set", "gates.Z.threshold=ships")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, `milestone \"Z\" is in no track`)
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

