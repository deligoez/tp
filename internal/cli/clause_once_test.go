package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clauseSuffixBytes is the suffix's length, and the number the removal below is
// positional on. §1.1 prices it and distinguishes it from 467, the NET delta
// after the body's trailing newline is stripped. Which of the two is an error
// depends on the slot it sits in: the off-by-one §1.1's table exists to fix is
// 468 written where the net delta belongs, and calling the SUFFIX "467-byte" is
// the same error in the opposite direction. An earlier version of this comment
// inverted that attribution, naming 467 as the error the table prevents.
const clauseSuffixBytes = 468

// TestSuffixIsAppendedOnceAndTheStripHappened is v0.36.0 §6.2 property 3.
//
// This file makes NO uniqueness claim, and that is the third answer to the same
// question. It first said it was "the only assertion in this release" that
// catches a double append or a missing strip — falsified in round 4, since
// clause_emission_test.go catches both over a superset of modes. It was then
// narrowed to the 468-byte positional removal being what TrimSuffix cannot
// substitute for — falsified in round 5 by a 3x2 mutation matrix: the two forms
// are indistinguishable, because the case cited is unreachable behind the
// require.True below, which aborts first.
//
// What survives is not a claim about other tests but a statement about this one:
// property 1's "ends with" cannot see a double append or a byte removed before
// the suffix, and these two assertions can. Whether a sibling also can is not
// this comment's business, and asserting it twice cost two rounds. `…A B A B` ends with the suffix
// exactly as `…A B` does, and a byte removed before the suffix is invisible to
// a suffix check — so without this, an implementation that appends without
// stripping passes every other property, and §1.1 calls that one byte not
// optional.
//
// Decidable on any live emission, in one comparison, with no fixture and no
// second version of the binary.
func TestSuffixIsAppendedOnceAndTheStripHappened(t *testing.T) {
	suffix := clauseSuffixFromSpec(t)
	require.Len(t, suffix, clauseSuffixBytes,
		"§1.1 prices the suffix at %d bytes; the spec's own clauses must total that", clauseSuffixBytes)

	spec := relocatedSpec(t, "spec/0.36.0.md")
	dir := filepath.Dir(spec)
	base := filepath.Base(spec)

	subject := filepath.Join(dir, "subject.go")
	require.NoError(t, os.WriteFile(subject, []byte("package subject\n"), 0o600))

	// Both commands: the suffix is appended at one point per command, so a
	// defect can live in one and not the other.
	emissions := map[string][]string{
		"review": {"review", base},
		"audit":  {"audit", base, "--affected-files", "subject.go"},
	}

	checked := 0
	for name, args := range emissions {
		for _, p := range emitFor(t, dir, args...) {
			if p.outputPath == "" {
				continue
			}
			checked++
			label := name + "/" + p.role

			require.True(t, strings.HasSuffix(p.body, suffix),
				"%s names an output file, so property 1 must hold before property 3 means anything", label)
			require.Greater(t, len(p.body), clauseSuffixBytes,
				"%s: the body is longer than the suffix", label)

			head := p.body[:len(p.body)-clauseSuffixBytes]

			assert.False(t, strings.HasSuffix(head, suffix),
				"%s: the suffix is appended once, not twice", label)
			assert.False(t, strings.HasSuffix(head, "\n"),
				"%s: §2.3 removes the body's trailing newline before appending", label)
		}
	}

	require.Positive(t, checked,
		"both emissions must produce at least one prompt with an output_path, or this measures nothing")
}
