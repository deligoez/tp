package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestClauseSuffixIsFourHundredSixtyEightBytes pins the figure §1.1's table
// derives everything else from, and §6.2 property 3 asserts on the emitted
// body.
//
// 468 is the suffix; 467 is the net delta, because §2.3's construction removes
// the body's trailing newline before appending. Two drafts of the spec confused
// the two in opposite directions, so both numbers are asserted where they are
// produced rather than left to arithmetic at the call site.
func TestClauseSuffixIsFourHundredSixtyEightBytes(t *testing.T) {
	t.Parallel()
	assert.Len(t, []byte(clauseSuffix()), 468,
		"§1.1: 2 + 287 + 2 + 177; the net delta is one less, after the strip")
}

// TestClauseSuffixShape pins the separators as well as the length. A suffix of
// the right size assembled in the wrong order — or with a single LF between the
// clauses — would satisfy a length check alone.
func TestClauseSuffixShape(t *testing.T) {
	t.Parallel()
	got := clauseSuffix()

	assert.True(t, strings.HasPrefix(got, "\n\n"+isolationClause),
		"the suffix opens with a blank line and §2.2's clause")
	assert.True(t, strings.HasSuffix(got, "\n\n"+incrementalClause),
		"the suffix closes with a blank line and §3.2's clause")
	assert.False(t, strings.HasSuffix(got, "\n"),
		"§2.3 fixes no trailing newline: the body ends on the clause")
	assert.Equal(t, 4, strings.Count(got, "\n"),
		"exactly four separator bytes, and neither clause carries its own newline")
}
