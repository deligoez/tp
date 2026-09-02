package cli

import (
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func iptr(v int) *int       { return &v }
func sptr(v string) *string { return &v }

func TestWorkflowDeviations_ReportsDifferingFields(t *testing.T) {
	project := model.WorkflowOverride{
		ReviewMaxRounds:   iptr(8),
		ReviewCleanRounds: iptr(2),
	}
	// Override differs on review_max_rounds; matches clean_rounds; sets an audit
	// cap the project does not (so it is not a deviation).
	override := model.WorkflowOverride{
		ReviewMaxRounds:   iptr(0),
		ReviewCleanRounds: iptr(2),
		AuditMaxRounds:    iptr(5),
	}

	devs := workflowDeviations("chapter-03.tasks.json", &override, &project)
	require.Len(t, devs, 1, "only the differing, project-set field is a deviation")
	d := devs[0]
	assert.Equal(t, "review_max_rounds", d["field"])
	assert.Equal(t, "0", d["override"])
	assert.Equal(t, "8", d["project"])
	assert.Equal(t, "chapter-03.tasks.json", d["file"])
}

func TestWorkflowDeviations_QualityGate(t *testing.T) {
	devs := workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{QualityGate: sptr("make test")},
		&model.WorkflowOverride{QualityGate: sptr("go test ./...")},
	)
	require.Len(t, devs, 1)
	assert.Equal(t, "quality_gate", devs[0]["field"])
}

func TestWorkflowDeviations_ChecksSetEquality(t *testing.T) {
	c1 := model.Check{Class: "a", Cmd: "run-a"}
	c2 := model.Check{Class: "b", Cmd: "run-b"}

	// Reordered but equal → not a deviation.
	devs := workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{Checks: &[]model.Check{c2, c1}},
		&model.WorkflowOverride{Checks: &[]model.Check{c1, c2}},
	)
	assert.Empty(t, devs, "a reordered but equal checks is not a deviation")

	// Different set → deviation reported with entry counts.
	devs = workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{Checks: &[]model.Check{c1}},
		&model.WorkflowOverride{Checks: &[]model.Check{c1, c2}},
	)
	require.Len(t, devs, 1)
	assert.Equal(t, "checks", devs[0]["field"])
	assert.Equal(t, "1 entries", devs[0]["override"])
	assert.Equal(t, "2 entries", devs[0]["project"])
}

func TestWorkflowDeviations_ReviewConvergeOn(t *testing.T) {
	// Both set and differ → deviation. --strict promotes any non-empty
	// deviation set to exit 1 generically, so reporting it here is what arms
	// --strict for review_converge_on.
	devs := workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{ReviewConvergeOn: sptr("all")},
		&model.WorkflowOverride{ReviewConvergeOn: sptr("blocking")},
	)
	require.Len(t, devs, 1)
	assert.Equal(t, "review_converge_on", devs[0]["field"])
	assert.Equal(t, "all", devs[0]["override"])
	assert.Equal(t, "blocking", devs[0]["project"])

	// Equal → no deviation.
	assert.Empty(t, workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{ReviewConvergeOn: sptr("all")},
		&model.WorkflowOverride{ReviewConvergeOn: sptr("all")},
	), "an equal review_converge_on is not a deviation")

	// Project does not set it → no policy, no deviation.
	assert.Empty(t, workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{ReviewConvergeOn: sptr("all")},
		&model.WorkflowOverride{},
	), "a review_converge_on the project does not set is not a deviation")
}

// commit_strategy is a genuine project-config field — .tp/config.json carries
// it and tp init authors a task-file override — so a task file that contradicts
// the project strategy is a deviation like any other.
func TestWorkflowDeviations_CommitStrategy(t *testing.T) {
	devs := workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{CommitStrategy: sptr("builtin")},
		&model.WorkflowOverride{CommitStrategy: sptr("hc")},
	)
	require.Len(t, devs, 1)
	assert.Equal(t, "commit_strategy", devs[0]["field"])
	assert.Equal(t, "builtin", devs[0]["override"])
	assert.Equal(t, "hc", devs[0]["project"])

	// Equal → no deviation.
	assert.Empty(t, workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{CommitStrategy: sptr("hc")},
		&model.WorkflowOverride{CommitStrategy: sptr("hc")},
	), "an equal commit_strategy is not a deviation")

	// Project does not set it → no policy, no deviation.
	assert.Empty(t, workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{CommitStrategy: sptr("hc")},
		&model.WorkflowOverride{},
	), "a commit_strategy the project does not set is not a deviation")
}

// audit_converge_on deviates on its twin's terms: this surface compares the two
// layers and --strict promotes any non-empty deviation set to exit 1, which is
// what arms --strict for the field. It does not grade the literal — §2 places
// that refusal at the write sinks (exit 2) and the consuming audit sinks
// (exit 1) — so an illegal stored value is reported here as the deviation it is.
func TestWorkflowDeviations_AuditConvergeOn(t *testing.T) {
	devs := workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{AuditConvergeOn: sptr("blocking")},
		&model.WorkflowOverride{AuditConvergeOn: sptr("all")},
	)
	require.Len(t, devs, 1)
	assert.Equal(t, "audit_converge_on", devs[0]["field"])
	assert.Equal(t, "blocking", devs[0]["override"])
	assert.Equal(t, "all", devs[0]["project"])

	// An illegal value against a legal project policy is reported on the same
	// terms — the comparison is over values, not over legality.
	illegal := workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{AuditConvergeOn: sptr("nope")},
		&model.WorkflowOverride{AuditConvergeOn: sptr("all")},
	)
	require.Len(t, illegal, 1)
	assert.Equal(t, "audit_converge_on", illegal[0]["field"])
	assert.Equal(t, "nope", illegal[0]["override"])

	// Equal → no deviation.
	assert.Empty(t, workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{AuditConvergeOn: sptr("all")},
		&model.WorkflowOverride{AuditConvergeOn: sptr("all")},
	), "an equal audit_converge_on is not a deviation")

	// Project does not set it → no policy, no deviation.
	assert.Empty(t, workflowDeviations("x.tasks.json",
		&model.WorkflowOverride{AuditConvergeOn: sptr("all")},
		&model.WorkflowOverride{},
	), "an audit_converge_on the project does not set is not a deviation")
}

// relOrSelf never drops a path: a location filepath.Rel cannot express relative
// to root is reported whole rather than as the empty string the discarded-error
// form produced.
func TestRelOrSelf_FallsBackToTheWholePath(t *testing.T) {
	assert.Equal(t, filepath.Join("b", "x.tasks.json"),
		relOrSelf(filepath.FromSlash("/a"), filepath.FromSlash("/a/b/x.tasks.json")))

	abs := filepath.FromSlash("/abs/x.tasks.json")
	assert.Equal(t, abs, relOrSelf(filepath.FromSlash("relative/root"), abs),
		"a path that cannot be made relative to root is reported whole")
}
