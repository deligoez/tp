package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// groundTestPrompt renders a prompt over a one-row index, for the assertions
// that are about the prompt's frame rather than about the floor in it.
func groundTestPrompt() string {
	return buildGroundPrompt("spec.md", ".tp-review/spec/snapshot-ground-round-7.md",
		"# commit unknown\nu1 §1 0123456789ab #1 12B\n# 1 in floor, 0 cut\n", "ground-r7.ndjson", 7)
}

// TestTheEmittedGroundPromptEndsWithItsOwnSuffix pins §4.2 at the emission:
// what tp ground prints ends with the ground isolation clause and
// incrementalClause, which is the suffix appendClausesGround builds and NOT
// the one review and audit share.
//
// The assertion joins the output-path line to the suffix rather than testing
// the suffix alone, because a suffix test on its own cannot see the trailing
// newline §2.3 requires be stripped first: a body that keeps it still ends
// with the suffix, with a blank line smuggled in front of it.
func TestTheEmittedGroundPromptEndsWithItsOwnSuffix(t *testing.T) {
	prompt := groundTestPrompt()

	assert.True(t, strings.HasSuffix(prompt, "ground-r7.ndjson"+groundClauseSuffix()),
		"the body's last line names the output file and the ground suffix follows it directly")
	assert.NotContains(t, prompt, isolationClause,
		"§4.2: the review/audit clause forbids the copy the tier table requires, so a ground prompt must not carry it")
	assert.Contains(t, prompt, incrementalClause, "§3.2's clause is carried unchanged")
}

