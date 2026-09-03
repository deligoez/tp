package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseFloorIndexIsTheInverseOfFormatFloorIndex is the whole contract in one
// assertion: what the emission wrote, read back, is the rows it wrote.
//
// The fixture is a DERIVED index rather than a literal, and it carries a cut row
// BETWEEN two floor rows — so the property is asserted over both of §2.2's row
// shapes, over real hashes, ordinals and byte counts, and over the arrangement
// in which a cut row occupies an id rather than trailing the list.
//
// Stating the property as an inverse rather than as an expected literal is what
// makes it survive a change to the rendering: a formatter and a parser that move
// together still round-trip, and one that moves alone cannot.
func TestParseFloorIndexIsTheInverseOfFormatFloorIndex(t *testing.T) {
	rows := groundFloorWithACutUnit(t)

	parsed, err := ParseFloorIndex(FormatFloorIndex("abc1234def56", rows))
	require.NoError(t, err)
	assert.Equal(t, rows, parsed, "the index reads back as the rows it was rendered from")
}

// TestParseFloorIndexKeepsACutRowCut pins the one confusion the two shapes admit:
// a cut row is three fields and a floor row is five, so a parser that read any
// three-field line as id/anchor/hash would give the cut unit the hash "(cut)" —
// non-empty, which is precisely what §2.2 says makes a unit a floor unit.
//
// The consequence is asserted through GroundCoverageOf rather than only on the
// field, because that is where the defect is felt: the cut unit would enter the
// denominator and owe a disposition no reader was ever asked for.
func TestParseFloorIndexKeepsACutRowCut(t *testing.T) {
	parsed, err := ParseFloorIndex("# commit unknown\n" +
		"u1 §1 aabbccddeeff #1 24B\n" +
		"u2 §2 (cut)\n" +
		"# 1 in floor, 1 cut\n")
	require.NoError(t, err)
	require.Len(t, parsed, 2)

	assert.Equal(t, FloorIndexRow{ID: "u1", Anchor: "§1", TextSHA: "aabbccddeeff", Ordinal: 1, Bytes: 24}, parsed[0])
	assert.Equal(t, FloorIndexRow{ID: "u2", Anchor: "§2"}, parsed[1],
		"the ABSENCE of the hash is the cut (§2.2), so a cut row carries no hash, ordinal or length")
	assert.Equal(t, 1, GroundCoverageOf(parsed, nil).Emitted,
		"a cut unit owes no disposition, so it is not in §8's denominator")
}

// TestParseFloorIndexSkipsTheLinesThatAreNotRows: FormatFloorIndex brackets the
// rows with a commit line and a summary line, and both begin with `#`. A parser
// that took every line for a row would reject the very file the emission writes.
func TestParseFloorIndexSkipsTheLinesThatAreNotRows(t *testing.T) {
	parsed, err := ParseFloorIndex("# commit unknown\n\nu1 §1 aabbccddeeff #1 24B\n\n# 1 in floor, 0 cut\n")
	require.NoError(t, err)
	assert.Equal(t, []FloorIndexRow{{ID: "u1", Anchor: "§1", TextSHA: "aabbccddeeff", Ordinal: 1, Bytes: 24}}, parsed)
}

// TestParseFloorIndexRejectsALineThatIsNeitherShape: every case here is one
// field or one sigil away from a legal row, which is what a truncated or
// half-written floor looks like. Reading one of them as a row would put a
// wrong denominator behind §9's advisory, and the advisory's whole content is
// that denominator.
//
// The line number is asserted, not just the failure: the bad line sits third in
// every case, so a parser counting rows rather than lines — which the commit
// line makes different here — reports 2.
func TestParseFloorIndexRejectsALineThatIsNeitherShape(t *testing.T) {
	for name, bad := range map[string]string{
		"two fields":          "u1 §1",
		"four fields":         "u1 §1 aabbccddeeff #1",
		"six fields":          "u1 §1 aabbccddeeff #1 24B extra",
		"no ordinal sigil":    "u1 §1 aabbccddeeff 1 24B",
		"no byte suffix":      "u1 §1 aabbccddeeff #1 24",
		"ordinal not a numer": "u1 §1 aabbccddeeff #x 24B",
		"bytes not a number":  "u1 §1 aabbccddeeff #1 xB",
		"cut marker misspelt": "u1 §1 (cutt)",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseFloorIndex("# commit unknown\nu1 §1 aabbccddeeff #1 24B\n" + bad + "\n")
			require.Error(t, err, "a line that is neither shape is not a row")
			assert.Contains(t, err.Error(), "line 3", "the failure names the line of the file, not the row")
		})
	}
}
