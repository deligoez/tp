package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPhase(t *testing.T) {
	cases := []struct {
		name              string
		numTasks, numDone int
		reviewConv        bool
		auditConv         bool
		want              string
	}{
		{"open task implements", 2, 1, false, false, PhaseImplement},
		{"open task implements even when review unconverged", 1, 0, false, false, PhaseImplement},
		{"all done and audit unconverged audits", 2, 2, false, false, PhaseAudit},
		{"all done and audit converged releases", 2, 2, false, true, PhaseRelease},
		{"zero tasks and review converged decomposes", 0, 0, true, false, PhaseDecompose},
		{"zero tasks and review unconverged reviews", 0, 0, false, false, PhaseReview},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, DetectPhase(tc.numTasks, tc.numDone, tc.reviewConv, tc.auditConv))
		})
	}
}

// setupResumeProject writes a spec and its adjacent task file into a temp dir
// whose empty .git bounds discovery but is not a real repo (so git status yields
// no changes), and returns the dir, spec path, and task file path.
func setupResumeProject(t *testing.T, tasksJSON string) (dir, specPath, taskFilePath string) {
	t.Helper()
	dir = t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	specPath = filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))
	taskFilePath = filepath.Join(dir, "s.tasks.json")
	require.NoError(t, os.WriteFile(taskFilePath, []byte(tasksJSON), 0o600))
	return dir, specPath, taskFilePath
}

// recordRounds records `review` clean review rounds and `audit` clean audit
// rounds; matchHash true stamps the current spec hash (converged, not stale),
// false stamps a mismatching hash (stale).
func recordRounds(t *testing.T, specPath string, review, audit int, matchHash bool) {
	t.Helper()
	hash, err := SpecHash(specPath)
	require.NoError(t, err)
	if !matchHash {
		hash = "sha256:stale"
	}
	st, err := EnsureReviewState(specPath)
	require.NoError(t, err)
	for i := 0; i < review; i++ {
		st.ReviewRounds = append(st.ReviewRounds, ReviewRound{Round: i + 1, Clean: true, SpecHash: hash})
	}
	for i := 0; i < audit; i++ {
		st.AuditRounds = append(st.AuditRounds, ReviewRound{Round: i + 1, Clean: true, SpecHash: hash})
	}
	require.NoError(t, SaveReviewState(specPath, st))
}

func assemble(t *testing.T, dir, specPath, taskFilePath string) ResumeResult {
	t.Helper()
	tf, err := model.ReadTaskFile(taskFilePath)
	require.NoError(t, err)
	res, err := AssembleResume(dir, taskFilePath, specPath, tf)
	require.NoError(t, err)
	return res
}

func TestAssembleResume_ImplementEvenWhenStale(t *testing.T) {
	dir, spec, tfp := setupResumeProject(t, `{"spec":"s.md","tasks":[{"id":"t","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`)
	recordRounds(t, spec, 1, 0, false) // a stale review round
	res := assemble(t, dir, spec, tfp)
	assert.Equal(t, PhaseImplement, res.Phase, "an open task implements even when the spec is stale")
	assert.Contains(t, blockerCodes(res.Blockers), "spec-stale")
}

func TestAssembleResume_AuditWhenAllDoneUnconverged(t *testing.T) {
	dir, spec, tfp := setupResumeProject(t, `{"spec":"s.md","tasks":[{"id":"t","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`)
	res := assemble(t, dir, spec, tfp)
	assert.Equal(t, PhaseAudit, res.Phase)
}

func TestAssembleResume_ReleaseWhenAuditConverged(t *testing.T) {
	dir, spec, tfp := setupResumeProject(t, `{"spec":"s.md","tasks":[{"id":"t","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`)
	recordRounds(t, spec, 0, 2, true) // audit converged
	res := assemble(t, dir, spec, tfp)
	assert.Equal(t, PhaseRelease, res.Phase)
}

func TestAssembleResume_DecomposeWhenReviewConverged(t *testing.T) {
	dir, spec, tfp := setupResumeProject(t, `{"spec":"s.md","tasks":[]}`)
	recordRounds(t, spec, 2, 0, true) // review converged
	res := assemble(t, dir, spec, tfp)
	assert.Equal(t, PhaseDecompose, res.Phase)
}

func TestAssembleResume_ReviewWhenUnconverged(t *testing.T) {
	dir, spec, tfp := setupResumeProject(t, `{"spec":"s.md","tasks":[]}`)
	res := assemble(t, dir, spec, tfp)
	assert.Equal(t, PhaseReview, res.Phase)
}
