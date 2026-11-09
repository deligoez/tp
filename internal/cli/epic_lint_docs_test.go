package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// TestEpicLintRulesAreInTheReadmeRuleTable covers what the rule census cannot
// see: the epic lint's rules reach EpicLintFinding through constants, not as
// engine.Finding literals, so each is listed here and must have a README row
// emitted by `tp epic lint` (spec/1.3.0.md §7).
func TestEpicLintRulesAreInTheReadmeRuleTable(t *testing.T) {
	t.Parallel()
	rows := map[string]bool{}
	for _, line := range strings.Split(readRepoDoc(t, "README.md"), "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) >= 2 && strings.Contains(cells[1], "`tp epic lint`") {
			rows[strings.Trim(strings.TrimSpace(cells[0]), "`")] = true
		}
	}
	for _, rule := range []string{
		engine.EpicRuleCycle, engine.EpicRuleDanglingReference, engine.EpicRuleDuplicateName,
		engine.EpicRuleUnknownKey, engine.EpicRuleMissingWhy, engine.EpicRuleUnknownMilestone,
		engine.EpicRuleLaterMilestoneDependency, engine.EpicRuleGateMemberOutsideMilestone, engine.EpicRuleGateMemberDeferrable,
		engine.EpicRuleUnprovidedTagCheck, engine.EpicRuleNoDependencies, engine.EpicRuleThinWhy,
		engine.EpicRuleRootWithDependencies, engine.EpicRuleOpensAfterCycle, engine.EpicRulePartialParallel,
	} {
		assert.True(t, rows[rule], "README.md's rule table has no `tp epic lint` row for %q", rule)
	}
}
