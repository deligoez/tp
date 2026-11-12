package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicChainThreshold is epicChain with A's gate naming a and b and a
// threshold, so a milestone slice asks gate:A and scope:A.
const epicChainThreshold = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "gates": {"A": {"members": ["a", "b"], "threshold": "ten users"}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "acceptance": "a ships", "depends_on": [{"id": "b", "why": "a needs b"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "c needs d"}]},
    {"id": "d", "title": "D", "milestone": "A"}
  ]
}
`

// writeEpicRoles writes each role file body under .tp/epic-reviewers/,
// keyed by role id.
func writeEpicRoles(t *testing.T, dir string, roles map[string]string) {
	t.Helper()
	rolesDir := filepath.Join(dir, ".tp", "epic-reviewers")
	require.NoError(t, os.MkdirAll(rolesDir, 0o755))
	for id, body := range roles {
		require.NoError(t, os.WriteFile(filepath.Join(rolesDir, id+".json"), []byte(body), 0o600))
	}
}

// epicSkipped reads an emission's skipped_roles as role -> reason.
func epicSkipped(t *testing.T, dir string, args ...string) map[string]string {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Skipped []struct {
			Role   string `json:"role"`
			Reason string `json:"reason"`
		} `json:"skipped_roles"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	got := map[string]string{}
	for _, s := range out.Skipped {
		got[s.Role] = s.Reason
	}
	return got
}

// scopeAndPlainRoles is a corpus of one scope-only role and one role with no
// units key, which keeps today's epics and gates.
var scopeAndPlainRoles = map[string]string{
	"plain":      `{"id": "plain", "title": "Plain", "instructions": "Read the epics and the gate."}`,
	"scope-only": `{"id": "scope-only", "title": "Scope only", "instructions": "Judge the gate whole.", "units": ["scope"]}`,
}

// TestEpicReview_EachRoleIsAskedOnlyItsUnitKinds: a role is asked the units
// of the kinds it lists, absent units meaning epics and gates, so a corpus
// never receives a scope unit it was not written for.
func TestEpicReview_EachRoleIsAskedOnlyItsUnitKinds(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, scopeAndPlainRoles)

	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A", "scope:A"}, out.Units)
	require.Equal(t, []string{"plain", "scope-only"}, promptRoles(&out))
	plain, scope := out.Prompts[0].Prompt, out.Prompts[1].Prompt
	assert.Contains(t, plain, "gate: A\n")
	assert.NotContains(t, plain, "scope: A\n", "a role without units is never asked a scope unit")
	assert.Contains(t, scope, "scope: A\n")
	assert.NotContains(t, scope, "\n---\nepic: a\n", "a scope-only role is asked no epic")
	assert.NotContains(t, scope, "No gate unit is asked", "a role without the gates kind is not told about gates")
	assert.Contains(t, scope, `"scope:<milestone>"`, "the finding contract names the scope location")
}

// TestEpicReview_AScopeUnitBringsTheLiveScope: a role asked scope:<m> reads
// the live scope as context, each epic once in its brief form; a role not
// asked it does not.
func TestEpicReview_AScopeUnitBringsTheLiveScope(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, scopeAndPlainRoles)

	out := runEpicReview(t, dir, "--milestone", "A")
	require.Equal(t, []string{"plain", "scope-only"}, promptRoles(&out))
	assert.NotContains(t, out.Prompts[0].Prompt, "Scope of gate A")
	scope := out.Prompts[1].Prompt
	assert.Contains(t, scope, "Scope of gate A")
	assert.Contains(t, scope, "\n- a [member] A\n  acceptance: a ships\n  depends_on: b: a needs b\n")
	assert.Contains(t, scope, "critical_path (by count): d -> c -> b -> a\n")
}

// TestEpicReview_ARoleWithNoUnitOfItsKindsIsSkipped: an epic slice holds no
// scope unit, so the scope-only role gets no prompt and is named in
// skipped_roles with no-units; the slice still converges.
func TestEpicReview_ARoleWithNoUnitOfItsKindsIsSkipped(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, scopeAndPlainRoles)

	out := runEpicReview(t, dir, "--epic", "b")
	assert.Equal(t, []string{"plain"}, promptRoles(&out))
	assert.Equal(t, map[string]string{"scope-only": "no-units"}, epicSkipped(t, dir, "--epic", "b"))

	empty := filepath.Join(t.TempDir(), "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	for range 2 {
		runEpicReview(t, dir, "--epic", "b")
		_, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--record", empty)
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}
	stdout, stderr, code := runTP(t, dir, "epic", "review", "--epic", "b", "--status", "--check")
	require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)
}

// TestEpicReview_AKindNoRoleReadsHasNoUnit: a corpus whose every role takes
// only epics produces no gate and no scope unit, in emission, status and
// export alike.
func TestEpicReview_AKindNoRoleReadsHasNoUnit(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, map[string]string{"placer": purposeRoles["placer"]})
	out := runEpicReview(t, dir, "--milestone", "A")
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d"}, out.Units)
	status, _, _ := runTP(t, dir, "epic", "review", "--milestone", "A", "--status")
	assert.NotContains(t, status, "gate:A")
	export, _, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code)
	assert.NotContains(t, export, `"gate:A"`)
	assert.NotContains(t, export, `"scope:A"`)
}

// TestEpicReview_AnUnreadableCorpusFailsTheStatus: a role file that does not
// parse is an exit-3 error in --status, not a corpus read as empty.
func TestEpicReview_AnUnreadableCorpusFailsTheStatus(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, map[string]string{"broken": `{"id": "broken", "title": "B", "instructions": "I", "units": []}`})
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--status")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "epic-reviewers")
	_, _, code = runTP(t, dir, "epic", "review", "--all", "--status")
	assert.Equal(t, 3, code)
}

// TestEpicReview_AScopeFindingRecords: a row located at scope:<m> is a unit of
// the slice, so --record takes it.
func TestEpicReview_AScopeFindingRecords(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChainThreshold)
	writeEpicRoles(t, dir, scopeAndPlainRoles)
	runEpicReview(t, dir, "--milestone", "A")
	rows := filepath.Join(t.TempDir(), "rows.ndjson")
	require.NoError(t, os.WriteFile(rows, []byte(`{"severity":"medium","finding":"the threshold needs a load test epic e9","location":"scope:A","evidence":"threshold: ten users","role":"scope-only"}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}
