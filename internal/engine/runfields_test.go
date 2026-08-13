package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectDir writes a .tp/config.json holding the given workflow block and
// returns the project root, so a field can be exercised through the real
// parse → clamp → resolve path rather than a hand-built override.
func projectDir(t *testing.T, workflowJSON string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
		[]byte(`{"workflow":`+workflowJSON+`}`), 0o600))
	return dir
}

// resolveProjectWorkflow resolves the workflow for a project whose config holds
// only the given workflow block, returning the effective values and the config
// validation warnings.
func resolveProjectWorkflow(t *testing.T, workflowJSON string) (wf model.Workflow, warnings []string) {
	t.Helper()
	dir := projectDir(t, workflowJSON)
	wf, warnings, err := ResolveEffectiveWorkflow(dir, &model.WorkflowOverride{})
	require.NoError(t, err)
	return wf, warnings
}

func TestRunFields_BuiltInDefaults(t *testing.T) {
	wf := ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})

	assert.Equal(t, 100, wf.RunMaxUnits, "run_max_units defaults to 100")
	assert.Equal(t, 28800, wf.RunMaxWallClockSeconds, "run_max_wall_clock_seconds defaults to 28800")
	assert.Equal(t, 0.0, wf.RunMaxBudgetUSD, "run_max_budget_usd defaults to 0 (disabled)")
	assert.Equal(t, 0.0, wf.RunMaxUnitBudgetUSD, "run_max_unit_budget_usd defaults to 0 (flag omitted)")
	assert.Equal(t, 1, wf.RunMaxUnitRetries, "run_max_unit_retries defaults to 1 (two attempts)")
	assert.JSONEq(t, `"claude"`, string(wf.Runner), "runner defaults to the claude template")
	assert.Empty(t, wf.NotifyCmd, "notify_cmd is unset by default")
}

// Each range is documented as inclusive, so both endpoints must be accepted and
// one step outside each must be rejected: a mid-range value would pass whether
// the bound were inclusive or exclusive and would prove nothing.
func TestRunFields_RangeEndpointsAndOneStepOutside(t *testing.T) {
	cases := []struct {
		field      string
		lo, hi     string // the endpoints, as JSON literals
		below      string // one step below the low bound
		above      string // one step above the high bound
		loWant     float64
		hiWant     float64
		defaultVal float64
		get        func(model.Workflow) float64
	}{
		{
			field: "run_max_units", lo: "1", hi: "10000", below: "0", above: "10001",
			loWant: 1, hiWant: 10000, defaultVal: 100,
			get: func(w model.Workflow) float64 { return float64(w.RunMaxUnits) },
		},
		{
			field: "run_max_wall_clock_seconds", lo: "60", hi: "604800", below: "59", above: "604801",
			loWant: 60, hiWant: 604800, defaultVal: 28800,
			get: func(w model.Workflow) float64 { return float64(w.RunMaxWallClockSeconds) },
		},
		{
			field: "run_max_unit_retries", lo: "0", hi: "5", below: "-1", above: "6",
			loWant: 0, hiWant: 5, defaultVal: 1,
			get: func(w model.Workflow) float64 { return float64(w.RunMaxUnitRetries) },
		},
		{
			field: "run_max_budget_usd", lo: "0", hi: "10000", below: "-0.01", above: "10000.01",
			loWant: 0, hiWant: 10000, defaultVal: 0,
			get: func(w model.Workflow) float64 { return w.RunMaxBudgetUSD },
		},
		{
			field: "run_max_unit_budget_usd", lo: "0", hi: "1000", below: "-0.01", above: "1000.01",
			loWant: 0, hiWant: 1000, defaultVal: 0,
			get: func(w model.Workflow) float64 { return w.RunMaxUnitBudgetUSD },
		},
	}

	for _, tc := range cases {
		t.Run(tc.field, func(t *testing.T) {
			wf, warnings := resolveProjectWorkflow(t, `{"`+tc.field+`":`+tc.lo+`}`)
			assert.Equal(t, tc.loWant, tc.get(wf), "the low endpoint is inside the range")
			assert.Empty(t, warnings, "an endpoint value warns about nothing")

			wf, warnings = resolveProjectWorkflow(t, `{"`+tc.field+`":`+tc.hi+`}`)
			assert.Equal(t, tc.hiWant, tc.get(wf), "the high endpoint is inside the range")
			assert.Empty(t, warnings, "an endpoint value warns about nothing")

			wf, warnings = resolveProjectWorkflow(t, `{"`+tc.field+`":`+tc.below+`}`)
			assert.Equal(t, tc.defaultVal, tc.get(wf), "one step below the low bound falls back to the default")
			require.Len(t, warnings, 1, "an out-of-range value is reported")
			assert.Contains(t, warnings[0], "workflow."+tc.field)
			assert.Contains(t, warnings[0], "out of range")

			wf, warnings = resolveProjectWorkflow(t, `{"`+tc.field+`":`+tc.above+`}`)
			assert.Equal(t, tc.defaultVal, tc.get(wf), "one step above the high bound falls back to the default")
			require.Len(t, warnings, 1, "an out-of-range value is reported")
			assert.Contains(t, warnings[0], "workflow."+tc.field)
		})
	}
}

func TestRunFields_TaskOverrideBeatsProjectBeatsDefault(t *testing.T) {
	project := model.WorkflowOverride{
		RunMaxUnits:            ptr(200),
		RunMaxWallClockSeconds: ptr(600),
		RunMaxBudgetUSD:        ptr(50.0),
		RunMaxUnitBudgetUSD:    ptr(2.5),
		RunMaxUnitRetries:      ptr(3),
		Runner:                 []byte(`"opencode"`),
	}
	override := model.WorkflowOverride{
		RunMaxUnits:     ptr(7),
		RunMaxBudgetUSD: ptr(1.25),
		Runner:          []byte(`{"cmd":"my-agent"}`),
	}

	wf := ResolveWorkflowLayers(&override, &project)
	assert.Equal(t, 7, wf.RunMaxUnits, "task override wins")
	assert.Equal(t, 1.25, wf.RunMaxBudgetUSD, "task override wins")
	assert.JSONEq(t, `{"cmd":"my-agent"}`, string(wf.Runner), "task override wins")
	assert.Equal(t, 600, wf.RunMaxWallClockSeconds, "project wins where the override is absent")
	assert.Equal(t, 2.5, wf.RunMaxUnitBudgetUSD, "project wins where the override is absent")
	assert.Equal(t, 3, wf.RunMaxUnitRetries, "project wins where the override is absent")

	// Presence, not value, defines an override: an explicit 0 retries wins over
	// a project value even though 0 is also the low end of the range.
	zero := model.WorkflowOverride{RunMaxUnitRetries: ptr(0)}
	assert.Equal(t, 0, ResolveWorkflowLayers(&zero, &project).RunMaxUnitRetries,
		"an explicit 0 is an override, not an absent field")
}

func TestRunFields_RunnerShapesStoredRaw(t *testing.T) {
	for _, raw := range []string{
		`"opencode"`,
		`{"cmd":"claude","args":["-p","{prompt}"],"spend_key":"total_cost_usd"}`,
		`{"audit-role":"opencode","default":"claude"}`,
	} {
		wf, warnings := resolveProjectWorkflow(t, `{"runner":`+raw+`}`)
		assert.Empty(t, warnings, "every runner shape parses; the resolver, not this layer, judges it")
		assert.JSONEq(t, raw, string(wf.Runner), "the runner value is carried through unchanged")
	}
}

func TestNotifyCmd_ReadFromLocalConfigOnly(t *testing.T) {
	dir := projectDir(t, `{"notify_cmd":"say-project"}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "local.json"),
		[]byte(`{"notify_cmd":"say-local"}`), 0o600))

	wf, warnings, err := ResolveEffectiveWorkflow(dir, &model.WorkflowOverride{})
	require.NoError(t, err)
	assert.Equal(t, "say-local", wf.NotifyCmd, "notify_cmd resolves from .tp/local.json")
	assert.Contains(t, warnings, "unknown workflow key: notify_cmd",
		"a project-config workflow block cannot supply notify_cmd")

	assert.Equal(t, "say-local", LocalNotifyCmd(dir))
}

func TestNotifyCmd_AbsentAndWrongType(t *testing.T) {
	dir := projectDir(t, `{}`)
	wf, _, err := ResolveEffectiveWorkflow(dir, &model.WorkflowOverride{})
	require.NoError(t, err)
	assert.Empty(t, wf.NotifyCmd, "no local.json means no notify_cmd")

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "local.json"),
		[]byte(`{"notify_cmd":42}`), 0o600))
	lc, warnings, err := LoadLocalConfig(filepath.Join(dir, ".tp"))
	require.NoError(t, err)
	assert.Nil(t, lc.NotifyCmd, "a wrong-typed notify_cmd is ignored")
	assert.Contains(t, warnings, "notify_cmd: expected a string, ignored")
	assert.NotContains(t, warnings, "unknown top-level key: notify_cmd",
		"notify_cmd is a known local.json key")
}

func TestRunFields_WrongTypeIgnored(t *testing.T) {
	wf, warnings := resolveProjectWorkflow(t, `{"run_max_units":"many","run_max_budget_usd":"free"}`)
	assert.Equal(t, 100, wf.RunMaxUnits, "a wrong-typed value falls back to the default")
	assert.Equal(t, 0.0, wf.RunMaxBudgetUSD)
	assert.Contains(t, warnings, "workflow.run_max_units: expected a number, ignored")
	assert.Contains(t, warnings, "workflow.run_max_budget_usd: expected a number, ignored")
}

func TestRunFields_OverridePresenceTracked(t *testing.T) {
	var empty model.WorkflowOverride
	assert.True(t, empty.IsEmpty(), "a bare override sets nothing")

	assert.False(t, (&model.WorkflowOverride{RunMaxUnits: ptr(1)}).IsEmpty())
	assert.False(t, (&model.WorkflowOverride{RunMaxWallClockSeconds: ptr(60)}).IsEmpty())
	assert.False(t, (&model.WorkflowOverride{RunMaxBudgetUSD: ptr(0.0)}).IsEmpty())
	assert.False(t, (&model.WorkflowOverride{RunMaxUnitBudgetUSD: ptr(0.0)}).IsEmpty())
	assert.False(t, (&model.WorkflowOverride{RunMaxUnitRetries: ptr(0)}).IsEmpty())
	assert.False(t, (&model.WorkflowOverride{Runner: []byte(`"claude"`)}).IsEmpty())
}
