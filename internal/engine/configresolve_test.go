package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestResolveWorkflowLayers_Ranking(t *testing.T) {
	// Override outranks project outranks built-in default, per field.
	override := model.WorkflowOverride{ReviewMaxRounds: ptr(5)}
	project := model.WorkflowOverride{ReviewMaxRounds: ptr(8), AuditMaxRounds: ptr(9)}

	wf := ResolveWorkflowLayers(override, project)
	assert.Equal(t, 5, wf.ReviewMaxRounds, "task override wins over project")
	assert.Equal(t, 9, wf.AuditMaxRounds, "project wins where override is absent")
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default wins where both layers are absent")
	assert.Equal(t, 600, wf.GateTimeoutSeconds, "built-in default gate timeout")
}

func TestResolveWorkflowLayers_QualityGatePrecedence(t *testing.T) {
	wf := ResolveWorkflowLayers(
		model.WorkflowOverride{},
		model.WorkflowOverride{QualityGate: ptr("make test")},
	)
	assert.Equal(t, "make test", wf.QualityGate, "project quality_gate applies when no task override")

	wf = ResolveWorkflowLayers(
		model.WorkflowOverride{QualityGate: ptr("go test ./...")},
		model.WorkflowOverride{QualityGate: ptr("make test")},
	)
	assert.Equal(t, "go test ./...", wf.QualityGate, "task override wins")
}

func TestResolveWorkflowLayers_CommitStrategy(t *testing.T) {
	// Parsed from a task-file workflow block, then resolved by precedence.
	wo, warnings := parseWorkflowOverride([]byte(`{"commit_strategy":"squash"}`))
	require.Empty(t, warnings)
	require.NotNil(t, wo.CommitStrategy)
	assert.Equal(t, "squash", *wo.CommitStrategy)

	wf := ResolveWorkflowLayers(wo, model.WorkflowOverride{CommitStrategy: ptr("rebase")})
	assert.Equal(t, "squash", wf.CommitStrategy, "task override wins over project")

	wf = ResolveWorkflowLayers(model.WorkflowOverride{}, model.WorkflowOverride{CommitStrategy: ptr("rebase")})
	assert.Equal(t, "rebase", wf.CommitStrategy, "project applies when no task override")

	wf = ResolveWorkflowLayers(model.WorkflowOverride{}, model.WorkflowOverride{})
	assert.Equal(t, "", wf.CommitStrategy, "built-in default is empty")
}

func TestResolveEffectiveWorkflow_SparseMerge(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	tp := filepath.Join(root, ".tp")
	require.NoError(t, os.Mkdir(tp, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":8,"gate_timeout_seconds":1200,"review_clean_rounds":3}}`), 0o600))

	// The task file sets only review_max_rounds; it inherits the rest.
	wf, warns, err := ResolveEffectiveWorkflow(root, model.WorkflowOverride{ReviewMaxRounds: ptr(0)})
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, 0, wf.ReviewMaxRounds, "task override (explicit 0) wins")
	assert.Equal(t, 1200, wf.GateTimeoutSeconds, "inherited from project")
	assert.Equal(t, 3, wf.ReviewCleanRounds, "inherited from project")
	assert.Equal(t, 2, wf.AuditCleanRounds, "built-in default where neither layer sets it")
}

func TestResolveEffectiveWorkflow_NoConfigIsV023(t *testing.T) {
	root := t.TempDir() // no .tp/
	wf, _, err := ResolveEffectiveWorkflow(root, model.WorkflowOverride{ReviewMaxRounds: ptr(4)})
	require.NoError(t, err)
	assert.Equal(t, 4, wf.ReviewMaxRounds)
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default with no project config")
	assert.Equal(t, 600, wf.GateTimeoutSeconds)
}

func TestResolveWorkflowLayers_PresenceZeroWins(t *testing.T) {
	// review_max_rounds:0 (explicit no-cap) is a present override that must win
	// over a non-zero project value, not be mistaken for absent.
	wf := ResolveWorkflowLayers(
		model.WorkflowOverride{ReviewMaxRounds: ptr(0)},
		model.WorkflowOverride{ReviewMaxRounds: ptr(8)},
	)
	assert.Equal(t, 0, wf.ReviewMaxRounds, "explicit 0 override wins over project 8")
}

func TestResolveWorkflowLayers_ChecksReplaceSemantics(t *testing.T) {
	projChecks := model.WorkflowOverride{Checks: &[]model.Check{{Class: "x", Cmd: "run-x"}}}

	// A present empty checks array replaces the project checks with nothing.
	empty := []model.Check{}
	wf := ResolveWorkflowLayers(model.WorkflowOverride{Checks: &empty}, projChecks)
	assert.Empty(t, wf.Checks, "present empty checks replaces project checks")

	// An absent checks key inherits the project checks.
	wf = ResolveWorkflowLayers(model.WorkflowOverride{}, projChecks)
	require.Len(t, wf.Checks, 1)
	assert.Equal(t, "x", wf.Checks[0].Class, "absent checks inherits project checks")
}

func TestResolveEffectiveWorkflow_AnchoredAtStartNoCrossProject(t *testing.T) {
	mkProject := func(t *testing.T, maxRounds string) string {
		t.Helper()
		root := t.TempDir()
		require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
		tp := filepath.Join(root, ".tp")
		require.NoError(t, os.Mkdir(tp, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
			[]byte(`{"workflow":{"review_max_rounds":`+maxRounds+`}}`), 0o600))
		return root
	}
	projA := mkProject(t, "8")
	_ = mkProject(t, "3") // a separate project B with a different policy

	// Resolution is anchored once at the start dir: projA's config applies and
	// is never re-anchored to another project's config (no cross-project merge).
	wf, _, err := ResolveEffectiveWorkflow(projA, model.WorkflowOverride{})
	require.NoError(t, err)
	assert.Equal(t, 8, wf.ReviewMaxRounds, "derive-at-read from the single anchored project")
}

func TestResolveWorkflow_ThinnedTaskInheritsProjectPolicy(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_clean_rounds":3,"review_max_rounds":7}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.md"), []byte("# S\n"), 0o600))
	// A thinned task file: its workflow block omits the convergence policy.
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{}}`), 0o600))

	t.Chdir(root)
	wf, _ := ResolveWorkflow("s.md", "s.tasks.json")
	assert.Equal(t, 3, wf.ReviewCleanRounds, "import enforcement resolves the inherited project clean_rounds")
	assert.Equal(t, 7, wf.ReviewMaxRounds, "and the inherited project cap")
}

func TestEffectiveWorkflowForTaskFile_InheritsProjectQualityGate(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"quality_gate":"make check","gate_timeout_seconds":900}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{}}`), 0o600))

	t.Chdir(root)
	wf := EffectiveWorkflowForTaskFile("s.tasks.json")
	assert.Equal(t, "make check", wf.QualityGate, "a task file omitting quality_gate runs the project gate")
	assert.Equal(t, 900, wf.GateTimeoutSeconds, "and the inherited project timeout")
}

func TestResolveWorkflow_BudgetCapsInherited(t *testing.T) {
	// The review and audit round-budget checks read ResolveWorkflow, so a
	// thinned task file inherits both project caps for budget enforcement.
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":6,"audit_max_rounds":9}}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{}}`), 0o600))

	t.Chdir(root)
	wf, _ := ResolveWorkflow("s.md", "s.tasks.json")
	assert.Equal(t, 6, wf.ReviewMaxRounds, "review budget uses the inherited project cap")
	assert.Equal(t, 9, wf.AuditMaxRounds, "audit budget uses the inherited project cap")
}

func TestDiscoverTaskFile_LocalActiveOverLegacy(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "new.tasks.json"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"),
		[]byte(`{"active":"new.tasks.json"}`), 0o600))
	// A legacy marker points elsewhere; the project-local pointer must win.
	require.NoError(t, os.WriteFile(filepath.Join(root, "old.tasks.json"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp-active"), []byte("old.tasks.json"), 0o600))

	got, err := DiscoverTaskFile(root, "")
	require.NoError(t, err)
	assert.Equal(t, evalLink(t, filepath.Join(root, "new.tasks.json")), evalLink(t, got),
		"the .tp/local.json active pointer wins over the legacy .tp-active")
}

func TestResolveLocalActive_RejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"),
		[]byte(`{"active":"/etc/passwd"}`), 0o600))
	assert.Empty(t, ResolveLocalActive(root), "an absolute active value is rejected and treated as unset")
}

func TestDiscoverTaskFile_DanglingActiveFallsThrough(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "local.json"),
		[]byte(`{"active":"gone.tasks.json"}`), 0o600)) // points at a missing file
	require.NoError(t, os.WriteFile(filepath.Join(root, "real.tasks.json"), []byte("{}"), 0o600))

	got, err := DiscoverTaskFile(root, "")
	require.NoError(t, err)
	assert.Contains(t, got, "real.tasks.json", "a dangling active pointer falls through to auto-detect")
}

func TestDiscoverTaskFile_LegacyActiveStillWorks(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "leg.tasks.json"), []byte("{}"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp-active"), []byte("leg.tasks.json\n"), 0o600))

	got, err := DiscoverTaskFile(root, "")
	require.NoError(t, err)
	assert.Contains(t, got, "leg.tasks.json", "the legacy .tp-active fallback still resolves through v0.24.x")
}

func TestEffectiveWorkflowForTaskFile_InheritsProjectCommitStrategy(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":{"commit_strategy":"hc"}}`), 0o600))
	// A thinned task file: its workflow block omits commit_strategy.
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{}}`), 0o600))

	t.Chdir(root)
	wf := EffectiveWorkflowForTaskFile("s.tasks.json")
	assert.Equal(t, "hc", wf.CommitStrategy, "a thinned task file inherits the project commit_strategy")
}
