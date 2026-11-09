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

// TestEpicReview_ACarriedGateStaysVisible: a carried gate is not asked, but its
// deferrable list is what a placement reader needs to see, so its block stays
// in the prompt as read-only context.
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
		assert.Contains(t, p.Prompt, "deferrable: c\n", "%s sees the carried gate's block", p.Role)
	}
}
