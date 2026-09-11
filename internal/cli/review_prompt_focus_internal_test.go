package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSpecOnlyDisclaimerAsksOnlyForWhatMakesTheImplementationWrong: the
// disclaimer rides on every review prompt, beside role text that says to
// report only what would make the implementation wrong. It used to ask for
// "completeness, ambiguity, contradictions, missing edge cases, testability",
// an invitation to report detail the code settles. The constant is the whole
// artifact, so an absence assertion over it is sound.
func TestSpecOnlyDisclaimerAsksOnlyForWhatMakesTheImplementationWrong(t *testing.T) {
	t.Parallel()
	assert.Contains(t, specOnlyDisclaimer,
		"Focus on: requirements that contradict each other or the stated goal, cannot be satisfied, "+
			"or read two ways that two correct implementations would disagree on.")
	for _, invitation := range []string{"completeness", "missing edge cases", "testability"} {
		assert.NotContains(t, specOnlyDisclaimer, invitation)
	}
}

// TestOverSpecificationNeverBlocksARoundByItself: the one class whose fix is a
// deletion must not hold a round open, and nothing but the prompt caps its
// severity, so the prompt says so.
func TestOverSpecificationNeverBlocksARoundByItself(t *testing.T) {
	t.Parallel()
	assert.Contains(t, overSpecificationInstruction, "Canonical class `over-specification`")
	assert.Contains(t, overSpecificationInstruction, "Record it at most medium")
}
