package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/engine"
)

// TestReferenceCarriesTheEpicExportContract pins spec/1.5.0.md §7's read
// payloads: tp epic ready, gate and status, and the versioned export's keys.
func TestReferenceCarriesTheEpicExportContract(t *testing.T) {
	t.Parallel()
	body := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), "### Read payloads and `tp epic export` (v1.5.0)")
	for _, term := range []string{
		"`tp epic ready`", "`tp epic gate`", "`tp epic status`", "`version`", "`graph`", "`milestones`",
		"`epics`", "`slices`", "`done_at`", "`phase`", "`in_flight`", "`key`", "`--compact`", "`--md`",
	} {
		assert.Contains(t, body, term, "the payload section names %q", term)
	}
}

// epicRuleSlugs is every rule tp epic lint can report, in the order of
// spec/1.3.0.md §4's table.
var epicRuleSlugs = []string{
	engine.EpicRuleCycle, engine.EpicRuleDanglingReference, engine.EpicRuleDuplicateName,
	engine.EpicRuleUnknownKey, engine.EpicRuleMissingWhy, engine.EpicRuleUnknownMilestone,
	engine.EpicRuleLaterMilestoneDependency, engine.EpicRuleGateMemberOutsideMilestone,
	engine.EpicRuleUnprovidedTagCheck, engine.EpicRuleNoDependencies, engine.EpicRuleThinWhy,
}

// TestReferenceOpensMilestonesThroughGates pins spec/1.5.0.md §4-§6: default
// gate members, green, open, the met record, conditions and the epic block.
func TestReferenceOpensMilestonesThroughGates(t *testing.T) {
	t.Parallel()
	doc := readRepoDoc(t, "skills/tp/REFERENCE.md")
	gates := docSectionBody(t, doc, "### Gates, conditions and the epic block (v1.5.0)")
	for _, term := range []string{
		"deferrable", "**Green**", "**Open**", "`met`", "`threshold_changed: true`", "`condition: true`",
		"`blocked_by_epics`", "`tp next`", "`tp claim`", "`tp done`", "`wip`", "behaves as unbound",
	} {
		assert.Contains(t, gates, term, "the gates section names %q", term)
	}
	resume := docSectionBody(t, doc, "### `tp resume [spec]` — resume oracle (read-only)")
	assert.Contains(t, resume, "`epic-blocked` (escalate, `{epic, blocked_by_epics}`;",
		"the blocker vocabulary names epic-blocked with its class and data")
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

// TestReferenceDerivesEpicStatus pins spec/1.5.0.md §2-§3 to REFERENCE.md: a
// row per derived status, done_at, and where bound files are found.
func TestReferenceDerivesEpicStatus(t *testing.T) {
	t.Parallel()
	body := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), "### Binding and status (v1.5.0)")
	for _, term := range []string{
		"| `planned` |", "| `claimed` |", "| `specced` |", "| `in_progress` |", "| `done` |",
		"`done_at`", "`release`", "*.tasks.json", "ignored",
	} {
		assert.Contains(t, body, term, "the binding and status section names %q", term)
	}
}

// TestSkillWorkflowETracksProgress pins spec/1.5.0.md's half of Workflow E:
// binding, reading the plan, claiming, gates with thresholds, conditions, the
// epic block on the task commands, deferrable trigger work, long-lead epics
// and the three escalation decisions the operator writes are fenced under.
func TestSkillWorkflowETracksProgress(t *testing.T) {
	t.Parallel()
	body := docSectionBody(t, readRepoDoc(t, "skills/tp/SKILL.md"), "## Workflow E:")
	for _, term := range []string{
		"tp init <spec> --epic", "tp set --epic", "tp epic ready", "tp epic status", "tp epic export",
		"tp epic claim", "threshold", "tp epic gate <m> --met --evidence", "condition",
		"tp epic done <id> --evidence", "blocked_by_epics", "epic-blocked", "deferrable",
		"earliest milestone", engine.EscalateEpicDone, engine.EscalateGateMet, engine.EscalateEpicBinding,
	} {
		assert.Contains(t, body, term, "Workflow E names %q", term)
	}
}

// TestReferenceCarriesTheEpicProgressSchema pins spec/1.5.0.md §8 to the graph
// file's key table: each new epic and gate key, and the task file's `epic`.
func TestReferenceCarriesTheEpicProgressSchema(t *testing.T) {
	t.Parallel()
	schema := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), "### The graph file (`.tp/epics.json`)")
	for _, key := range []string{
		"| `epics[].condition` |", "| `epics[].estimate_minutes` |", "| `epics[].claimed` |",
		"| `epics[].done` |", "| `gates.<milestone>.threshold` |", "| `gates.<milestone>.met` |",
		"| a task file's `epic` |",
	} {
		assert.Contains(t, schema, key, "the schema table has a %s row", key)
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

// TestReferenceCoversTheEpicProgressRefusals pins spec/1.5.0.md's refusals to
// the `tp epic` exit-code table: claims, done records, met records, conditions,
// bindings and an absent --milestone each have a row naming them.
func TestReferenceCoversTheEpicProgressRefusals(t *testing.T) {
	t.Parallel()
	codes := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), "### Exit codes (`tp epic`)")
	for _, term := range []string{
		"tp epic claim", "--release", "--by", "tp epic done", "--undo", "--met", "threshold",
		"condition=true", "empty acceptance", "tp init --epic", "tp set --epic", "names `claimed`",
		"`status --milestone`", "`gate`'s milestone", "red",
	} {
		assert.Contains(t, codes, term, "the exit code table covers %q", term)
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

// TestReferenceListsEveryEscalationDecision pins the `tp escalate` section to
// the closed set itself, so a decision the enum gains (v1.5.0 added epic-done,
// gate-met and epic-binding) fails here until REFERENCE.md names it.
func TestReferenceListsEveryEscalationDecision(t *testing.T) {
	t.Parallel()
	body := docSectionBody(t, readRepoDoc(t, "skills/tp/REFERENCE.md"), "### `tp escalate` — the escalation record")
	for _, decision := range engine.EscalationDecisions() {
		assert.Contains(t, body, "`"+decision+"`", "the escalate section lists the %q decision", decision)
	}
}

// TestReadmeEpicGraphTracksProgress pins spec/1.5.0.md §1 to README.md's Epic
// Graph section: binding, derived status, gates opening milestones,
// conditions, the epic block and the export contract.
func TestReadmeEpicGraphTracksProgress(t *testing.T) {
	t.Parallel()
	body := docSectionBody(t, readRepoDoc(t, "README.md"), "## Epic Graph")
	for _, term := range []string{
		"tp init <spec> --epic <id>", "tp set --epic", "`planned`", "`in_progress`", "`release`",
		"**Gates open milestones.**", "`threshold`", "--met", "`condition: true`", "`blocked_by_epics`",
		"`epic-blocked`", "tp epic ready", "tp epic status", "versioned JSON",
	} {
		assert.Contains(t, body, term, "the Epic Graph section names %q", term)
	}
}
