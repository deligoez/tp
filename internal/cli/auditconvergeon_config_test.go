package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// v0.37.0 §7 row 4, end to end. The two halves are asserted separately because
// they fail separately: the named mutant (drop the field from hoistedFields)
// leaves tp validate --project untouched, and the validate-side mutant leaves
// tp config --extract untouched. v0.31.0 shipped this field's twin with the
// resolution half and neither config surface, and needed v0.31.1 to finish it;
// that is the whole reason the row names two commands.

// extractProject seeds a repository holding two task files that both carry the
// same audit_converge_on, which is the population --extract hoists from.
func extractProject(t *testing.T, value string, bases ...string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	for _, base := range bases {
		require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"),
			[]byte("# S\n\n## 1. Setup\n\nDo the thing.\n"), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(dir, base+".tasks.json"),
			[]byte(`{"version":1,"spec":"`+base+`.md","tasks":[],"workflow":`+
				`{"audit_converge_on":"`+value+`","review_max_rounds":7}}`), 0o600))
	}
	return dir
}

// The extract half of row 4. The environment is named rather than inherited
// (§7's closing note): the fence is a separate decision and this test is about
// the hoist, so it runs with TP_UNATTENDED unset.
func TestConfigExtract_HoistsAuditConvergeOn(t *testing.T) {
	t.Parallel()
	dir := extractProject(t, "blocking", "a", "b")

	out, stderr, code := runTPFence(t, dir, false, "config", "--extract")
	require.Equal(t, 0, code, "config --extract: %s", stderr)

	var res struct {
		Hoisted []string `json:"hoisted"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Contains(t, res.Hoisted, "audit_converge_on",
		"the command emits the field it hoisted")

	// The value reaches the project layer...
	cfg, err := os.ReadFile(filepath.Join(dir, ".tp", "config.json"))
	require.NoError(t, err)
	assert.Contains(t, string(cfg), `"audit_converge_on": "blocking"`,
		"mergeCommon writes the hoisted value into .tp/config.json")

	// ...and leaves the task layer, which is what makes the hoist a move
	// rather than a copy. Asserted beside the line above rather than instead
	// of it: a hoist that strips the task files without writing the project
	// config destroys the value, and either assertion alone passes that.
	tasks, err := os.ReadFile(filepath.Join(dir, "a.tasks.json"))
	require.NoError(t, err)
	assert.NotContains(t, string(tasks), "audit_converge_on",
		"the hoisted field is stripped from the task file")
}

// What the hoist must preserve, read through the surface an operator reads it
// through: the value is unchanged and only its source moves down a layer. One
// task file, because tp config resolves against the discovered one and the
// unanimity the test above needs is not what this one is measuring.
func TestConfigExtract_AuditConvergeOnStillResolvesAfterTheHoist(t *testing.T) {
	t.Parallel()
	dir := extractProject(t, "blocking", "a")

	before := resolvedAuditConvergeOn(t, dir)
	require.Equal(t, "blocking", before["value"])
	require.Equal(t, "override", before["source"], "the task file holds it to begin with")

	_, stderr, code := runTPFence(t, dir, false, "config", "--extract")
	require.Equal(t, 0, code, "config --extract: %s", stderr)

	after := resolvedAuditConvergeOn(t, dir)
	assert.Equal(t, "blocking", after["value"], "the hoist leaves the resolved value alone")
	assert.Equal(t, "project", after["source"], "and moves the layer it wins on")
}

// The validate half of row 4, through the command rather than through
// workflowDeviations. "Accepts a legal value" is exit 0 with nothing reported;
// "rejects an illegal one" is the deviation report plus --strict's exit 1 —
// which is where tp validate --project refuses, since §2 places the literal's
// refusal at the write sinks (exit 2) and the consuming audit sinks (exit 1).
func TestValidateProject_AuditConvergeOn_AcceptsLegalRejectsIllegal(t *testing.T) {
	t.Parallel()
	deviationFields := func(t *testing.T, stdout string) []string {
		t.Helper()
		var payload struct {
			Deviations []map[string]any `json:"deviations"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
		fields := make([]string, 0, len(payload.Deviations))
		for _, d := range payload.Deviations {
			fields = append(fields, d["field"].(string))
		}
		return fields
	}

	t.Run("a legal value the project also holds is accepted", func(t *testing.T) {
		dir := writeProjectConfigDir(t, `{"audit_converge_on":"all"}`)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
			[]byte(`{"spec":"s.md","tasks":[],"workflow":{"audit_converge_on":"all"}}`), 0o600))

		stdout, stderr, code := runTP(t, dir, "validate", "--project", "--strict")
		require.Equal(t, 0, code, "validate --project --strict: %s", stderr)
		assert.NotContains(t, deviationFields(t, stdout), "audit_converge_on",
			"a task file agreeing with the project policy is not a deviation")
	})

	t.Run("an illegal value against a legal project policy is rejected", func(t *testing.T) {
		dir := writeProjectConfigDir(t, `{"audit_converge_on":"all"}`)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
			[]byte(`{"spec":"s.md","tasks":[],"workflow":{"audit_converge_on":"nope"}}`), 0o600))

		stdout, stderr, code := runTP(t, dir, "validate", "--project")
		require.Equal(t, 0, code, "validate --project: %s", stderr)
		fields := deviationFields(t, stdout)
		require.Contains(t, fields, "audit_converge_on",
			"a task file contradicting the project policy is reported")

		// --strict is what turns the report into a refusal, and it is asserted
		// here rather than assumed from the generic promotion rule: a field the
		// report never names cannot arm it.
		_, stderr, code = runTP(t, dir, "validate", "--project", "--strict")
		assert.Equal(t, 1, code, "--strict refuses the reported deviation: %s", stderr)
	})

	// A legal value that differs from the project policy is reported on the
	// same terms — the surface compares, it does not grade legality — so the
	// test above is not read as "only illegal values deviate".
	t.Run("a legal value that contradicts the project policy is reported too", func(t *testing.T) {
		dir := writeProjectConfigDir(t, `{"audit_converge_on":"all"}`)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"),
			[]byte(`{"spec":"s.md","tasks":[],"workflow":{"audit_converge_on":"blocking"}}`), 0o600))

		stdout, stderr, code := runTP(t, dir, "validate", "--project")
		require.Equal(t, 0, code, "validate --project: %s", stderr)
		assert.Contains(t, deviationFields(t, stdout), "audit_converge_on")
	})
}
