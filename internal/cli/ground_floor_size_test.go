package cli_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// TestFloorSizeIsTheUnitsListingsLineCount is §2's observable for `floor_size`:
// the number of lines `tp ground <spec> --units` prints. The command is RUN and
// its stdout counted, rather than the listing being re-derived in the test —
// re-deriving it would assert that two calls of one function agree, and the
// observable §2 names is the command's output.
//
// The fixture is the `--units` fixture, and two of its properties are what make
// this discriminating; both are required rather than assumed.
//
// It carries units the arms cut, so the candidate-inclusive population is
// strictly larger than the listing. Without that, `FloorUnits` and the uncut
// floor return the same number and the NotEqual below passes on a fixture that
// could not tell them apart — which is the shape §2 records two exported
// functions being wrong in: `engine.FloorUnits` returns the candidates, cuts
// included, and `engine.FloorIndexRows` gives every candidate a row
// unconditionally, so their difference is identically zero.
//
// It carries no frontmatter. `--units` reads the spec's bytes as they are, so a
// frontmatter block would become candidate units of its own and put a second
// reason under any disagreement this test finds.
func TestFloorSizeIsTheUnitsListingsLineCount(t *testing.T) {
	t.Parallel()
	dir := writeGroundUnitsFixture(t)

	require.False(t, strings.HasPrefix(groundUnitsFixtureSpec, "---"),
		"the fixture must carry no frontmatter, or a second population is in play")

	rows := engine.FloorIndexRows(groundUnitsFixtureSpec, engine.FloorAnchorOf(groundUnitsFixtureSpec))
	candidates := len(engine.FloorUnits(groundUnitsFixtureSpec))
	require.Equal(t, candidates, len(rows),
		"FloorIndexRows emits one row per candidate unit, so their difference is no cut count")

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--units")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.NotEmpty(t, stdout, "an empty listing would make the line count below a fiction")
	printed := len(strings.Split(strings.TrimSuffix(stdout, "\n"), "\n"))

	require.Greater(t, candidates, printed,
		"the fixture must carry units the arms cut, or the two populations agree and nothing here discriminates")

	assert.Equal(t, printed, engine.FloorSize(rows),
		"`floor_size` is the number of lines `tp ground <spec> --units` prints")
	assert.NotEqual(t, printed, candidates,
		"and the candidate-inclusive population is not that number")
	assert.Equal(t, candidates-printed, len(rows)-engine.FloorSize(rows),
		"`cut` is the index's row count less `floor_size`")
}
