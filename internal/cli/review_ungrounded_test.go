package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// groundedFixture emits a ground round on the shared fixture spec and, when
// dispositioned > 0, records a round grading that many of its EMITTED floor
// units.
//
// The floor is read back from the file the emission wrote rather than derived
// again, so the ids, anchors, hashes and ordinals in the record are the ones tp
// itself emitted. The two requires are what make the tests built on it say
// anything: the fixture's floor must be exactly two units, so grading one
// leaves exactly one open, and the index must carry a cut row, or `floor_size`
// and the index's length agree and the difference between them is untested.
// groundedFixture writes the shared fixture spec and grounds it.
func groundedFixture(t *testing.T, dispositioned int) string {
	t.Helper()
	dir := writeGroundFixture(t)
	groundFixtureRound(t, dir, dispositioned)
	return dir
}

// groundFixtureRound emits a ground round on dir's spec and, when
// dispositioned > 0, records a round grading that many of its EMITTED floor
// units.
//
// The floor is read back from the file the emission wrote rather than derived
// again, so the ids, anchors, hashes and ordinals in the record are the ones tp
// itself emitted. The two requires are what make the tests built on it say
// anything: the fixture's floor must be exactly two units, so grading one
// leaves exactly one open, and the index must carry a cut row, or `floor_size`
// and the index's length agree and the difference between them is untested.
func groundFixtureRound(t *testing.T, dir string, dispositioned int) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "ground", "spec.md")
	require.Equal(t, 0, code, "ground: %s", stderr)

	index, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "floor-ground-round-1.txt"))
	require.NoError(t, err)
	rows, err := engine.ParseFloorIndex(string(index))
	require.NoError(t, err)

	emitted := make([]engine.FloorIndexRow, 0, len(rows))
	for _, r := range rows {
		if r.TextSHA != "" {
			emitted = append(emitted, r)
		}
	}
	require.Len(t, emitted, 2, "the fixture's floor must be two units")
	require.Less(t, len(emitted), len(rows), "and the index must carry a cut row it does not count")
	require.LessOrEqual(t, dispositioned, len(emitted))

	if dispositioned == 0 {
		return
	}
	var b strings.Builder
	for _, r := range emitted[:dispositioned] {
		fmt.Fprintf(&b, `{"unit_id":%q,"anchor":%q,"text_sha":%q,"ordinal":%d,"verdict":"NOT-A-CLAIM"}`+"\n",
			r.ID, r.Anchor, r.TextSHA, r.Ordinal)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ground-r1.ndjson"), []byte(b.String()), 0o600))
	_, stderr, code = runTP(t, dir, "ground", "spec.md", "--record", "ground-r1.ndjson")
	require.Equal(t, 0, code, "record: %s", stderr)
}

// reviewEnvelope runs a review and returns its decoded top-level payload with
// the raw text beside it, because §11 row 17's second half is a claim about how
// MANY times the key appears and a decoded map cannot answer that.
func reviewEnvelope(t *testing.T, dir string, args ...string) (map[string]any, string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"review", "spec.md"}, args...)...)
	require.Equal(t, 0, code, "review: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out, stdout
}

// TestReviewCarriesTheUngroundedAdvisoryOnceInTheEnvelope is §11 row 17's second
// half: the advisory appears exactly ONCE, in the top-level envelope, on a run
// emitting more than one role prompt.
//
// The prompt count is a require rather than an assumption — with one prompt the
// row's mutant, emitting the advisory per role, is indistinguishable from the
// shipped behaviour — and the count of the key in the RAW payload is what
// separates them: a per-role emission puts one copy in every prompt.
func TestReviewCarriesTheUngroundedAdvisoryOnceInTheEnvelope(t *testing.T) {
	out, raw := reviewEnvelope(t, groundedFixture(t, 1))

	prompts, ok := out["prompts"].([]any)
	require.True(t, ok, "prompts: %T", out["prompts"])
	require.Greater(t, len(prompts), 1, "row 17 asks for a run emitting more than one role prompt")

	assert.Equal(t, map[string]any{
		"round": float64(1), "undispositioned": float64(1), "floor_size": float64(2),
	}, out["ungrounded"], "the round, the units without a disposition, and the floor's size")
	assert.Equal(t, 1, strings.Count(raw, `"ungrounded"`),
		"once in the envelope, never one copy per role — the reader is the operator, not the role")
}

// TestReviewsExitCodeIsIdenticalWithAndWithoutUngroundedUnits is §11 row 17's
// first half, and Non-Goal 3: review is told, and not stopped.
//
// The two runs are asserted to actually DIFFER in the advisory before their exit
// codes are compared. Without that, the equality is a statement about two runs
// that were the same, which is the shape this claim is easiest to fake.
func TestReviewsExitCodeIsIdenticalWithAndWithoutUngroundedUnits(t *testing.T) {
	withUnits, withRaw := reviewEnvelope(t, groundedFixture(t, 1))
	withoutUnits, withoutRaw := reviewEnvelope(t, groundedFixture(t, 2))

	require.Contains(t, withUnits, "ungrounded", "the open fixture must have something to say")
	require.NotContains(t, withoutUnits, "ungrounded", "and the grounded one must not")

	_, _, openCode := runTP(t, groundedFixture(t, 1), "review", "spec.md")
	_, _, closedCode := runTP(t, groundedFixture(t, 2), "review", "spec.md")
	assert.Equal(t, closedCode, openCode, "the exit code is identical with and without ungrounded units")
	assert.Equal(t, 0, openCode)
	assert.NotEqual(t, withRaw, withoutRaw, "the two payloads differ, so the exit codes are not trivially equal")
}

// TestTheUngroundedAdvisorySurvivesCompact: §9 puts the advisory on
// `divergence`'s footing — one key in the envelope, absent rather than
// zero-valued when there is nothing to say, surviving --compact. What --compact
// drops is explanatory; this is the payload.
func TestTheUngroundedAdvisorySurvivesCompact(t *testing.T) {
	out, _ := reviewEnvelope(t, groundedFixture(t, 1), "--compact")

	assert.Equal(t, map[string]any{
		"round": float64(1), "undispositioned": float64(1), "floor_size": float64(2),
	}, out["ungrounded"], "--compact does not drop it")
}

// TestReviewOnASpecWithNoGroundRoundCarriesNoUngroundedKey: absent ENTIRELY,
// not present and zeroed. A key that is always there is a key every reader
// learns to skip, which is the economy §9 inherits from `divergence`.
func TestReviewOnASpecWithNoGroundRoundCarriesNoUngroundedKey(t *testing.T) {
	out, raw := reviewEnvelope(t, writeGroundFixture(t))

	assert.NotContains(t, out, "ungrounded", "no ground round, so nothing to say")
	assert.NotContains(t, raw, "ungrounded")
}
