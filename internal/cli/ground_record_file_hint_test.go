package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTheGroundRecordFileHintNamesNoPanel is the third of --record's hints
// carved out of the shared constant, for the reason the other two already were:
// grounding has no reviewers and no auditors.
//
// §7.1 emits ONE prompt and Non-Goal 4 says why there is no panel to fan out
// over, so a hint telling an operator to look for "the NDJSON results file the
// reviewers/auditors wrote" names two roles that never ran and a task file that
// has nothing to do with this mode. groundRecordRowHint and
// groundRecordEmptyHint were split off for exactly this; the file hint was left
// on the shared constant.
//
// **The assertion's subject is the whole of a bounded artifact**, which is what
// makes a NotContains sound here: a const string has no elsewhere for a
// negation to go and restate itself in. The same assertion over the emitted
// envelope, or over any unbounded text, would be a presence check against a
// three-item blacklist and would prove nothing.
func TestTheGroundRecordFileHintNamesNoPanel(t *testing.T) {
	t.Parallel()
	require.NotEqual(t, recordFileMissingHint, groundRecordFileHint,
		"the point of the constant is that it is not the shared one")

	for _, word := range []string{"reviewer", "auditor", "task file"} {
		assert.NotContains(t, strings.ToLower(groundRecordFileHint), word,
			"grounding has no %s: §7.1 emits one prompt (Non-Goal 4)", word)
	}
}

// TestTheEmptyRecordHintCoversTheFloorItCannotRepair holds the empty-record
// hint to both states that reach it, not only the one it was written for.
//
// The refusal fires when the payload and the carry are both zero, and that has
// two causes with opposite recoveries. A reader who dispositioned nothing
// should go and disposition something — the sentence already said so. A reader
// on an all-cut document was told the same thing, and for them "ground the
// units the emitted prompt asks for" names a set with no members: §2.1 cut
// every unit, so there is nothing to ground and no round to record. Three
// commands reach it, and `--status --check` reports that floor permanently.
//
// **The subject is the whole of a bounded artifact** — one const string — which
// is what makes an assertion over its text sound here, and it is the same
// ground the NotContains above stands on. The same words checked inside the
// emitted prompt would be a presence test in an unbounded document.
func TestTheEmptyRecordHintCoversTheFloorItCannotRepair(t *testing.T) {
	t.Parallel()
	assert.Contains(t, groundRecordEmptyHint, "cut",
		"the all-cut floor reaches this refusal, and dispositioning something is not its recovery")
	assert.Contains(t, groundRecordEmptyHint, "--status --check",
		"and the hint names where that floor is reported instead")
	assert.NotContains(t, groundRecordEmptyHint, "ground the units the emitted prompt asks for",
		"which is not an instruction when the prompt asked for none")
}

// TestEveryGroundRecordHintIsItsOwn keeps the three apart as a set rather than
// one at a time.
//
// --record has three refusals an operator can act on — a file tp cannot open, a
// row that fails §7.2's table, and a payload that would record nothing — and
// each names a different recovery: check the path, fix the row, disposition
// something. A repair that gave the file case its own words by pointing it at
// one of the other two would satisfy the test above and still send the operator
// to the wrong artifact.
func TestEveryGroundRecordHintIsItsOwn(t *testing.T) {
	t.Parallel()
	hints := map[string]string{
		"file":  groundRecordFileHint,
		"row":   groundRecordRowHint,
		"empty": groundRecordEmptyHint,
	}
	seen := make(map[string]string, len(hints))
	for name, hint := range hints {
		require.NotEmpty(t, hint, "%s: every non-zero envelope carries a hint", name)
		other, clash := seen[hint]
		require.False(t, clash, "%s and %s answer different refusals with one sentence", name, other)
		seen[hint] = name
	}
}
