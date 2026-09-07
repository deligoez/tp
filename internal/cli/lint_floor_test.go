package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// lintFloorFixture runs `tp ground <spec> --units` and `tp lint <spec>` over the
// one fixture §6 row 1 prescribes and returns the three quantities a row can be
// decided on: the listing's line count, and lint's own `floor_size` and `cut`.
//
// The two commands are RUN rather than re-derived. Re-deriving either side would
// assert that two calls of one function agree, while row 1's requirement is
// about what the two commands print.
//
// The fixture is `groundUnitsFixtureSpec`, shared with the `--units` tests, and
// BOTH halves of §6 row 1's precondition are asserted here rather than trusted:
//
//   - it carries units the arms cut, so the candidate-inclusive population is
//     strictly larger than the listing. Without that the two populations
//     coincide and the mutant §6 row 1 names — count the floor index's rows —
//     passes on a fixture that cannot tell them apart;
//   - it carries no frontmatter, so `tp lint` and `tp ground --units` see the
//     same text: lint reads the text parseSpecFile blanked the frontmatter out
//     of, `--units` reads the spec's bytes as they are. Whether that difference
//     shows depends on the block's CONTENT, so a frontmatter fixture would be a
//     coin toss rather than a control — measured on two six-line blocks over
//     this same spec, one took the listing from 2 units to 3 while the other
//     changed nothing. Either way it would put a second reason under any
//     disagreement this test finds.
func lintFloorFixture(t *testing.T) (printed, candidates, floorSize, cut int) {
	t.Helper()
	dir := writeGroundUnitsFixture(t)

	require.False(t, strings.HasPrefix(groundUnitsFixtureSpec, "---"),
		"the fixture must carry no frontmatter, or lint and --units do not read the same text")

	unitsOut, stderr, code := runTP(t, dir, "ground", "spec.md", "--units")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.NotEmpty(t, unitsOut, "an empty listing would make the line count below a fiction")
	printed = len(strings.Split(strings.TrimSuffix(unitsOut, "\n"), "\n"))

	rows := engine.FloorIndexRows(groundUnitsFixtureSpec, engine.FloorAnchorOf(groundUnitsFixtureSpec))
	candidates = len(rows)
	require.Greater(t, candidates, printed,
		"the fixture must carry at least one cut unit, or the two populations agree and nothing here discriminates")

	lintOut, stderr, code := runTP(t, dir, "lint", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(lintOut), &payload))

	raw, ok := payload["floor_size"]
	require.True(t, ok, "tp lint reports floor_size: %s", lintOut)
	size, ok := raw.(float64)
	require.True(t, ok, "floor_size is a number, not %T", raw)

	raw, ok = payload["cut"]
	require.True(t, ok, "tp lint reports cut: %s", lintOut)
	cutValue, ok := raw.(float64)
	require.True(t, ok, "cut is a number, not %T", raw)

	return printed, candidates, int(size), int(cutValue)
}

// TestLintFloorSizeIsTheUnitsListingsLineCount is §6 row 1: while a spec carries
// at least one cut unit, `tp lint`'s `floor_size` equals the number of lines
// `tp ground <spec> --units` prints.
//
// The named mutant is "count the floor index's rows in lint" — the
// cut-inclusive population — and the closing NotEqual is what makes it fail
// here rather than only the first assertion, so the row is decided twice over.
func TestLintFloorSizeIsTheUnitsListingsLineCount(t *testing.T) {
	t.Parallel()
	printed, candidates, floorSize, _ := lintFloorFixture(t)

	assert.Equal(t, printed, floorSize,
		"`floor_size` is the number of lines `tp ground <spec> --units` prints")
	assert.NotEqual(t, candidates, floorSize,
		"and it is not the floor index's row count, which counts the units the arms cut too")
}

// TestLintCutIsTheFloorIndexRowCountLessFloorSize is §2's second field: `cut` is
// the floor index's row count less `floor_size`.
//
// The expected value is built from the two observables — the index's rows, and
// the `--units` listing's line count — and never from lint's own `floor_size`.
// That is the whole of what makes this discriminating: `len(rows) - floor_size`
// compared against lint's own answer is 0 == 0 under the row 1 mutant, which
// collapses `cut` to zero while satisfying its own arithmetic. The Greater below
// pins that the fixture's expected `cut` is not zero to begin with.
func TestLintCutIsTheFloorIndexRowCountLessFloorSize(t *testing.T) {
	t.Parallel()
	printed, candidates, _, cut := lintFloorFixture(t)

	require.Greater(t, candidates-printed, 0,
		"the fixture's cut count must be non-zero, or a `cut` of 0 passes for the wrong reason")
	assert.Equal(t, candidates-printed, cut,
		"`cut` is the floor index's row count less `floor_size`")
}

// TestLintPayloadIsByteIdenticalUnderCompactAndQuiet holds §2's sentence that
// `tp lint` varies its payload by neither `--compact` nor `--quiet`, which is
// what makes the two new fields "emitted on every invocation" rather than
// emitted in one form of the output.
//
// The comparison is over BYTES of stdout, not over a decoded object: a field
// dropped, reordered or re-indented under a flag is a different payload to the
// agent reading it, and an object comparison cannot see two of those three.
//
// The first assertion is what keeps the other two from comparing three copies
// of nothing — a lint that failed under every form would satisfy byte-identity
// perfectly.
func TestLintPayloadIsByteIdenticalUnderCompactAndQuiet(t *testing.T) {
	t.Parallel()
	dir := writeGroundUnitsFixture(t)

	forms := []struct {
		name string
		args []string
	}{
		{"plain", []string{"lint", "spec.md"}},
		{"compact", []string{"lint", "spec.md", "--compact"}},
		{"quiet", []string{"lint", "spec.md", "--quiet"}},
	}

	var plain string
	for _, form := range forms {
		stdout, stderr, code := runTP(t, dir, form.args...)
		require.Equal(t, 0, code, "%s: stderr: %s", form.name, stderr)
		require.Contains(t, stdout, `"floor_size"`,
			"%s: the payload under test must be the one carrying the new fields", form.name)
		if form.name == "plain" {
			plain = stdout
			continue
		}
		assert.Equal(t, plain, stdout, "%s: tp lint's payload varies by neither --compact nor --quiet", form.name)
	}
}
