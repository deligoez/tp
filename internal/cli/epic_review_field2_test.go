package cli_test

import (
	"encoding/json"
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

// TestEpicReview_RegressionSeesTheEpicAFixHandsWorkTo: b's acceptance is
// reworded to hand a run to d, an epic outside b's slice and context. The
// regression prompt shows d's block, so it can check d takes the work; the
// other roles' prompts do not.
func TestEpicReview_RegressionSeesTheEpicAFixHandsWorkTo(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	r1 := runEpicReview(t, dir, "--epic", "b")
	writeEpicRound(t, dir, &r1, "")
	editEpicGraph(t, dir, `{"id": "b", "title": "B", "milestone": "A",`,
		`{"id": "b", "title": "B", "milestone": "A", "acceptance": "b hands its export run to d",`)
	r2 := runEpicReview(t, dir, "--epic", "b")
	for _, p := range r2.Prompts {
		if p.Role == "regression" {
			assert.Contains(t, p.Prompt, "epic: d\n", "regression sees the handoff target")
		} else {
			assert.NotContains(t, p.Prompt, "epic: d\n", "%s sees no handoff block", p.Role)
		}
	}
	assert.Contains(t, promptRoles(&r2), "regression")
}

// TestEpicReviewStatus_NamesWhatIsStillOpen: under the default
// review_converge_on (blocking) an open medium finding does not hold a round
// open, so the slice can be done with it still open; --status says so with
// the policy it converged under and the last round's open findings by
// severity.
func TestEpicReviewStatus_NamesWhatIsStillOpen(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	writeEpicProjectConfig(t, dir, `{"review_clean_rounds": 1}`)
	slice := []string{"--epic", "b"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, `{"severity":"medium","finding":"b's why is loose","location":"epic:b","evidence":"b needs c"}`+"\n", slice...)

	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, append(slice, "--status")...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var st struct {
		Done       bool           `json:"done"`
		ConvergeOn string         `json:"converge_on"`
		Open       map[string]int `json:"open"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &st), "stdout: %s", stdout)
	assert.True(t, st.Done, "a medium finding does not block under blocking")
	assert.Equal(t, "blocking", st.ConvergeOn)
	assert.Equal(t, map[string]int{"critical": 0, "high": 0, "medium": 1, "low": 0}, st.Open)
}

// TestEpicReview_RegressionWithoutFixesSaysSo: a round that follows an edit
// rather than a resolved fix tells regression there are no fixed findings to
// confirm, instead of pointing at a list that is not there.
func TestEpicReview_RegressionWithoutFixesSaysSo(t *testing.T) {
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
	require.NotEmpty(t, regression)
	assert.Contains(t, regression, "No finding was resolved fixed since round 1")
	assert.NotContains(t, regression, "each fixed finding below")
}
