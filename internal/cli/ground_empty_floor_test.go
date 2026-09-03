package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// groundAllCutSpec carries four real prose claims and NOT ONE floor unit: no
// sentence holds a digit, a backtick span or a verb from §2.1's list, so all
// three arms cut everything.
//
// It is a document about which grounding has nothing to say and plenty to check,
// which is exactly the state `--check` used to certify.
const groundAllCutSpec = `# Fixture

## 1. Claims

The tool always refuses a request whose owner cannot be named.

Every unit of the corpus belongs to exactly one owner, and no owner is shared.

The reader is told what the writer meant, never what the writer wrote.

## 2. More

A caller who asks twice is answered once.
`

// groundNoUnitsSpec is the OTHER floor of size zero: headings and nothing else,
// so §2.1 produces no unit for the arms to cut.
//
// It is the input on which 0-of-0 is honestly covered, and the reason this test
// has two arms rather than one.
const groundNoUnitsSpec = `# Fixture

## 1. Nothing
`

func writeGroundSpec(t *testing.T, text string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(text), 0o600))
	return dir
}

// groundFloorSummary returns the emitted floor's own two counts, read back off
// the artifact rather than re-derived: how many units the round owes a
// disposition for, and how many the arms cut.
func groundFloorSummary(t *testing.T, dir string) (inFloor, cut int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "floor-ground-round-1.txt"))
	require.NoError(t, err)
	rows, err := engine.ParseFloorIndex(string(data))
	require.NoError(t, err)
	for _, r := range rows {
		if r.TextSHA == "" {
			cut++
			continue
		}
		inFloor++
	}
	return inFloor, cut
}

// TestCheckRefusesAFloorTheArmsEmptiedAndAcceptsADocumentWithNoUnits is the one
// state in which `--check`'s exit 0 said nothing at all.
//
// **The two arms are the whole finding, and the second is why this is not a
// blanket rule about zero.** `--check` gates on §8's coverage — dispositioned
// against emitted — and on a floor of no units that comparison is 0 < 0, which
// is false, so the gate that certifies a release exits 0 for a spec nobody
// checked. Measured on the first fixture below before the repair: round 1 emits,
// nothing is dispositioned, and `--status --check` exits 0.
//
// On the SECOND fixture 0-of-0 is honest. §2.1 produced no unit, so there is
// nothing the round failed to look at, and refusing there would refuse every
// spec that is all headings. The floor index tells the two apart on its own
// terms — `0 in floor, 4 cut` against `0 in floor, 0 cut` — which is why the
// rule is stated on the CUT count and not on the emitted one, and why both
// counts are read back off the emitted artifact here rather than assumed.
//
// A blanket "exit 1 whenever emitted is 0" passes the first arm and reddens the
// second; the shipped comparison alone passes the second and reddens the first.
// Neither arm alone decides it.
func TestCheckRefusesAFloorTheArmsEmptiedAndAcceptsADocumentWithNoUnits(t *testing.T) {
	t.Run("every unit cut", func(t *testing.T) {
		dir := writeGroundSpec(t, groundAllCutSpec)
		groundEmit(t, dir)

		inFloor, cut := groundFloorSummary(t, dir)
		require.Equal(t, 0, inFloor, "the fixture must leave no unit owing a disposition")
		require.Positive(t, cut, "and must have had units for the arms to cut, or it is the other fixture")

		payload, code := groundStatusCheck(t, dir)
		require.Equal(t, float64(0), payload["emitted"])
		require.Equal(t, float64(0), payload["dispositioned"])
		assert.Equal(t, 1, code,
			"a floor the arms emptied is not a spec that was checked: nothing was, and units existed")
	})

	t.Run("no unit to cut", func(t *testing.T) {
		dir := writeGroundSpec(t, groundNoUnitsSpec)
		groundEmit(t, dir)

		inFloor, cut := groundFloorSummary(t, dir)
		require.Equal(t, 0, inFloor)
		require.Equal(t, 0, cut, "the fixture must produce no unit at all, or it is the other fixture")

		payload, code := groundStatusCheck(t, dir)
		require.Equal(t, float64(0), payload["emitted"])
		assert.Equal(t, 0, code,
			"0 of 0 is honestly covered when §2.1 produced nothing: there is no claim the round skipped")
	})
}

// TestTheEmptiedFloorRefusalSaysWhyOnStderr keeps the exit code from being the
// only thing an operator gets.
//
// `--check` prints no error — §7.1 makes it a read-back on --status's payload,
// and Non-Goal 3 keeps it from being a refusal — so the payload it prints is
// `emitted: 0, dispositioned: 0`, which reads as complete. The notice is what
// names the cut count the payload has nowhere to carry. It goes to
// output.Notice's channel rather than into the envelope, because the envelope's
// keys are documented surface and this is a remark about one invocation.
//
// The exit code is asserted beside it, so this cannot pass on a build that
// prints the notice and still exits 0.
func TestTheEmptiedFloorRefusalSaysWhyOnStderr(t *testing.T) {
	dir := writeGroundSpec(t, groundAllCutSpec)
	groundEmit(t, dir)

	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--status", "--check")
	require.Equal(t, 1, code)
	assert.Contains(t, strings.ToLower(stderr), "cut",
		"the notice names what the payload cannot: units existed and the arms dropped every one")
}
