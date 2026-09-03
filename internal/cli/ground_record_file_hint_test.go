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
	require.NotEqual(t, recordFileMissingHint, groundRecordFileHint,
		"the point of the constant is that it is not the shared one")

	for _, word := range []string{"reviewer", "auditor", "task file"} {
		assert.NotContains(t, strings.ToLower(groundRecordFileHint), word,
			"grounding has no %s: §7.1 emits one prompt (Non-Goal 4)", word)
	}
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
