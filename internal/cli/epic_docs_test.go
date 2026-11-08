package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSkillCarriesWorkflowE pins spec/1.3.0.md §7's Workflow E: its section
// names the five steps of the graph loop, in order.
func TestSkillCarriesWorkflowE(t *testing.T) {
	t.Parallel()
	body := docSectionBody(t, readRepoDoc(t, "skills/tp/SKILL.md"), "## Workflow E:")
	last := -1
	for _, step := range []string{
		"Interview the topic", "tp epic set milestones=", "tp epic add --bulk",
		"tp epic lint", "tp epic review", "--status --check", "next topic",
	} {
		at := strings.Index(body, step)
		if assert.GreaterOrEqual(t, at, 0, "Workflow E names %q", step) {
			assert.Greater(t, at, last, "Workflow E names %q after the step before it", step)
			last = at
		}
	}
}
