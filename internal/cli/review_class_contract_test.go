package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReviewPrompt_ClassIsAskedOneWay: every review role prompt asked for
// `class` twice, once as optional ("omit it otherwise") beside the JSON
// example and once in the output contract as a stamp on EVERY finding.
// The contract is the one recorded rows follow, so the optional sentence goes.
func TestReviewPrompt_ClassIsAskedOneWay(t *testing.T) {
	t.Parallel()
	dir := configCheckFixture(t, true)
	texts, _ := panelPrompts(t, dir)
	for i, text := range texts {
		assert.False(t, strings.Contains(text, "omit it otherwise"),
			"prompt %d still calls class optional", i)
		assert.Contains(t, text, "- class: a kebab-case slug", "prompt %d lost the contract's class line", i)
	}
}
