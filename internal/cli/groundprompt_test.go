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

// TestThePromptsPartialKindListNamesEveryValueTheRecorderAccepts is the
// kind-tier test's rule applied to §7.2's third enum: the values the prompt
// tells a unit to choose between are exactly the values ParseGroundPartialKind
// accepts. A value the recorder takes and the prompt never names is a value no
// unit will ever write, and a value the prompt names and the recorder rejects
// refuses the whole round (§7.2).
//
// The comparison is between SETS, not a Contains per accepted value. A
// Contains loop is sound in one direction only, and it is the direction this
// test's own doc comment does not claim: an extra value in the prompt leaves
// every Contains satisfied, so the mutant that names a kind the recorder
// rejects — the one that costs a unit its whole round — passed the loop this
// replaces. Its sibling below asserts set equality for the same reason.
//
// The assertion is over the sentence bounded by the partial_kind marker and the
// held_at line that follows it, not over the whole prompt, so a value that
// happens to appear elsewhere cannot satisfy it.
func TestThePromptsPartialKindListNamesEveryValueTheRecorderAccepts(t *testing.T) {
	prompt := groundTestPrompt()

	const marker = "partial_kind — required on PARTIAL:"
	start := strings.Index(prompt, marker)
	require.GreaterOrEqual(t, start, 0, "the prompt must carry the partial_kind line")
	sentence := prompt[start+len(marker):]
	end := strings.Index(sentence, "\nheld_at")
	require.GreaterOrEqual(t, end, 0, "the partial_kind sentence ends where held_at's line begins")
	sentence = sentence[:end]

	const lead = "exactly one of "
	listAt := strings.Index(sentence, lead)
	require.GreaterOrEqual(t, listAt, 0, "the sentence must offer the choice before listing it: %q", sentence)
	listed := strings.Split(strings.TrimSuffix(strings.TrimSpace(sentence[listAt+len(lead):]), "."), ", ")

	accepted := make([]string, 0, len(engine.GroundPartialKinds()))
	for _, kind := range engine.GroundPartialKinds() {
		accepted = append(accepted, string(kind))
	}

	assert.ElementsMatch(t, accepted, listed,
		"the prompt's partial_kind list and the values ParseGroundPartialKind accepts are the same set")
}

// TestThePromptsKindTierTableIsDerivedFromTheRuleThatRejectsRows checks the
// one thing in the prompt that is normative: the tiers it tells a unit are
// acceptable for a kind are the tiers TierAcceptableFor accepts. A prompt
// stating the rule in its own words can drift from the recorder, and the unit
// pays for the drift — every row it writes under the prompt's reading is
// rejected at --record, with the whole round refused (§7.2).
//
// The comparison is between SETS, one per kind. The form this replaces asked
// TierAcceptableFor about every (kind, tier) pair and looked each tier up in
// the row, which is blind in the direction that costs a unit its round: a name
// in the row that is not a tier at all is never asked about. Measured —
// appending "stale-tier" to every row's list left that form green, so every
// kind advertised a tier the recorder rejects and no assertion moved.
//
// It still runs over every kind rather than over a stated table, so a kind
// added to §4.1 cannot be silently absent from the prompt either.
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
		listed := make([]string, 0, len(engine.GroundTiers()))
		if rest := strings.TrimSpace(line[gap:]); rest != "" {
			listed = strings.Split(rest, ", ")
		}

		acceptable := make([]string, 0, len(engine.GroundTiers()))
		for _, tier := range engine.GroundTiers() {
			if engine.TierAcceptableFor(kind, tier) {
				acceptable = append(acceptable, string(tier))
			}
		}

		assert.ElementsMatch(t, acceptable, listed,
			"the prompt's tier list for %q and the tiers TierAcceptableFor accepts must be the same set", kind)
	}
}
