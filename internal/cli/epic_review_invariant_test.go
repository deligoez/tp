package cli_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicPromptAsked returns the unit keys a prompt's "Asked units" section
// holds, read from each block's first line.
func epicPromptAsked(prompt string) []string {
	_, rest, ok := strings.Cut(prompt, "Asked units:\n")
	if !ok {
		return nil
	}
	lines := strings.Split(rest, "\n")
	var keys []string
	for i := 0; i+1 < len(lines) && lines[i] == "---"; {
		kind, id, found := strings.Cut(lines[i+1], ": ")
		if !found || !slices.Contains([]string{"epic", "gate", "scope"}, kind) {
			break
		}
		keys = append(keys, kind+":"+id)
		i++
		for i < len(lines) && lines[i] != "---" {
			i++
		}
	}
	return keys
}

// assertEveryUnitAsked is the H1 invariant: every unit an emission lists in
// units is in the asked set of at least one of its prompts.
func assertEveryUnitAsked(t *testing.T, out *epicReviewPayload) {
	t.Helper()
	asked := map[string]bool{}
	for _, p := range out.Prompts {
		for _, k := range epicPromptAsked(p.Prompt) {
			asked[k] = true
		}
	}
	for _, u := range out.Units {
		assert.True(t, asked[u], "unit %s is asked of no role; prompts %v", u, promptRoles(out))
	}
}

// TestEpicReview_AScopeUnitInAVerifyOnlyRoundGoesToTheScopeRole is the
// reproduced H1 case: a low fix on epic:a leaves a regression-only round, and
// a purpose written meanwhile changes scope:A; the scope role is asked it, so
// no unit of the round is asked of nobody.
func TestEpicReview_AScopeUnitInAVerifyOnlyRoundGoesToTheScopeRole(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, purposeRoles)
	recordEpicRows(t, dir, `{"severity":"low","finding":"a's title is terse","location":"epic:a","evidence":"title: A","role":"placer","resolved":{"status":"fixed","evidence":"retitled"}}`+"\n")
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")

	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"placer", "scope-judge", "regression"}, promptRoles(&out))
	assertEveryUnitAsked(t, &out)
}

// TestEpicReview_AScopeFixIsConfirmedByTheScopeRole: a scope row resolved
// fixed re-asks scope:A of the scope role, with the fix to confirm, and
// appends no regression role.
func TestEpicReview_AScopeFixIsConfirmedByTheScopeRole(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, purposeRoles)
	recordEpicRows(t, dir, `{"severity":"medium","finding":"the threshold needs a load test","location":"scope:A","evidence":"threshold: ten users","role":"scope-judge","resolved":{"status":"fixed","evidence":"threshold reworded"}}`+"\n")

	out := runEpicReview(t, dir, "--milestone", "A")
	require.Equal(t, []string{"scope-judge"}, promptRoles(&out))
	assert.Contains(t, out.Prompts[0].Prompt, "- scope:A: the threshold needs a load test\n")
	assertEveryUnitAsked(t, &out)
}

// TestEpicReview_APurposeEditAppendsNoRegression is M2: an open row re-asks
// epic:a and a purpose-only edit changes scope:A; regression counts only the
// epic and gate units, so it is not appended.
func TestEpicReview_APurposeEditAppendsNoRegression(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, purposeRoles)
	recordEpicRows(t, dir, `{"severity":"medium","finding":"a sits early","location":"epic:a","evidence":"title: A","role":"placer"}`+"\n")
	setGateField(t, dir, "gates.A.purpose=prove the chain end to end")

	out := runEpicReview(t, dir, "--milestone", "A")
	assert.NotContains(t, promptRoles(&out), "regression")
	assert.NotContains(t, epicSkipped(t, dir, "--milestone", "A"), "regression")
	assertEveryUnitAsked(t, &out)
}
