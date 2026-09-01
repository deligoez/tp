package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clauseSuffixBytes is §1.1's priced length: -1 strip, +2, +287, +2, +177.
//
// It is the number the removal below is positional on. Using it rather than
// strings.TrimSuffix is the whole point of property 3: TrimSuffix removes
// nothing when the suffix is absent, so the second assertion would pass on a
// body that never got one.
const clauseSuffixBytes = 468

// TestSuffixIsAppendedOnceAndTheStripHappened is v0.36.0 §6.2 property 3.
//
// An earlier version of this comment called it "the only assertion in this
// release that fails on a double append or a missing strip". Audit round 4
// falsified that: clause_emission_test.go's sibling assertions catch both, over
// a superset of modes. What is unique here is the 468-byte POSITIONAL removal —
// TrimSuffix removes nothing when the suffix is absent, so a body that never
// received one satisfies the sibling's form.
//
// Property 1's "ends with" cannot see either defect. `…A B A B` ends with the suffix
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

			// Positional removal, not TrimSuffix: TrimSuffix removes nothing
			// when the suffix is absent, which would let a body that never
			// received one satisfy both assertions below.
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
