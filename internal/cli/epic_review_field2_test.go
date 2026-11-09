package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicReview_ADecisionCarriesItsRound: a decision shown a round later
// names the round it was made in, so evidence that says "this round" still
// points somewhere a reader can find.
func TestEpicReview_ADecisionCarriesItsRound(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	r1 := runEpicReview(t, dir, "--milestone", "A")
	writeEpicRound(t, dir, &r1,
		`{"severity":"low","finding":"b repeats a's check","location":"epic:b","evidence":"b",`+
			`"resolved":{"status":"duplicate","evidence":"same defect as row 0 of this round"}}`+"\n")
	editEpicGraph(t, dir, "c needs d", "c needs d's schema")
	r2 := runEpicReview(t, dir, "--milestone", "A")
	require.NotEmpty(t, r2.Prompts)
	for _, p := range r2.Prompts {
		assert.Contains(t, p.Prompt, "round 1, duplicate", "%s", p.Role)
	}
}

// TestEpicReview_RegressionSeesWhatChanged: after b's why is reworded, the
// regression prompt shows b's block as round 1 read it and as it reads now,
// so the role need not reconstruct the change from history. Snapshots that
// keep only hashes leave it nothing to show.
func TestEpicReview_RegressionSeesWhatChanged(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	r1 := runEpicReview(t, dir, "--milestone", "A")
	writeEpicRound(t, dir, &r1, "")
	editEpicGraph(t, dir, "b needs c", "b needs c's parser")
	r2 := runEpicReview(t, dir, "--milestone", "A")
	var regression string
	for _, p := range r2.Prompts {
		if p.Role == "regression" {
			regression = p.Prompt
		}
	}
	require.NotEmpty(t, regression, "a changed round appends regression")
	i := strings.Index(regression, "Changed since round 1")
	require.NotEqual(t, -1, i)
	after := regression[i:]
	assert.Contains(t, after, "- c: b needs c\n", "the block round 1 read")
	assert.Contains(t, after, "- c: b needs c's parser\n", "the block now")
}

// TestEpicReview_ALowFixTailIsVerifiedByRegressionAlone: when every finding
// of the round before is decided and none it fixed was blocking, the next
// round still reads the change but asks only the regression role; a blocking
// fix brings the whole panel back. A rule keyed on "any fix" alone would run
// the panel after a low fix, and one that ignored severity would skip it after
// a high one.
func TestEpicReview_ALowFixTailIsVerifiedByRegressionAlone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		severity string
		roles    []string
	}{
		{"low", []string{"regression"}},
		{"high", []string{"dod-satisfiability", "why-auditor", "phase-economist", "regression"}},
	} {
		t.Run(tc.severity, func(t *testing.T) {
			t.Parallel()
			dir := epicGraphDir(t, epicChain)
			r1 := runEpicReview(t, dir, "--milestone", "A")
			writeEpicRound(t, dir, &r1,
				`{"severity":"`+tc.severity+`","finding":"b's why is generic","location":"epic:b","evidence":"b needs c",`+
					`"resolved":{"status":"fixed","evidence":"reworded"}}`+"\n")
			editEpicGraph(t, dir, "b needs c", "b needs c's parser")
			r2 := runEpicReview(t, dir, "--milestone", "A")
			assert.Equal(t, tc.roles, promptRoles(&r2))
		})
	}
}
