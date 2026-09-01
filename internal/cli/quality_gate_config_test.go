package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoQualityGate is the literal gate every task file in this repo must resolve
// to (v0.34.0 §5, extended with the deadcode and complexity steps). It is
// spelled out here rather than read from .tp/config.json on purpose: comparing
// the resolved value against the project value passes when the project gate
// itself weakens, and a substring check for "-race" passes when the rest of the
// gate changes.
//
// The third step closes a hole the first two cannot see: golangci-lint's
// `unused` skips exported identifiers by design, so an exported function with
// no callers passes both. Dropping it would restore that blindness silently,
// which is why it belongs in the literal rather than in prose somewhere.
//
// The fourth is a ratchet rather than a threshold: scripts/check-complexity.sh
// fails on a cognitive-complexity violation absent from its committed baseline,
// and equally on a baseline entry that no longer violates — so the baseline can
// only shrink. A plain gocognit linter entry would have to fail the 54 existing
// functions on day one, or sit at a threshold high enough to measure nothing.
const repoQualityGate = "./scripts/check-suite-state.sh && golangci-lint run && ./scripts/check-deadcode.sh && ./scripts/check-complexity.sh"

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

// TestSuiteStateWrapperStillRunsTheRaceDetector keeps the guard above able to
// see what it is named for.
//
// v0.36.0 §6.1 moved the suite behind scripts/check-suite-state.sh, so the gate
// literal no longer contains `-race` — the flag is inside the script now. A
// literal comparison alone would go on passing while the wrapper quietly
// dropped it, which is the exact failure mode v0.34.0 §5 added the guard for.
// Naming a thing is not the same as being able to observe it.
func TestSuiteStateWrapperStillRunsTheRaceDetector(t *testing.T) {
	script := filepath.Join(repoRoot(t), "scripts", "check-suite-state.sh")
	body, err := os.ReadFile(script) //nolint:gosec // a fixed path inside the repo under test
	require.NoError(t, err, "the gate's first step must exist")

	// The assertion is over the script's `go test` INVOCATION LINES, not over
	// the file text, and that distinction was measured rather than assumed. A
	// bare assert.Contains(body, "-count=1") does not discriminate: the flag
	// appears in two branches and again in the comment explaining it, so
	// deleting it from one branch leaves the assertion true. That mutation was
	// run — dropping it from the ./... branch left a Contains-based guard GREEN
	// while dropping -race turned it red, which is the same defect this cycle
	// found eight times: an assertion whose falsifying input was never built.
	//
	// Both flags fail in opposite directions and every invocation needs both.
	// Without -race the repo gate stops running the race detector. Without
	// -count=1 `go test` serves a cached PASS for a package it did not run, so
	// the wrapper's before/after digest brackets nothing and reports green with
	// a live-state mutation still in the tree (audit round 8: caught on run 1,
	// "(cached)" and exit 0 on runs 2 and 3, mutation present each time).
	var invocations []string
	for _, line := range strings.Split(string(body), "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "go test ") {
			invocations = append(invocations, trimmed)
		}
	}
	require.Len(t, invocations, 2,
		"the wrapper runs go test in exactly two branches — a narrowed run and the gate's own")
	for _, invocation := range invocations {
		assert.Contains(t, invocation, "-count=1",
			"every branch must defeat the test cache: %q", invocation)
	}
	assert.Contains(t, strings.Join(invocations, "\n"), "-race ./...",
		"the no-argument branch is where -race lives now; it must run the whole suite under it")

	info, statErr := os.Stat(script)
	require.NoError(t, statErr)
	assert.NotZero(t, info.Mode().Perm()&0o111,
		"the gate invokes it as ./scripts/check-suite-state.sh, so it must be executable")
}
