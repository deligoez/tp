package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// epicRuleSlugs is every rule tp epic lint can report, in the order of
// spec/1.3.0.md §4's table.
var epicRuleSlugs = []string{
	engine.EpicRuleCycle, engine.EpicRuleDanglingReference, engine.EpicRuleDuplicateName,
	engine.EpicRuleUnknownKey, engine.EpicRuleMissingWhy, engine.EpicRuleUnknownMilestone,
	engine.EpicRuleLaterMilestoneDependency, engine.EpicRuleGateMemberOutsideMilestone,
	engine.EpicRuleUnprovidedTagCheck, engine.EpicRuleNoDependencies, engine.EpicRuleThinWhy,
}

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

// TestReferenceCarriesTheEpicGraphContract pins §7's REFERENCE.md half: the
// file schema, a row per lint rule, the state directory and the exit codes.
func TestReferenceCarriesTheEpicGraphContract(t *testing.T) {
	t.Parallel()
	doc := readRepoDoc(t, "skills/tp/REFERENCE.md")
	schema := docSectionBody(t, doc, "### The graph file (`.tp/epics.json`)")
	for _, key := range []string{`"version"`, `"milestones"`, `"gates"`, `"members"`, `"deferrable"`,
		`"tag_checks"`, `"provided_by"`, `"epics"`, `"id"`, `"title"`, `"milestone"`, `"tags"`,
		`"depends_on"`, `"why"`, `"acceptance"`} {
		assert.Contains(t, schema, key, "the schema shows the %s key", key)
	}
	rules := docSectionBody(t, doc, "### Lint rules (`tp epic lint`)")
	for _, rule := range epicRuleSlugs {
		assert.Contains(t, rules, "| `"+rule+"` |", "the lint rule table has a row for %q", rule)
	}
	state := docSectionBody(t, doc, "### State directory (`.tp/epic-review/`)")
	for _, name := range []string{"round-<N>.ndjson", "snapshot-round-<N>.json", "SHA-256"} {
		assert.Contains(t, state, name, "the state directory section names %q", name)
	}
	codes := docSectionBody(t, doc, "### Exit codes (`tp epic`)")
	for _, row := range []string{"| 0 |", "| 1 |", "| 2 |", "| 3 |", "| 4 |"} {
		assert.Contains(t, codes, row, "the exit code table has a %s row", row)
	}
}

// TestReadmeIndexListsEveryEpicSubcommand pins §7's README.md half against the
// command tree itself, so a subcommand added later without a row fails here.
func TestReadmeIndexListsEveryEpicSubcommand(t *testing.T) {
	t.Parallel()
	readme := readRepoDoc(t, "README.md")
	for _, sub := range newEpicCmd().Commands() {
		assert.Contains(t, readme, "\ntp epic "+sub.Name()+" ", "README.md's command index has a `tp epic %s` row", sub.Name())
	}
}
