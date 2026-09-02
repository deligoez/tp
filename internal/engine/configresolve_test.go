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

	wf := ResolveWorkflowLayers(&override, &project)
	assert.Equal(t, 5, wf.ReviewMaxRounds, "task override wins over project")
	assert.Equal(t, 9, wf.AuditMaxRounds, "project wins where override is absent")
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default wins where both layers are absent")
	assert.Equal(t, 600, wf.GateTimeoutSeconds, "built-in default gate timeout")
	assert.Equal(t, 5, wf.LockTimeoutSeconds, "built-in default lock timeout")
}

func TestResolveWorkflowLayers_QualityGatePrecedence(t *testing.T) {
	wf := ResolveWorkflowLayers(
		&model.WorkflowOverride{},
		&model.WorkflowOverride{QualityGate: ptr("make test")},
	)
	assert.Equal(t, "make test", wf.QualityGate, "project quality_gate applies when no task override")

	wf = ResolveWorkflowLayers(
		&model.WorkflowOverride{QualityGate: ptr("go test ./...")},
		&model.WorkflowOverride{QualityGate: ptr("make test")},
	)
	assert.Equal(t, "go test ./...", wf.QualityGate, "task override wins")
}

func TestResolveWorkflowLayers_CommitStrategy(t *testing.T) {
	// Parsed from a task-file workflow block, then resolved by precedence.
	wo, warnings := parseWorkflowOverride([]byte(`{"commit_strategy":"squash"}`))
	require.Empty(t, warnings)
	require.NotNil(t, wo.CommitStrategy)
	assert.Equal(t, "squash", *wo.CommitStrategy)

	wf := ResolveWorkflowLayers(&wo, &model.WorkflowOverride{CommitStrategy: ptr("rebase")})
	assert.Equal(t, "squash", wf.CommitStrategy, "task override wins over project")

	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{CommitStrategy: ptr("rebase")})
	assert.Equal(t, "rebase", wf.CommitStrategy, "project applies when no task override")

	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
	assert.Equal(t, "", wf.CommitStrategy, "built-in default is empty")
}

func TestResolveWorkflowLayers_ReviewConvergeOn(t *testing.T) {
	// Parsed from a task-file workflow block, then resolved by precedence.
	wo, warnings := parseWorkflowOverride([]byte(`{"review_converge_on":"all"}`))
	require.Empty(t, warnings)
	require.NotNil(t, wo.ReviewConvergeOn)
	assert.Equal(t, "all", *wo.ReviewConvergeOn)

	wf := ResolveWorkflowLayers(&wo, &model.WorkflowOverride{ReviewConvergeOn: ptr("blocking")})
	assert.Equal(t, "all", wf.ReviewConvergeOn, "task override wins over project")

	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{ReviewConvergeOn: ptr("all")})
	assert.Equal(t, "all", wf.ReviewConvergeOn, "project applies when no task override")

	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
	assert.Equal(t, ReviewConvergeOnBlocking, wf.ReviewConvergeOn, "built-in default is blocking")

	// A stored bad value is preserved raw (not rejected) at resolution — a
	// consuming command validates it, config --resolved surfaces it (§3.3).
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{ReviewConvergeOn: ptr("bogus")}, &model.WorkflowOverride{})
	assert.Equal(t, "bogus", wf.ReviewConvergeOn, "an invalid stored value resolves raw, not clamped")
}

// TestResolveWorkflowLayers_AuditConvergeOn covers v0.37.0 §2: audit_converge_on
// parses out of a workflow block with the same string-type warning as its twin,
// resolves task override > project config > built-in, and its built-in is all.
// The default assertion is the one v0.37.0 §7 row 1 names: the mutant that must
// fail it is shipping blocking — review_converge_on's default — as this field's.
func TestResolveWorkflowLayers_AuditConvergeOn(t *testing.T) {
	// Parsed from a task-file workflow block, then resolved by precedence.
	wo, warnings := parseWorkflowOverride([]byte(`{"audit_converge_on":"blocking"}`))
	require.Empty(t, warnings)
	require.NotNil(t, wo.AuditConvergeOn)
	assert.Equal(t, AuditConvergeOnBlocking, *wo.AuditConvergeOn)

	wf := ResolveWorkflowLayers(&wo, &model.WorkflowOverride{AuditConvergeOn: ptr(AuditConvergeOnAll)})
	assert.Equal(t, AuditConvergeOnBlocking, wf.AuditConvergeOn, "task override wins over project")

	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{AuditConvergeOn: ptr(AuditConvergeOnBlocking)})
	assert.Equal(t, AuditConvergeOnBlocking, wf.AuditConvergeOn, "project applies when no task override")

	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
	assert.Equal(t, AuditConvergeOnAll, wf.AuditConvergeOn, "built-in default is all, not the twin's blocking")

	// A non-string value is collected as a warning and leaves the field unset,
	// on the same terms as review_converge_on's parse.
	bad, warnings := parseWorkflowOverride([]byte(`{"audit_converge_on":7}`))
	assert.Nil(t, bad.AuditConvergeOn, "a wrong-typed value leaves the field unset")
	assert.Contains(t, warnings, "workflow.audit_converge_on: expected a string, ignored")

	// A stored bad value is preserved raw (not rejected) at resolution — a
	// consuming command validates it, config --resolved surfaces it (§2).
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{AuditConvergeOn: ptr("bogus")}, &model.WorkflowOverride{})
	assert.Equal(t, "bogus", wf.AuditConvergeOn, "an invalid stored value resolves raw, not clamped")
}

// TestResolveWorkflowLayers_AuditConvergeOnConsultsExactlyTwoLayers covers
// v0.37.0 §7 row 2: the resolver consults task override > project config >
// built-in and nothing else. The mutant that must fail it is adding an
// environment layer to ResolveWorkflowLayers, at any rank.
//
// The environment subtest is the half that carries the row. Asserting a
// resolved value with no TP_AUDIT_CONVERGE_ON set is a tautology — it passes
// whether or not such a layer exists — so the variable is set here to the value
// neither real layer names, and every case then answers with what the two real
// layers say. pickString walks its list in order and falls through to the
// default only when every entry is nil, so an environment entry inserted
// anywhere in that list wins the third case; the first two pin the ranks above
// it as well. The precedence subtest deliberately leaves the environment alone,
// which is what makes the pair discriminating: under the mutant it stays green
// while the environment subtest reddens.
func TestResolveWorkflowLayers_AuditConvergeOnConsultsExactlyTwoLayers(t *testing.T) {
	all := ptr(AuditConvergeOnAll)
	blocking := ptr(AuditConvergeOnBlocking)

	t.Run("the two layers rank", func(t *testing.T) {
		wf := ResolveWorkflowLayers(
			&model.WorkflowOverride{AuditConvergeOn: all},
			&model.WorkflowOverride{AuditConvergeOn: blocking},
		)
		assert.Equal(t, AuditConvergeOnAll, wf.AuditConvergeOn, "task override beats project config")

		wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{AuditConvergeOn: blocking})
		assert.Equal(t, AuditConvergeOnBlocking, wf.AuditConvergeOn, "project config beats the built-in")

		wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
		assert.Equal(t, AuditConvergeOnAll, wf.AuditConvergeOn, "the built-in answers when neither layer sets it")
	})

	t.Run("there is no environment layer", func(t *testing.T) {
		t.Setenv("TP_AUDIT_CONVERGE_ON", AuditConvergeOnBlocking)

		wf := ResolveWorkflowLayers(
			&model.WorkflowOverride{AuditConvergeOn: all},
			&model.WorkflowOverride{AuditConvergeOn: all},
		)
		assert.Equal(t, AuditConvergeOnAll, wf.AuditConvergeOn, "the environment does not outrank the task override")

		wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{AuditConvergeOn: all})
		assert.Equal(t, AuditConvergeOnAll, wf.AuditConvergeOn, "the environment does not outrank the project config")

		wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &model.WorkflowOverride{})
		assert.Equal(t, AuditConvergeOnAll, wf.AuditConvergeOn, "the environment does not displace the built-in")
	})
}

func TestResolveEffectiveWorkflow_SparseMerge(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	tp := filepath.Join(root, ".tp")
	require.NoError(t, os.Mkdir(tp, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tp, "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":8,"gate_timeout_seconds":1200,"review_clean_rounds":3}}`), 0o600))

	// The task file sets only review_max_rounds; it inherits the rest.
	wf, warns, err := ResolveEffectiveWorkflow(root, &model.WorkflowOverride{ReviewMaxRounds: ptr(0)})
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, 0, wf.ReviewMaxRounds, "task override (explicit 0) wins")
	assert.Equal(t, 1200, wf.GateTimeoutSeconds, "inherited from project")
	assert.Equal(t, 3, wf.ReviewCleanRounds, "inherited from project")
	assert.Equal(t, 2, wf.AuditCleanRounds, "built-in default where neither layer sets it")
}

func TestResolveEffectiveWorkflow_NoConfigIsV023(t *testing.T) {
	root := t.TempDir() // no .tp/
	wf, _, err := ResolveEffectiveWorkflow(root, &model.WorkflowOverride{ReviewMaxRounds: ptr(4)})
	require.NoError(t, err)
	assert.Equal(t, 4, wf.ReviewMaxRounds)
	assert.Equal(t, 2, wf.ReviewCleanRounds, "built-in default with no project config")
	assert.Equal(t, 600, wf.GateTimeoutSeconds)
}

func TestResolveWorkflowLayers_PresenceZeroWins(t *testing.T) {
	// review_max_rounds:0 (explicit no-cap) is a present override that must win
	// over a non-zero project value, not be mistaken for absent.
	wf := ResolveWorkflowLayers(
		&model.WorkflowOverride{ReviewMaxRounds: ptr(0)},
		&model.WorkflowOverride{ReviewMaxRounds: ptr(8)},
	)
	assert.Equal(t, 0, wf.ReviewMaxRounds, "explicit 0 override wins over project 8")
}

func TestResolveWorkflowLayers_ChecksReplaceSemantics(t *testing.T) {
	projChecks := model.WorkflowOverride{Checks: &[]model.Check{{Class: "x", Cmd: "run-x"}}}

	// A present empty checks array replaces the project checks with nothing.
	empty := []model.Check{}
	wf := ResolveWorkflowLayers(&model.WorkflowOverride{Checks: &empty}, &projChecks)
	assert.Empty(t, wf.Checks, "present empty checks replaces project checks")

	// An absent checks key inherits the project checks.
	wf = ResolveWorkflowLayers(&model.WorkflowOverride{}, &projChecks)
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
	wf, _, err := ResolveEffectiveWorkflow(projA, &model.WorkflowOverride{})
	require.NoError(t, err)
	assert.Equal(t, 8, wf.ReviewMaxRounds, "derive-at-read from the single anchored project")
}

// thinnedProjectRoot creates a repo root carrying .tp/config.json with the
// given project workflow JSON, a spec, and a THINNED task file whose workflow
// block is empty, then chdirs into it. The inheritance tests below differ only
// in that JSON and in the resolved fields they assert.
func thinnedProjectRoot(t *testing.T, projectWorkflow string) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(root, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"),
		[]byte(`{"workflow":`+projectWorkflow+`}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "s.tasks.json"),
		[]byte(`{"spec":"s.md","tasks":[],"workflow":{}}`), 0o600))
	t.Chdir(root)
}

func TestResolveWorkflow_ThinnedTaskInheritsProjectPolicy(t *testing.T) {
	thinnedProjectRoot(t, `{"review_clean_rounds":3,"review_max_rounds":7}`)

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
	thinnedProjectRoot(t, `{"review_max_rounds":6,"audit_max_rounds":9}`)

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
