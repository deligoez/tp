package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fencedTextUnder returns the first ```text block that follows the named
// heading in the spec.
func fencedTextUnder(t *testing.T, spec, heading string) string {
	t.Helper()
	i := strings.Index(spec, heading)
	require.GreaterOrEqual(t, i, 0, "the spec must still carry the heading %q", heading)

	m := regexp.MustCompile("(?s)```text\n(.*?)\n```").FindStringSubmatch(spec[i:])
	require.Len(t, m, 2, "heading %q must be followed by a ```text block", heading)
	return m[1]
}

// TestClauseConstantsMatchTheSpec asserts the two constants against the spec's
// own fenced blocks rather than against literals repeated here.
//
// A literal in this file would be a third copy of each string, and three copies
// drift in pairs: the constant and the test would agree with each other while
// the spec said something else, which is the failure this guard exists to make
// impossible. Reading the source of truth is what makes it a guard rather than
// a restatement.
func TestClauseConstantsMatchTheSpec(t *testing.T) {
	spec := readRepoDoc(t, "spec/0.36.0.md")

	assert.Equal(t, fencedTextUnder(t, spec, "### 2.2 The clause"), isolationClause,
		"isolationClause must equal §2.2's fenced block byte for byte")
	assert.Equal(t, fencedTextUnder(t, spec, "### 3.2 The clause"), incrementalClause,
		"incrementalClause must equal §3.2's fenced block byte for byte")
}

// TestClauseConstantsAreSingleLines pins the shape §2.2 fixes: one line each,
// no embedded newline. The suffix §2.3 builds puts the separators in, so a
// clause carrying its own would double them and change the 468-byte figure
// §6.2 property 3 asserts.
//
// spec/1.0.0.md §4.2's clause is in the map because groundClauseSuffix() is
// assembled the same way; only the shipped pair carries the byte figure.
func TestClauseConstantsAreSingleLines(t *testing.T) {
	for name, clause := range map[string]string{
		"isolationClause":       isolationClause,
		"incrementalClause":     incrementalClause,
		"groundIsolationClause": groundIsolationClause,
	} {
		assert.NotContains(t, clause, "\n", "%s must be one line", name)
		assert.Equal(t, strings.TrimSpace(clause), clause,
			"%s must carry no leading or trailing whitespace", name)
	}
}

// TestClauseByteCounts pins the two figures §1.1 derives its table from, so a
// reworded clause fails here rather than in the suffix arithmetic downstream.
func TestClauseByteCounts(t *testing.T) {
	assert.Len(t, []byte(isolationClause), 287, "§1.1's table prices §2.2's clause at 287 bytes")
	assert.Len(t, []byte(incrementalClause), 177, "§1.1's table prices §3.2's clause at 177 bytes")
}

// TestAppendClausesSkipsAPromptWithNoOutputPath tests §2.3's predicate WHERE IT
// CAN FAIL.
//
// Audit round 1 measured that the guard inside appendClausesReview/Audit is
// unreachable through the CLI: the only call sites take the default panel, and
// every prompt there names an output file. Replacing the guard with a panic left
// the whole suite green, and deleting it left the end-to-end property-2 sweep
// green too — so that sweep, for all its breadth, asserts nothing about the
// predicate it is named for. It is a structural statement: the modes that
// produce an empty output_path never reach the appender at all.
//
// The predicate is still worth keeping, because "unreachable today" is a fact
// about today's call sites and not a property of the function. So it is tested
// as a unit, on a mixed slice the CLI cannot currently produce. Deleting the
// guard fails this and only this.
func TestAppendClausesSkipsAPromptWithNoOutputPath(t *testing.T) {
	suffix := clauseSuffix()

	t.Run("review", func(t *testing.T) {
		got := appendClausesReview([]reviewPrompt{
			{Role: "named", OutputPath: "r-named.ndjson", Prompt: "body\n"},
			{Role: "unnamed", OutputPath: "", Prompt: "body\n"},
		})
		require.Len(t, got, 2)
		assert.True(t, strings.HasSuffix(got[0].Prompt, suffix),
			"a prompt that names an output file gets the suffix")
		assert.Equal(t, "body\n", got[1].Prompt,
			"a prompt with no output file is left byte-identical, trailing newline included")
	})

	t.Run("audit", func(t *testing.T) {
		got := appendClausesAudit([]auditPrompt{
			{Role: "named", OutputPath: "a-named.ndjson", Prompt: "body\n"},
			{Role: "unnamed", OutputPath: "", Prompt: "body\n"},
		})
		require.Len(t, got, 2)
		assert.True(t, strings.HasSuffix(got[0].Prompt, suffix))
		assert.Equal(t, "body\n", got[1].Prompt)
	})
}
