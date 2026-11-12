package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// purposeRoles is a corpus of a scope role and an epic role that both take
// the purpose, and a plain role that takes neither the scope nor the purpose.
var purposeRoles = map[string]string{
	"placer":      `{"id": "placer", "title": "Placer", "instructions": "Judge where each epic sits.", "units": ["epics"], "purpose": true}`,
	"plain":       `{"id": "plain", "title": "Plain", "instructions": "Read the epics and the gate."}`,
	"scope-judge": `{"id": "scope-judge", "title": "Scope judge", "instructions": "Judge the gate whole.", "units": ["scope"], "purpose": true}`,
}

// epicPurposeStatus is the part of A's --status payload these tests read.
type epicPurposeStatus struct {
	Converged    bool     `json:"converged"`
	Done         bool     `json:"done"`
	Stale        bool     `json:"stale"`
	StaleUnits   []string `json:"stale_units"`
	PurposeStale []string `json:"purpose_stale_units"`
}

func epicPurposeStatusOf(t *testing.T, dir string) (st epicPurposeStatus, code int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--status", "--check")
	require.Contains(t, []int{0, 1}, code, "stderr: %s", stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &st), stdout)
	return st, code
}

// recordEpicRows emits A's next round and records rows as it.
func recordEpicRows(t *testing.T, dir, rows string) {
	t.Helper()
	runEpicReview(t, dir, "--milestone", "A")
	recordEpicRound(t, dir, rows, "--milestone", "A")
}

// convergedPurposeSlice is milestone A of epicChainThreshold under corpus,
// converged by two clean rounds.
func convergedPurposeSlice(t *testing.T, corpus map[string]string) string {
	t.Helper()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, corpus)
	recordEpicRows(t, dir, "")
	recordEpicRows(t, dir, "")
	st, code := epicPurposeStatusOf(t, dir)
	require.Equal(t, 0, code, "the slice converged: %+v", st)
	return dir
}

// TestEpicReview_APurposeReasksOnlyThePurposeRoles: writing a purpose on a
// converged slice leaves it pending in purpose_stale_units, asks only the roles
// with purpose: true, each its own kinds, delivers the purpose once as a
// header, and skips the rest with no-units; that round records and closes,
// and an unchanged purpose asks nothing again.
func TestEpicReview_APurposeReasksOnlyThePurposeRoles(t *testing.T) {
	t.Parallel()
	dir := convergedPurposeSlice(t, purposeRoles)
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")

	st, code := epicPurposeStatusOf(t, dir)
	assert.Equal(t, 1, code, "--check fails while the purpose re-ask is pending")
	assert.False(t, st.Converged)
	assert.Equal(t, []string{"scope:A"}, st.StaleUnits, "the purpose is part of the scope fingerprint")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d"}, st.PurposeStale, "the placer's epics wait on the purpose")

	batch, _, _ := runTP(t, dir, "epic", "review", "--all", "--status")
	assert.Contains(t, batch, `"purpose_stale_units": [`, "the batch row shows it too")
	export, _, _ := runTP(t, dir, "epic", "export")
	assert.Contains(t, export, `"purpose_stale_units": [`, "tp epic export's slices carry it")

	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"placer", "scope-judge"}, promptRoles(&out), "no regression: a purpose change is no unit change")
	assert.Equal(t, map[string]string{"plain": "no-units"}, epicSkipped(t, dir, "--milestone", "A"), "a scope change appends no regression")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "scope:A"}, out.Units)
	placer := out.Prompts[0].Prompt
	assert.Contains(t, placer, "\nMilestone purposes:\n- A: prove the chain end to end\n")
	assert.Equal(t, 1, strings.Count(placer, "prove the chain end to end"), "the purpose is delivered once")
	assert.NotContains(t, placer, "scope: A\n", "the placer is not asked the scope unit")

	recordEpicRows(t, dir, `{"severity":"low","finding":"a sits early for the purpose","location":"epic:a","evidence":"title: A","role":"placer","resolved":{"status":"wontfix","evidence":"kept by choice"}}`+"\n")
	st, code = epicPurposeStatusOf(t, dir)
	assert.Equal(t, 0, code, "the round recorded and closed: %+v", st)
	assert.Empty(t, st.PurposeStale)

	again := runEpicReview(t, dir, "--milestone", "A")
	assert.Empty(t, again.Prompts, "an unchanged purpose asks nothing")
	assert.Empty(t, again.Units)
}

// TestEpicReview_ACorpusWithoutAPurposeRoleIsNotReasked: with no role taking
// the scope or the purpose, a gate with a threshold has no scope unit, the
// slice converges, and writing a purpose re-asks nothing and leaves nothing
// stale.
func TestEpicReview_ACorpusWithoutAPurposeRoleIsNotReasked(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, map[string]string{"plain": purposeRoles["plain"]})
	first := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A"}, first.Units, "no role reads a scope unit, so the slice holds none")
	dir = convergedPurposeSlice(t, map[string]string{"plain": purposeRoles["plain"]})
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")

	st, code := epicPurposeStatusOf(t, dir)
	assert.Equal(t, 0, code, "still converged: %+v", st)
	assert.Empty(t, st.PurposeStale)
	assert.Empty(t, st.StaleUnits)
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Empty(t, out.Prompts)
	assert.Empty(t, out.Units)
}

// TestEpicReview_APurposeReaskCarriesItsDecisions: a unit re-asked only for a
// changed purpose still shows its earlier wontfix decision, and a row on it
// records.
func TestEpicReview_APurposeReaskCarriesItsDecisions(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, purposeRoles)
	recordEpicRows(t, dir, `{"severity":"medium","finding":"a sits early","location":"epic:a","evidence":"title: A","role":"placer","resolved":{"status":"wontfix","evidence":"kept by choice"}}`+"\n")
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")

	out := runEpicReview(t, dir, "--milestone", "A")
	require.Equal(t, "placer", out.Prompts[0].Role)
	assert.Contains(t, out.Prompts[0].Prompt, "Already decided")
	assert.Contains(t, out.Prompts[0].Prompt, "- epic:a: a sits early — round 1, wontfix: kept by choice\n")

	rows := filepath.Join(t.TempDir(), "rows.ndjson")
	require.NoError(t, os.WriteFile(rows, []byte(`{"severity":"low","finding":"b reads late for the purpose","location":"epic:b","evidence":"title: B","role":"placer"}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record", rows)
	assert.Equal(t, 0, code, "a row on a purpose-stale unit records: %s", stderr)
}

// TestEpicReview_RemovingAPurposeReasks: a purpose removed is a purpose
// changed, so the purpose roles are asked again.
func TestEpicReview_RemovingAPurposeReasks(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, purposeRoles)
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")
	recordEpicRows(t, dir, "")
	recordEpicRows(t, dir, "")
	setGateField(t, dir, "gates.A.purpose=")

	st, _ := epicPurposeStatusOf(t, dir)
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d"}, st.PurposeStale)
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"placer", "scope-judge"}, promptRoles(&out))
	assert.NotContains(t, out.Prompts[0].Prompt, "Milestone purposes:", "no purpose is left to show")
}
