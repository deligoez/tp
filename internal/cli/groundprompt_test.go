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
//
// One floor unit and nothing carried, so §8's ask is the whole of it: these
// assertions are about the suffix and the kind-tier table, and a narrowed ask
// would put the round's arithmetic in front of them for no reason.
func groundTestPrompt() string {
	return buildGroundPrompt("spec.md", ".tp-review/spec/snapshot-ground-round-7.md",
		"# commit unknown\nu1 §1 0123456789ab #1 12B\n# 1 in floor, 0 cut\n", "ground-r7.ndjson", 7, 1, 0)
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

// TestThePromptsKindTierTableIsDerivedFromTheRuleThatRejectsRows checks the
// one thing in the prompt that is normative: the tiers it tells a unit are
// acceptable for a kind are the tiers TierAcceptableFor accepts. A prompt
// stating the rule in its own words can drift from the recorder, and the unit
// pays for the drift — every row it writes under the prompt's reading is
// rejected at --record, with the whole round refused (§7.2).
//
// The check runs over every (kind, tier) pair rather than over a stated table,
// so a kind added to §4.1 cannot be silently absent from the prompt.
func TestThePromptsKindTierTableIsDerivedFromTheRuleThatRejectsRows(t *testing.T) {
	prompt := groundTestPrompt()

	for _, kind := range engine.GroundKinds() {
		line := ""
		for _, l := range strings.Split(prompt, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), string(kind)+" ") {
				line = l
				break
			}
		}
		require.NotEmpty(t, line, "the prompt must carry a row for kind %q", kind)

		// The tier list is the line's last column: the columns are padded, and
		// the list itself joins with ", " and so holds no double space.
		gap := strings.LastIndex(line, "  ")
		require.GreaterOrEqual(t, gap, 0, "row %q must be columnar", line)
		listed := make(map[string]bool)
		for _, tier := range strings.Split(strings.TrimSpace(line[gap:]), ", ") {
			listed[tier] = true
		}

		for _, tier := range engine.GroundTiers() {
			assert.Equal(t, engine.TierAcceptableFor(kind, tier), listed[string(tier)],
				"the prompt and TierAcceptableFor must agree on {%s, %s}", kind, tier)
		}
	}
}
