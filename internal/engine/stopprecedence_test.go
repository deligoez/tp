package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// declaredStopReasons reads every constant declared with type StopReason out of
// this package's own sources.
//
// The vocabulary is recovered from the declarations rather than transcribed,
// which is the whole point: a list this file happened to remember would let a
// tenth reason ship unranked and quietly lose to every one of the nine, while a
// list checked against the declarations cannot. Every non-test file is parsed,
// not just stopreason.go, so moving a constant to a neighbouring file does not
// shrink the vocabulary the ordering is measured against.
func declaredStopReasons(t *testing.T) []StopReason {
	t.Helper()

	sources, err := filepath.Glob("*.go")
	require.NoError(t, err)
	require.NotEmpty(t, sources, "the engine package's sources are where the vocabulary is declared")

	fset := token.NewFileSet()
	reasons := make([]StopReason, 0, len(stopPrecedence))
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, source, nil, 0)
		require.NoError(t, err, "parsing %s", source)

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				named, ok := value.Type.(*ast.Ident)
				if !ok || named.Name != "StopReason" {
					continue
				}
				for _, expr := range value.Values {
					lit, ok := expr.(*ast.BasicLit)
					require.True(t, ok, "a StopReason constant in %s is not a literal", source)
					require.Equal(t, token.STRING, lit.Kind)
					unquoted, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					reasons = append(reasons, StopReason(unquoted))
				}
			}
		}
	}
	require.NotEmpty(t, reasons, "the parse recovered this package's StopReason constants")
	return reasons
}

// Section 3.4's precedence sentence, transcribed: the recorded reason is the
// first of converged, driver-error, escalation, unit-failure, interrupted,
// cap-budget, cap-wall-clock, cap-units, no-units.
//
// The transcription is the claim; the second half of this test is what makes it
// exhaustive, by deriving the vocabulary from the package's own declarations.
// The order therefore covers the whole vocabulary by construction rather than
// by anyone remembering to extend it, which is what stops a tenth reason from
// entering the loop unordered.
func TestStopPrecedence_RanksTheWholeVocabularyInSection34sOrder(t *testing.T) {
	documented := []StopReason{
		StopConverged,
		StopDriverError,
		StopEscalation,
		StopUnitFailure,
		StopInterrupted,
		StopCapBudget,
		StopCapWallClock,
		StopCapUnits,
		StopNoUnits,
	}
	assert.Equal(t, documented, stopPrecedence[:], "the order is section 3.4's, verbatim")
	assert.Equal(t, StopConverged, stopPrecedence[0], "converged leads")

	ranked := make(map[StopReason]bool, len(stopPrecedence))
	for i, reason := range stopPrecedence {
		assert.False(t, ranked[reason], "%q is ranked once", reason)
		ranked[reason] = true
		assert.True(t, reason.Known(), "%q is inside the closed vocabulary", reason)
		assert.Equal(t, i, stopRank(reason))
	}

	for _, reason := range declaredStopReasons(t) {
		assert.True(t, ranked[reason],
			"%q has no precedence: decide where it sits among the others, then add it to stopPrecedence", reason)
	}
	assert.Len(t, stopPrecedence, 9, "section 3.4 names nine stop reasons")
}

// The rule itself, over every pair the vocabulary can form: whichever of two
// reasons stands earlier in the order is the one a checkpoint that satisfied
// both records, whichever order the caller happened to find them in.
//
// Driving it off stopPrecedence rather than off a hand-written list of pairs is
// deliberate — a tenth reason is ranked against all nine the moment it is
// placed, and a pair nobody thought to write down cannot go untested.
func TestHighestPrecedence_TheEarlierOfAnyTwoIsWhatIsRecorded(t *testing.T) {
	for i, first := range stopPrecedence {
		for _, second := range stopPrecedence[i+1:] {
			assert.Equal(t, first, highestPrecedence(first, second),
				"%q precedes %q", first, second)
			assert.Equal(t, first, highestPrecedence(second, first),
				"%q precedes %q whichever order they are given in", first, second)
		}
	}
}

