package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicChain is §9's chain a → b → c → d, all in milestone A of tracks
// [[A, B], [L1]], every why distinct.
const epicChain = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "a needs b"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "c needs d"}]},
    {"id": "d", "title": "D", "milestone": "A"}
  ]
}
`

// epicReviewPayload is the part of tp epic review's emission these tests read.
type epicReviewPayload struct {
	StateDir string   `json:"state_dir"`
	Round    int      `json:"round"`
	Units    []string `json:"units"`
	Context  []string `json:"context"`
	Carried  []string `json:"carried"`
	Prompts  []struct {
		Role       string `json:"role"`
		Prompt     string `json:"prompt"`
		OutputPath string `json:"output_path"`
	} `json:"prompts"`
	MechanicalChecks []struct {
		Class    string `json:"class"`
		Passed   bool   `json:"passed"`
		Findings int    `json:"findings"`
	} `json:"mechanical_checks"`
	MechanizedClasses []string `json:"mechanized_classes"`
	NextAction        string   `json:"next_action"`
}

// runEpicReview runs tp epic review in dir and decodes its payload on exit 0.
func runEpicReview(t *testing.T, dir string, args ...string) epicReviewPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicReviewPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

func promptRoles(p *epicReviewPayload) []string {
	roles := []string{}
	for _, pr := range p.Prompts {
		roles = append(roles, pr.Role)
	}
	return roles
}

// TestEpicReview_BothSliceFlagsOrNeitherIsAUsageError is §9 row 15: a command
// that picked the first flag and emitted would exit 0 and open a state
// directory.
func TestEpicReview_BothSliceFlagsOrNeitherIsAUsageError(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	for _, args := range [][]string{
		{"epic", "review"},
		{"epic", "review", "--milestone", "A", "--epic", "b"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 2, code, "%v: stderr: %s", args, stderr)
	}
	_, err := os.Stat(filepath.Join(dir, ".tp", "epic-review"))
	assert.True(t, os.IsNotExist(err), "a usage error creates no state directory")
}

// TestEpicReview_BuiltInRolesRunUntilTheDirectoryReplacesThem is §9 row 22:
// three prompts named by the built-in roles with no .tp/epic-reviewers/, and
// one prompt by the one role file once the directory holds it — never merged.
func TestEpicReview_BuiltInRolesRunUntilTheDirectoryReplacesThem(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	out := runEpicReview(t, dir, "--epic", "b")
	assert.Equal(t, []string{"dod-satisfiability", "why-auditor", "phase-economist"}, promptRoles(&out))
	assert.Equal(t, []string{"epic:b"}, out.Units)
	assert.Equal(t, []string{"a", "c"}, out.Context)
	for _, p := range out.Prompts {
		assert.Contains(t, p.Prompt, "epic: b\n", "%s asks b's block", p.Role)
		assert.Contains(t, p.Prompt, "epic: c\n", "%s carries b's dependency c as context", p.Role)
		assert.NotContains(t, p.Prompt, "epic: d\n", "%s carries no block for d", p.Role)
		assert.NotEmpty(t, p.OutputPath)
	}

	rolesDir := filepath.Join(dir, ".tp", "epic-reviewers")
	require.NoError(t, os.Mkdir(rolesDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rolesDir, "edge-reader.json"),
		[]byte(`{"id": "edge-reader", "title": "Edge reader", "instructions": "Read every edge."}`), 0o600))

	out = runEpicReview(t, dir, "--epic", "b")
	assert.Equal(t, []string{"edge-reader"}, promptRoles(&out))
}

// TestEpicReview_LintErrorsAreMechanicalChecksWithheldFromThePrompts is §5's
// mechanized classes: a cycle outside the slice does not refuse the emission,
// mechanical_checks reports cycle failing, and every error rule's slug is in
// mechanized_classes and in each prompt's do-not-report sentence.
func TestEpicReview_LintErrorsAreMechanicalChecksWithheldFromThePrompts(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "depends_on": [{"id": "b", "why": "a needs b"}]},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "c", "why": "b needs c"}]},
    {"id": "c", "title": "C", "milestone": "A", "depends_on": [{"id": "d", "why": "c needs d"}]},
    {"id": "d", "title": "D", "milestone": "A"},
    {"id": "x", "title": "X", "milestone": "A", "depends_on": [{"id": "y", "why": "x needs y"}]},
    {"id": "y", "title": "Y", "milestone": "A", "depends_on": [{"id": "x", "why": "y needs x"}]}
  ]
}
`)
	out := runEpicReview(t, dir, "--epic", "b")

	passed := map[string]bool{}
	for _, c := range out.MechanicalChecks {
		passed[c.Class] = c.Passed
	}
	require.Contains(t, passed, "cycle")
	assert.False(t, passed["cycle"], "the cycle x <-> y fails its check")
	assert.True(t, passed["dangling-reference"], "a rule with no finding passes")

	errorRules := []string{
		"cycle", "dangling-reference", "duplicate-name", "gate-member-outside-milestone",
		"later-milestone-dependency", "missing-why", "opens-after-cycle", "unknown-key",
		"unknown-milestone", "unprovided-tag-check",
	}
	assert.Equal(t, errorRules, out.MechanizedClasses)
	for _, p := range out.Prompts {
		for _, rule := range errorRules {
			assert.Contains(t, p.Prompt, rule, "%s withholds %s", p.Role, rule)
		}
	}
}

// TestEpicReview_RoleNarrowsTheEmissionToOnePrompt is §5's --role: the name of
// an emitted role keeps its prompt alone, and a name tp knows nowhere exits 2.
func TestEpicReview_RoleNarrowsTheEmissionToOnePrompt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	out := runEpicReview(t, dir, "--milestone", "A", "--role", "why-auditor")
	assert.Equal(t, []string{"why-auditor"}, promptRoles(&out))
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A"}, out.Units)

	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--role", "no-such-role")
	assert.Equal(t, 2, code, "stderr: %s", stderr)
}

// TestEpicReview_AnEmissionRecordsTheSliceKey is spec/1.5.0.md §7: every
// emission writes its slice's key into the slice's state directory, so a
// reader can list the slices without inverting the directory's hash.
func TestEpicReview_AnEmissionRecordsTheSliceKey(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	out := runEpicReview(t, dir, "--epics", "c,a", "--name", "pair")
	data, err := os.ReadFile(filepath.Join(dir, out.StateDir, "key.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind": "epics", "ids": ["a", "c"], "name": "pair"}`, string(data))

	out = runEpicReview(t, dir, "--milestone", "A")
	data, err = os.ReadFile(filepath.Join(dir, out.StateDir, "key.json"))
	require.NoError(t, err)
	assert.JSONEq(t, `{"kind": "milestone", "value": "A"}`, string(data))
}
