package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicReview_OutputContractPrecedesTheContext: on a large slice the
// context dwarfs everything else, and a reader that stopped before the end
// wrote rows with no severity. The contract comes before the context and the
// prompt's last line repeats the required keys and the path.
func TestEpicReview_OutputContractPrecedesTheContext(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--epic", "b")
	require.NotEmpty(t, out.Prompts)
	for _, p := range out.Prompts {
		contract := strings.Index(p.Prompt, "Output contract")
		context := strings.Index(p.Prompt, "Context —")
		require.NotEqual(t, -1, contract, "%s has a contract", p.Role)
		require.NotEqual(t, -1, context, "%s has a context", p.Role)
		assert.Less(t, contract, context, "%s: the contract comes before the context", p.Role)

		lines := strings.Split(strings.TrimRight(p.Prompt, "\n"), "\n")
		last := lines[len(lines)-1]
		for _, key := range []string{"severity", "finding", "location", "evidence", p.OutputPath} {
			assert.Contains(t, last, key, "%s: the last line names %s", p.Role, key)
		}
	}
}

// TestEpicReview_DependencyScopedRolesSeeNoDependents: a role whose file says
// "context": "dependencies" is shown only the dependencies of the asked units;
// a role without it still sees both directions. For --epic b, a depends on b
// and c is b's dependency.
func TestEpicReview_DependencyScopedRolesSeeNoDependents(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--epic", "b")
	byRole := map[string]string{}
	for _, p := range out.Prompts {
		byRole[p.Role] = p.Prompt
	}
	for _, role := range []string{"dod-satisfiability", "why-auditor"} {
		assert.Contains(t, byRole[role], "epic: c\n", "%s sees the dependency", role)
		assert.NotContains(t, byRole[role], "epic: a\n", "%s sees no dependent", role)
	}
	assert.Contains(t, byRole["phase-economist"], "epic: a\n", "phase-economist still sees dependents")

	rolesDir := filepath.Join(dir, ".tp", "epic-reviewers")
	require.NoError(t, os.Mkdir(rolesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rolesDir, "edge-reader.json"),
		[]byte(`{"id": "edge-reader", "title": "Edge reader", "instructions": "Read every edge.", "context": "sideways"}`), 0o600))
	_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b")
	assert.Equal(t, 3, code, "an unknown context value is a bad role file")
	assert.Contains(t, stderr, "context")
}

// epicBriefChain is epicChain with acceptance text and a second edge on the
// dependent a, so a brief of a can be told from its full block.
const epicBriefChain = `{
  "version": 1,
  "milestones": [["A", "B"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "acceptance": "a ships the report",
     "depends_on": [{"id": "b", "why": "a needs b"}, {"id": "d", "why": "a also needs d"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "acceptance": "c parses input"},
    {"id": "d", "title": "D", "milestone": "A"}
  ]
}
`

// epicRolePrompt returns the prompt the named role was given.
func epicRolePrompt(t *testing.T, out *epicReviewPayload, role string) string {
	t.Helper()
	for _, p := range out.Prompts {
		if p.Role == role {
			return p.Prompt
		}
	}
	require.Failf(t, "no prompt", "role %s emitted no prompt", role)
	return ""
}

// TestEpicReview_PhaseEconomistSeesDependentsInBrief (spec/1.7.0.md §4): the
// shipped phase-economist reads each dependency's full block and each
// dependent as its id, title, milestone and its edge into the asked unit —
// not the dependent's acceptance or its other edges.
func TestEpicReview_PhaseEconomistSeesDependentsInBrief(t *testing.T) {
	t.Parallel()
	out := runEpicReview(t, epicGraphDir(t, epicBriefChain), "--epic", "b")
	prompt := epicRolePrompt(t, &out, "phase-economist")
	assert.Contains(t, prompt, "epic: c\ntitle: C\nmilestone: A\nacceptance: c parses input\n", "the dependency in full")
	assert.Contains(t, prompt, "epic: a\ntitle: A\nmilestone: A\n", "the dependent's id, title and milestone")
	assert.Contains(t, prompt, "- b: a needs b\n", "the dependent's edge into the asked unit, with its why")
	assert.NotContains(t, prompt, "a ships the report", "no dependent acceptance")
	assert.NotContains(t, prompt, "a also needs d", "no dependent edge outside the asked units")
}

// TestEpicReview_RegressionRoundsKeepThePhaseEconomistBrief (spec/1.7.0.md
// §4): a round that appends the regression role still gives the
// phase-economist its dependents in brief; only the regression role's own
// prompt reads every neighbour in full.
func TestEpicReview_RegressionRoundsKeepThePhaseEconomistBrief(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicBriefChain)
	r1 := runEpicReview(t, dir, "--epic", "b")
	writeEpicRound(t, dir, &r1, "")

	editEpicGraph(t, dir, "b needs c", "b needs c's parser")
	r2 := runEpicReview(t, dir, "--epic", "b")
	require.Contains(t, promptRoles(&r2), "regression")
	economist := epicRolePrompt(t, &r2, "phase-economist")
	assert.Contains(t, economist, "- b: a needs b\n", "the dependent's edge, in brief")
	assert.NotContains(t, economist, "a ships the report", "no dependent acceptance in a regression round either")
	assert.Contains(t, epicRolePrompt(t, &r2, "regression"), "a ships the report", "the regression role keeps the full neighbours")
}

// TestEpicReview_ACarriedGateStaysVisible: a carried gate is not asked, but its
// block stays in the prompt as read-only context for every role that takes
// gate units; phase-economist takes only epics, so it is not shown one.
func TestEpicReview_ACarriedGateStaysVisible(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, strings.Replace(epicChain, `"epics": [`,
		`"gates": {"A": {"members": ["d"], "deferrable": ["c"]}},
  "epics": [`, 1))
	r1 := runEpicReview(t, dir, "--milestone", "A")
	require.Contains(t, r1.Units, "gate:A")
	writeEpicRound(t, dir, &r1, "")

	editEpicGraph(t, dir, "b needs c", "b needs c's parser")
	r2 := runEpicReview(t, dir, "--milestone", "A")
	require.Contains(t, r2.Carried, "gate:A")
	require.NotEmpty(t, r2.Prompts)
	for _, p := range r2.Prompts {
		if p.Role == "phase-economist" {
			assert.NotContains(t, p.Prompt, "Carried gate", "phase-economist takes no gate units")
			continue
		}
		assert.Contains(t, p.Prompt, "deferrable: c\n", "%s sees the carried gate's block", p.Role)
	}
}
