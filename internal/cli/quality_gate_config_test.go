package cli

import (
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoQualityGate is the literal gate every task file in this repo must resolve
// to (v0.34.0 §5). It is spelled out here rather than read from
// .tp/config.json on purpose: comparing the resolved value against the project
// value passes when the project gate itself weakens, and a substring check for
// "-race" passes when the rest of the gate changes.
const repoQualityGate = "go test -race ./... && golangci-lint run"

// TestRepoTaskFilesResolveRaceQualityGate guards the v0.34.0 §5 outcome: the
// gate resolved for every task file in this repo — this release's included —
// runs the race detector. Two ways to lose that are checked separately:
//
//   - the resolved value stops being the literal gate, which covers the project
//     gate in .tp/config.json weakening (e.g. -race dropped) as well as a
//     task-level override reintroducing the pre-race gate;
//   - a task file carries a quality_gate override at all, which masks the
//     project layer even while it happens to agree with it — the shape
//     `tp init --quality-gate "…"` used to create on every new task file.
func TestRepoTaskFilesResolveRaceQualityGate(t *testing.T) {
	root := repoRoot(t)
	taskFiles, err := filepath.Glob(filepath.Join(root, "spec", "*.tasks.json"))
	require.NoError(t, err)
	require.NotEmpty(t, taskFiles, "spec/*.tasks.json must exist at the repo root")

	for _, path := range taskFiles {
		rel, relErr := filepath.Rel(root, path)
		require.NoError(t, relErr)

		override, loadErr := engine.LoadTaskWorkflowOverride(path)
		require.NoError(t, loadErr, "%s must parse", rel)
		assert.Nil(t, override.QualityGate,
			"%s must not carry a task-level quality_gate: it masks the project gate", rel)

		workflow, _, resolveErr := engine.ResolveEffectiveWorkflow(root, &override)
		require.NoError(t, resolveErr, "%s must resolve against the project config", rel)
		assert.Equal(t, repoQualityGate, workflow.QualityGate,
			"the gate resolved for %s must be the literal repo gate", rel)
	}
}
