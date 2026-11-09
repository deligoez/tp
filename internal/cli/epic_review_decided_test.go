package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicReview_AReaskedUnitSeesItsEarlierDecisions: b's finding was
// resolved wontfix in round 1, and b is asked again in round 2 only because
// its dependency c changed. Every role's prompt names the decision and its
// evidence, so a role does not raise it again; a unit with no decision gets
// no such block.
func TestEpicReview_AReaskedUnitSeesItsEarlierDecisions(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)

	r1 := runEpicReview(t, dir, "--milestone", "A")
	writeEpicRound(t, dir, &r1,
		`{"severity":"medium","finding":"b needs a parser of its own","location":"epic:b","evidence":"b needs c",`+
			`"resolved":{"status":"wontfix","evidence":"the parser is c's by design"}}`+"\n")

	editEpicGraph(t, dir, "c needs d", "c needs d's schema")
	r2 := runEpicReview(t, dir, "--milestone", "A")
	require.Contains(t, r2.Units, "epic:b", "b is asked again through its changed dependency")
	require.NotEmpty(t, r2.Prompts)
	for _, p := range r2.Prompts {
		assert.Contains(t, p.Prompt, "b needs a parser of its own", "%s sees the decided finding", p.Role)
		assert.Contains(t, p.Prompt, "the parser is c's by design", "%s sees the decision's evidence", p.Role)
	}

	fresh := epicGraphDir(t, epicChain)
	out := runEpicReview(t, fresh, "--milestone", "A")
	for _, p := range out.Prompts {
		assert.NotContains(t, p.Prompt, "Already decided", "%s: no decision, no block", p.Role)
	}
}
