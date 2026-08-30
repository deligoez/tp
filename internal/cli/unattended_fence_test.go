package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unattendedOn is the variable tp run exports for every child (§3.1.1), which
// §5.1 turns into a fence around the decisions CLAUDE.md reserves for the user.
const unattendedOn = "TP_UNATTENDED=1"

// fenceProject seeds a project whose quality gate fails, plus one open task —
// the situation in which an agent reaches for --skip-gate.
func fenceProject(t *testing.T) string {
	t.Helper()
	dir := setupProjectWithGate(t, "exit 7")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)
	return dir
}

// §5.1 row 1: tp done --skip-gate is refused under the variable and is the
// ordinary user decision without it.
func TestUnattendedFence_DoneSkipGateRefused(t *testing.T) {
	dir := fenceProject(t)

	stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn},
		"done", "t1", "task complete and verified fully", "--skip-gate", "the gate is broken")
	require.Equal(t, 2, code, "--skip-gate under TP_UNATTENDED must exit 2: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "user-approved decision")
	assert.Contains(t, stdout+stderr, "tp escalate", "the hint names the supported route")

	task := showTask(t, dir, "t1")
	assert.Equal(t, "open", task["status"], "a refused skip closes nothing")

	_, stderr, code = runTPEnv(t, dir, nil,
		"done", "t1", "task complete and verified fully", "--skip-gate", "the gate is broken")
	require.Equal(t, 0, code, "without the variable the same command is the user's own decision: %s", stderr)
}

// The refusal is at the sink, not at one flag: every supported route to a
// skipped gate is fenced, since a unit that can reach it through --batch or
// tp close has not been fenced at all.
func TestUnattendedFence_SkipGateRefusedThroughEveryRoute(t *testing.T) {
	dir := fenceProject(t)

	ndjson := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(ndjson,
		[]byte(`{"id":"t1","reason":"task complete and verified fully","skip_gate":"the gate is broken"}`+"\n"), 0o600))
	stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn}, "done", "--batch", ndjson)
	require.Equal(t, 2, code, "a batch entry's skip_gate is the same decision: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "user-approved decision")

	_, stderr, code = runTPEnv(t, dir, nil, "claim", "t1")
	require.Equal(t, 0, code, "claim: %s", stderr)
	stdout, stderr, code = runTPEnv(t, dir, []string{unattendedOn},
		"close", "t1", "task complete and verified fully", "--skip-gate", "the gate is broken")
	require.Equal(t, 2, code, "tp close --skip-gate is the same decision: %s%s", stdout, stderr)

	task := showTask(t, dir, "t1")
	assert.Equal(t, "wip", task["status"], "neither route closed the task")
}

// §5.1 row 3: tp import --force.
func TestUnattendedFence_ImportForceRefused(t *testing.T) {
	dir := fenceProject(t)

	importPath := filepath.Join(dir, "import.json")
	doc := `{"version":1,"spec":"spec.md","tasks":[` +
		`{"id":"n1","title":"New","estimate_minutes":5,"acceptance":"New task complete.","source_sections":["# Test Spec"],"depends_on":[]}]}`
	require.NoError(t, os.WriteFile(importPath, []byte(doc), 0o600))

	stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn}, "import", importPath, "--force")
	require.Equal(t, 2, code, "--force under TP_UNATTENDED must exit 2: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "user-approved decision")
	assert.Contains(t, stdout+stderr, "import-force", "the hint names the escalation decision")

	_, stderr, code = runTPEnv(t, dir, nil, "import", importPath, "--force")
	require.Equal(t, 0, code, "without the variable the import is the user's own decision: %s", stderr)
}

// §5.1 row 2: the two round budgets. The comparison is against the resolved
// value, an equal or lower non-zero value is accepted, and 0 means *disabled*
// rather than *lowest*, so it is a raise wherever a non-zero value resolves.
func TestUnattendedFence_RoundBudgetRaiseRefused(t *testing.T) {
	dir := setupProject(t)

	_, stderr, code := runTPEnv(t, dir, nil, "set", "--workflow", "review_max_rounds=5", "audit_max_rounds=5")
	require.Equal(t, 0, code, "seeding the resolved caps: %s", stderr)

	stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "review_max_rounds=6")
	require.Equal(t, 2, code, "a raise must exit 2: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "user-approved decision")
	assert.Contains(t, stdout+stderr, "raise-review-cap", "the hint names the escalation decision")

	stdout, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "audit_max_rounds=50")
	require.Equal(t, 2, code, "the audit cap is fenced on the same terms: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "raise-audit-cap")

	_, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "review_max_rounds=5")
	require.Equal(t, 0, code, "an equal value is accepted: %s", stderr)
	_, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "review_max_rounds=4")
	require.Equal(t, 0, code, "a lower value is accepted: %s", stderr)

	_, _, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "review_max_rounds=0")
	require.Equal(t, 2, code, "0 removes the cap, so it is a raise")

	_, stderr, code = runTPEnv(t, dir, nil, "set", "--workflow", "review_max_rounds=9")
	require.Equal(t, 0, code, "without the variable the raise is the user's own decision: %s", stderr)
}

// §5.1 row 4: the driver's own caps, fenced on the same terms — a unit that
// could raise run_max_units or run_max_wall_clock_seconds could run itself
// indefinitely.
func TestUnattendedFence_RunCapRaiseRefused(t *testing.T) {
	dir := setupProject(t)

	stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_units=200")
	require.Equal(t, 2, code, "200 over the resolved default of 100 is a raise: %s%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "user-approved decision")

	_, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_units=50")
	require.Equal(t, 0, code, "lowering a driver cap is accepted: %s", stderr)
	_, _, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_units=51")
	require.Equal(t, 2, code, "the comparison follows the newly resolved 50")

	_, _, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_wall_clock_seconds=604800")
	require.Equal(t, 2, code, "the wall clock cap is fenced")
	_, _, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_unit_retries=5")
	require.Equal(t, 2, code, "more retries than resolved is a raise")
	_, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_unit_retries=0")
	require.Equal(t, 0, code, "0 retries is the strictest value, not disabled: %s", stderr)

	// A budget's 0 is "disabled", so a real number under it is a lowering and
	// a return to 0 is a raise.
	_, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_budget_usd=5")
	require.Equal(t, 0, code, "a real budget under an unbounded one is a lowering: %s", stderr)
	_, _, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_budget_usd=10")
	require.Equal(t, 2, code, "a bigger budget is a raise")
	_, _, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_budget_usd=0")
	require.Equal(t, 2, code, "0 disables the budget, so it is a raise")
	_, stderr, code = runTPEnv(t, dir, []string{unattendedOn}, "set", "--workflow", "run_max_budget_usd=2.5")
	require.Equal(t, 0, code, "a smaller budget is accepted: %s", stderr)

	// Without the variable every run field is an ordinary settable field.
	_, stderr, code = runTPEnv(t, dir, nil, "set", "--workflow",
		"run_max_units=500", "run_max_wall_clock_seconds=100000",
		"run_max_budget_usd=42.5", "run_max_unit_budget_usd=3", "run_max_unit_retries=4")
	require.Equal(t, 0, code, "without the variable the raises are the user's own decision: %s", stderr)

	stdout, stderr, code = runTPEnv(t, dir, nil, "config", "--resolved")
	require.Equal(t, 0, code, "config --resolved: %s", stderr)
	var resolved map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &resolved))
	workflow := resolved["workflow"].(map[string]any)
	assert.InDelta(t, 500.0, workflow["run_max_units"].(map[string]any)["value"], 1e-9,
		"the accepted write actually landed")
	assert.InDelta(t, 42.5, workflow["run_max_budget_usd"].(map[string]any)["value"], 1e-9)
}

// Section 5.1: the fence compares against the value section 7's precedence
// actually resolves. TP_REVIEW_MAX_ROUNDS is set here as a decoy, not as a
// layer: tp reads no TP_<FIELD> variable for any workflow field, so its mere
// presence proves nothing — this assertion would pass with the fence removed if
// the decoy were all it rested on.
//
// What discriminates is the pair of numbers. The resolved cap is 5; 40 is a
// raise against it and must be refused. Were the fence reading the environment,
// 40 would be a lowering against the decoy's 50 and would be accepted, so the
// refusal is evidence about which number the fence compared.
func TestUnattendedFence_EnvLayerIgnored(t *testing.T) {
	dir := setupProject(t)

	_, stderr, code := runTPEnv(t, dir, nil, "set", "--workflow", "review_max_rounds=5")
	require.Equal(t, 0, code, "seeding the resolved cap: %s", stderr)

	_, _, code = runTPEnv(t, dir, []string{unattendedOn, "TP_REVIEW_MAX_ROUNDS=50"},
		"set", "--workflow", "review_max_rounds=40")
	require.Equal(t, 2, code, "40 is a raise against the resolved 5, whatever the environment says")

	_, _, code = runTPEnv(t, dir, []string{unattendedOn, "TP_RUN_MAX_UNITS=10000"},
		"set", "--workflow", "run_max_units=9000")
	require.Equal(t, 2, code, "9000 is a raise against the resolved 100, whatever the environment says")
}

// §5.1: runner and notify_cmd name commands the driver executes, so under the
// variable a unit cannot set them at all, at any layer.
func TestUnattendedFence_RunnerAndNotifyCmdRefusedAtEveryLayer(t *testing.T) {
	dir := setupProject(t)

	for _, args := range [][]string{
		{"set", "--workflow", "runner=claude"},
		{"set", "--workflow", "--project", "runner=claude"},
		{"set", "--workflow", "notify_cmd=say done"},
		{"set", "--workflow", "--project", "notify_cmd=say done"},
		{"set", "--local", "notify_cmd=say done"},
	} {
		stdout, stderr, code := runTPEnv(t, dir, []string{unattendedOn}, args...)
		require.Equal(t, 2, code, "%v must be refused: %s%s", args, stdout, stderr)
		assert.Contains(t, stdout+stderr, "TP_UNATTENDED", "the refusal names the fence: %v", args)
	}
}

// §5.1 / test 64, end to end: the mode activates on any present, non-empty
// value other than 0.
func TestUnattendedFence_ActivationVocabulary(t *testing.T) {
	for _, value := range []string{"1", "true", "yes"} {
		dir := fenceProject(t)
		_, _, code := runTPEnv(t, dir, []string{"TP_UNATTENDED=" + value},
			"done", "t1", "task complete and verified fully", "--skip-gate", "the gate is broken")
		assert.Equal(t, 2, code, "TP_UNATTENDED=%q activates the fence", value)
	}
	for _, value := range []string{"", "0"} {
		dir := fenceProject(t)
		_, stderr, code := runTPEnv(t, dir, []string{"TP_UNATTENDED=" + value},
			"done", "t1", "task complete and verified fully", "--skip-gate", "the gate is broken")
		assert.Equal(t, 0, code, "TP_UNATTENDED=%q leaves the decision available: %s", value, stderr)
	}
}
