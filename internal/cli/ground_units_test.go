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

// groundUnitsFixtureSpec is the spec `--units` is asserted over, and every part
// of it is load-bearing.
//
// Its first prose sentence is deliberately longer than sixty bytes. §11 row 4b's
// named mutant is "print a truncation", and every unit of the shipped emit
// fixture is shorter than a sixty-byte head — so on that fixture the head IS the
// whole unit and the mutant is invisible. The test requires the length rather
// than trusting this sentence.
//
// Two units the arms cut sit BETWEEN the two floor units — the prose line
// carrying no digit, no backtick and no listed verb, and the table's header row
// of bare labels — which is the only arrangement in which numbering over every
// unit and numbering over the floor alone disagree.
//
// The second floor unit is a table data row, so one printed line carries an em
// dash and is longer in bytes than in runes; the index's own byte length is what
// the test compares its length against.
const groundUnitsFixtureSpec = `# Units fixture

## 1. Prose

The gate runs 4 steps, and this sentence is long on purpose so that a sixty-byte head of it is not the whole of it.

Prose with no signal at all.

## 2. Table

| subject | how it was checked |
|---|---|
| the floor | the derivation ran over 3 documents |
`

// writeGroundUnitsFixture puts the --units fixture in a fresh directory and
// returns it, so a test names the spec by the relative path tp is run with.
func writeGroundUnitsFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(groundUnitsFixtureSpec), 0o600))
	return dir
}

// TestGroundUnitsPrintsEveryFloorUnitWholeWithTheIndexsHash is §11 row 4b: one
// line per floor unit, whose text is the WHOLE canonical unit and whose
// `text_sha` is the sha256 of what follows it on the line.
//
// The index it is checked against is the emitted artifact READ BACK FROM DISK,
// not one re-derived inside the test. That is the claim `--units` exists to
// make: a reader holding the prompt's index — which carries no text — can join
// this listing to it by `unit_id`, and the hash beside each id is the same hash.
// Comparing against an index the test derived would assert that two calls of one
// function agree.
//
// The length assertion is the truncation mutant's own: the index says how many
// UTF-8 bytes each unit is, so a line carrying a head of the unit disagrees with
// its own index row without the test ever stating a literal.
func TestGroundUnitsPrintsEveryFloorUnitWholeWithTheIndexsHash(t *testing.T) {
	dir := writeGroundUnitsFixture(t)
	groundEmit(t, dir)

	floorData, err := os.ReadFile(groundStatePath(dir, "floor-ground-round-1.txt"))
	require.NoError(t, err)
	index, err := engine.ParseFloorIndex(string(floorData))
	require.NoError(t, err)

	byID := map[string]engine.FloorIndexRow{}
	wantIDs := make([]string, 0, len(index))
	cut := 0
	for _, row := range index {
		byID[row.ID] = row
		if row.TextSHA == "" {
			cut++
			continue
		}
		wantIDs = append(wantIDs, row.ID)
	}
	// Facts of the fixture, required rather than assumed: without a cut unit
	// between the floor units, numbering over every unit and numbering over the
	// floor alone produce the same ids and the assertion below decides nothing.
	require.Equal(t, 2, cut, "the fixture must hold units the arms cut")
	require.Equal(t, []string{"u1", "u4"}, wantIDs,
		"the fixture's floor units must be the first and the last of four, with the cut ones between them")

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--units")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	require.Len(t, lines, len(wantIDs), "one line per FLOOR unit and none for a unit the arms cut: %q", stdout)

	longest := 0
	for i, line := range lines {
		fields := strings.SplitN(line, "\t", 3)
		require.Len(t, fields, 3, "a line is <unit_id>\\t<text_sha>\\t<text>: %q", line)
		id, sha, text := fields[0], fields[1], fields[2]

		assert.Equal(t, wantIDs[i], id, "the ids are the index's floor rows, in emission order")
		row := byID[id]
		assert.Equal(t, row.TextSHA, sha, "the hash on the line is the one the index row for %s carries", id)
		assert.Equal(t, row.Bytes, len(text),
			"the line carries the WHOLE unit: the index row for %s says %d bytes", id, row.Bytes)
		assert.Equal(t, sha, engine.FloorTextSHA(text), "the text_sha on the line is the sha256 of what follows it")
		longest = max(longest, len(text))
	}
	require.Greater(t, longest, 60,
		"the fixture must hold a unit longer than a sixty-byte head, or a truncation cannot be seen here")
}

// TestGroundUnitsReadsTheSpecAndWritesNothing pins the two halves §7.1's table
// leaves implicit: `--units` answers from the spec named on the command line —
// so it works before any round has been emitted — and it is a report, so it
// writes nothing.
//
// The state directory is asserted absent AFTER the run as well as before it: an
// implementation routed through the emission would answer this listing
// correctly and leave a round behind, and a pair of assertions on stdout alone
// cannot see that.
func TestGroundUnitsReadsTheSpecAndWritesNothing(t *testing.T) {
	dir := writeGroundUnitsFixture(t)
	_, err := os.Stat(filepath.Join(dir, ".tp-review"))
	require.True(t, os.IsNotExist(err), "the fixture must start with no state directory")

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--units")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, engine.FormatFloorUnits(engine.FloorUnitRows(groundUnitsFixtureSpec)), stdout,
		"--units prints the engine's units listing over the spec's own text, with no emission behind it")

	_, err = os.Stat(filepath.Join(dir, ".tp-review"))
	assert.True(t, os.IsNotExist(err), "--units reports; it does not emit a round")
}

// TestGroundUnitsIsAModeOfItsOwn holds §7.1's table to its shape: each
// invocation names one mode and no row pairs two. Resolving a pair by the
// dispatch's order hands the operator an exit 0 for the mode they did not ask
// for.
//
// Neither subtest emits first, and the --record path names a file that does not
// exist. Both are what make the assertion discriminating: an implementation
// refusing the pair AFTER opening the record file exits 3, and one running
// --status on an unemitted spec exits 3 too, so only a refusal taken before
// either mode runs produces the 2 asserted here.
func TestGroundUnitsIsAModeOfItsOwn(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"paired with --record", []string{"--units", "--record", "absent.ndjson"}},
		{"paired with --status", []string{"--units", "--status"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeGroundUnitsFixture(t)
			require.NoFileExists(t, filepath.Join(dir, "absent.ndjson"))

			stdout, stderr, code := runTP(t, dir, append([]string{"ground", "spec.md"}, tc.args...)...)
			require.Equal(t, 2, code, "stdout: %s stderr: %s", stdout, stderr)
			assert.Equal(t, float64(2), groundErrorEnvelope(t, stderr)["code"])
			assert.Empty(t, stdout, "a refused pairing runs neither mode")
		})
	}
}
