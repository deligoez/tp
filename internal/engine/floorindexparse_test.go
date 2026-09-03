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

