package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fptr(v float64) *float64 { return &v }

// tp config --resolved reports every §7 field with its value and the layer it
// came from — including notify_cmd, which can only ever report local or
// default because .tp/local.json is its only layer.
func TestResolvedConfig_CoversRunFields(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"run_max_wall_clock_seconds":600,"run_max_units":50}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"),
		[]byte(`{"notify_cmd":"say done"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{"run_max_units":7,"runner":{"cmd":"my-agent"}}}`), 0o600))

	t.Chdir(root)
	wf, override := resolveConfigWorkflow()
	workflow := resolvedConfig(&wf, &override)["workflow"].(map[string]any)

	field := func(name string) map[string]any {
		entry, ok := workflow[name].(map[string]any)
		require.True(t, ok, "tp config --resolved reports %s", name)
		return entry
	}

	assert.Equal(t, "override", field("run_max_units")["source"])
	assert.Equal(t, 7, field("run_max_units")["value"])
	assert.Equal(t, "project", field("run_max_wall_clock_seconds")["source"])
	assert.Equal(t, 600, field("run_max_wall_clock_seconds")["value"])
	assert.Equal(t, "default", field("run_max_budget_usd")["source"])
	assert.Equal(t, 0.0, field("run_max_budget_usd")["value"])
	assert.Equal(t, "default", field("run_max_unit_budget_usd")["source"])
	assert.Equal(t, "default", field("run_max_unit_retries")["source"])
	assert.Equal(t, 1, field("run_max_unit_retries")["value"])
	assert.Equal(t, "override", field("runner")["source"])
	assert.JSONEq(t, `{"cmd":"my-agent"}`, string(field("runner")["value"].(json.RawMessage)))
	assert.Equal(t, "local", field("notify_cmd")["source"], "notify_cmd resolves from .tp/local.json only")
	assert.Equal(t, "say done", field("notify_cmd")["value"])
}

func TestResolvedConfig_RunnerAndNotifyCmdDefaults(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"), []byte(`{}`), 0o600))

	t.Chdir(root)
	wf, override := resolveConfigWorkflow()
	workflow := resolvedConfig(&wf, &override)["workflow"].(map[string]any)

	runner := workflow["runner"].(map[string]any)
	assert.Equal(t, "default", runner["source"])
	assert.JSONEq(t, `"claude"`, string(runner["value"].(json.RawMessage)))

	notify := workflow["notify_cmd"].(map[string]any)
	assert.Equal(t, "default", notify["source"], "an unset notify_cmd reports default")
	assert.Equal(t, "", notify["value"])
}

// tp config --extract hoists the six project-layer run fields on the same terms
// as every existing workflow field, and never notify_cmd — which is not a
// task-file override at all, so there is nothing to hoist.
func TestComputeCommonPolicy_HoistsRunFields(t *testing.T) {
	runner := json.RawMessage(`{"cmd":"my-agent"}`)
	overrides := []model.WorkflowOverride{
		{
			RunMaxUnits: iptr(50), RunMaxWallClockSeconds: iptr(600),
			RunMaxBudgetUSD: fptr(12.5), RunMaxUnitBudgetUSD: fptr(0.5),
			RunMaxUnitRetries: iptr(2), Runner: runner,
		},
		{
			RunMaxUnits: iptr(50), RunMaxWallClockSeconds: iptr(900),
			RunMaxBudgetUSD: fptr(12.5), RunMaxUnitBudgetUSD: fptr(0.5),
			RunMaxUnitRetries: iptr(2), Runner: json.RawMessage(`{"cmd":"my-agent"}`),
		},
	}

	common := computeCommonPolicy(overrides)
	require.NotNil(t, common.RunMaxUnits)
	assert.Equal(t, 50, *common.RunMaxUnits)
	assert.Nil(t, common.RunMaxWallClockSeconds, "a divergent run field is not hoisted")
	require.NotNil(t, common.RunMaxBudgetUSD)
	assert.Equal(t, 12.5, *common.RunMaxBudgetUSD)
	require.NotNil(t, common.RunMaxUnitRetries)
	assert.Equal(t, 2, *common.RunMaxUnitRetries)
	assert.JSONEq(t, `{"cmd":"my-agent"}`, string(common.Runner), "an identical runner is hoisted")

	assert.ElementsMatch(t,
		[]string{"run_max_units", "run_max_budget_usd", "run_max_unit_budget_usd", "run_max_unit_retries", "runner"},
		hoistedFields(&common))
	assert.NotContains(t, hoistedFields(&common), "notify_cmd",
		"notify_cmd is per-operator and is never hoisted into the project config")

	// A divergent runner is not hoisted either.
	divergent := computeCommonPolicy([]model.WorkflowOverride{
		{Runner: json.RawMessage(`"claude"`)},
		{Runner: json.RawMessage(`"opencode"`)},
	})
	assert.Nil(t, divergent.Runner)
}

func TestMergeCommon_WritesRunFields(t *testing.T) {
	dst := model.WorkflowOverride{AuditMaxRounds: iptr(3)} // hand-set project field
	mergeCommon(&dst, &model.WorkflowOverride{
		RunMaxUnits: iptr(50), RunMaxWallClockSeconds: iptr(600),
		RunMaxBudgetUSD: fptr(12.5), RunMaxUnitBudgetUSD: fptr(0.5),
		RunMaxUnitRetries: iptr(0), Runner: json.RawMessage(`"opencode"`),
	})

	require.NotNil(t, dst.RunMaxUnits)
	assert.Equal(t, 50, *dst.RunMaxUnits)
	require.NotNil(t, dst.RunMaxWallClockSeconds)
	assert.Equal(t, 600, *dst.RunMaxWallClockSeconds)
	require.NotNil(t, dst.RunMaxBudgetUSD)
	assert.Equal(t, 12.5, *dst.RunMaxBudgetUSD)
	require.NotNil(t, dst.RunMaxUnitBudgetUSD)
	assert.Equal(t, 0.5, *dst.RunMaxUnitBudgetUSD)
	require.NotNil(t, dst.RunMaxUnitRetries)
	assert.Equal(t, 0, *dst.RunMaxUnitRetries)
	assert.JSONEq(t, `"opencode"`, string(dst.Runner))
	require.NotNil(t, dst.AuditMaxRounds)
	assert.Equal(t, 3, *dst.AuditMaxRounds, "other hand-set fields are preserved")
}

// tp validate --project reports a run-field deviation on the same terms as
// every other workflow field.
func TestWorkflowDeviations_RunFields(t *testing.T) {
	project := model.WorkflowOverride{
		RunMaxUnits: iptr(100), RunMaxWallClockSeconds: iptr(600),
		RunMaxBudgetUSD: fptr(10), RunMaxUnitBudgetUSD: fptr(0.5),
		RunMaxUnitRetries: iptr(1), Runner: json.RawMessage(`"claude"`),
	}
	override := model.WorkflowOverride{
		RunMaxUnits: iptr(7), RunMaxWallClockSeconds: iptr(600),
		RunMaxBudgetUSD: fptr(2.5), RunMaxUnitBudgetUSD: fptr(0.5),
		RunMaxUnitRetries: iptr(0), Runner: json.RawMessage(`"opencode"`),
	}

	devs := workflowDeviations("s.tasks.json", &override, &project)
	byField := make(map[string]map[string]any, len(devs))
	for _, d := range devs {
		byField[d["field"].(string)] = d
	}

	require.Contains(t, byField, "run_max_units")
	assert.Equal(t, "7", byField["run_max_units"]["override"])
	assert.Equal(t, "100", byField["run_max_units"]["project"])
	require.Contains(t, byField, "run_max_budget_usd")
	assert.Equal(t, "2.5", byField["run_max_budget_usd"]["override"], "a budget prints as decimal dollars")
	assert.Equal(t, "10", byField["run_max_budget_usd"]["project"])
	require.Contains(t, byField, "run_max_unit_retries")
	assert.Equal(t, "0", byField["run_max_unit_retries"]["override"])
	require.Contains(t, byField, "runner")
	assert.Equal(t, `"opencode"`, byField["runner"]["override"])
	assert.NotContains(t, byField, "run_max_wall_clock_seconds", "an equal field is not a deviation")
	assert.NotContains(t, byField, "run_max_unit_budget_usd", "an equal budget is not a deviation")
	assert.NotContains(t, byField, "notify_cmd", "notify_cmd is not a project-config field")

	// A field only one side sets carries no policy and is not a deviation.
	assert.Empty(t, workflowDeviations("s.tasks.json",
		&model.WorkflowOverride{RunMaxUnits: iptr(7)},
		&model.WorkflowOverride{}))
}
