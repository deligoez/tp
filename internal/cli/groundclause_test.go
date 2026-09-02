package cli

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// quotedSentence matches §4.2's italicised quotations of the clause it overrides.
var quotedSentence = regexp.MustCompile(`(?s)\*"(.*?)"\*`)

// sentencesGroundMustNotCarry returns the sentences §4.2 quotes out of
// isolationClause as the ones that "forbid the probe the tier table requires".
//
// They are read from spec/1.0.0.md rather than written here so the claim is
// bound to the document that makes it: a §4.2 that stops quoting them, or an
// isolationClause reworded out from under the quote, fails the require below
// instead of leaving a stale literal quietly passing.
func sentencesGroundMustNotCarry(t *testing.T) []string {
	t.Helper()
	spec := readRepoDoc(t, "spec/1.0.0.md")
	i := strings.Index(spec, "### 4.2 Where the probe runs")
	require.GreaterOrEqual(t, i, 0, "spec/1.0.0.md must still carry §4.2")

	section := spec[i:]
	if end := strings.Index(section, "\n## "); end >= 0 {
		section = section[:end]
	}

	out := make([]string, 0, 2)
	for _, m := range quotedSentence.FindAllStringSubmatch(section, -1) {
		// The quotes are hard-wrapped in the spec; the constant is one line.
		sentence := strings.Join(strings.Fields(m[1]), " ")
		if strings.Contains(isolationClause, sentence) {
			out = append(out, sentence)
		}
	}
	require.Len(t, out, 2,
		"§4.2 quotes two sentences of isolationClause; %d of its quotations are still in the constant", len(out))
	return out
}

// TestGroundClauseDropsTheProhibitionItOverrides is why §4.2 needs a third
// constant at all: isolationClause forbids the probe §4.1's tier table
// requires, so the clause tp ground puts in its place must not carry it.
//
// The subject is the whole of a bounded artifact — one constant, with no
// elsewhere for a negation to go — which is the only shape in which a
// NotContains over prose decides anything. The same assertion aimed at an
// emitted prompt would be forgeable, because tp embeds spec text verbatim and
// this release's own spec quotes both sentences.
func TestGroundClauseDropsTheProhibitionItOverrides(t *testing.T) {
	for _, sentence := range sentencesGroundMustNotCarry(t) {
		assert.NotContains(t, groundIsolationClause, sentence,
			"§4.2 names this sentence as one that forbids the probe, so the ground clause must not carry it")
	}
}

// TestGroundClausePermitsTheCopyAndFencesTheRepository is §4.2's positive half:
// writing inside a copy the unit makes outside the repository is permitted, and
// every write inside the repository but the round's output file is not.
//
// This is a presence check over a bounded constant and it is worth exactly
// that: it cannot pin what the sentence means, only catch a rewrite that drops
// one of the two halves outright. The prohibition it replaces is pinned by
// TestGroundClauseDropsTheProhibitionItOverrides, where the assertion can fail.
func TestGroundClausePermitsTheCopyAndFencesTheRepository(t *testing.T) {
	assert.Contains(t, groundIsolationClause, "outside the repository",
		"§4.2: the probe runs in a copy the unit creates outside the repository")
	assert.Contains(t, groundIsolationClause, "write no file except the output file this prompt names",
		"§4.2: inside the repository every write but the round's own output file is forbidden")
}

// TestGroundClauseSuffixShape pins the separators as well as the parts. A
// suffix of the right length assembled in the wrong order — or with a single LF
// between the clauses — satisfies a length check alone.
func TestGroundClauseSuffixShape(t *testing.T) {
	got := groundClauseSuffix()

	assert.True(t, strings.HasPrefix(got, "\n\n"+groundIsolationClause),
		"the suffix opens with a blank line and §4.2's clause")
	assert.True(t, strings.HasSuffix(got, "\n\n"+incrementalClause),
		"the suffix closes with a blank line and §3.2's clause, which §4.2 leaves unchanged")
	assert.False(t, strings.HasSuffix(got, "\n"),
		"§2.3 fixes no trailing newline: the body ends on the clause")
	assert.Equal(t, 4, strings.Count(got, "\n"),
		"exactly four separator bytes, and neither clause carries its own newline")
}

// TestEachCommandCarriesItsOwnSuffix is §11 row 18, and the widened form of
// v0.36.0 §6.2 property 1: each command's prompts carry that command's suffix,
// byte for byte, asserted for all three commands over TWO suffixes.
//
// Two, not three: clauseSuffix() is built once and called by both
// appendClausesReview and appendClausesAudit, so review's and audit's are
// byte-identical and only ground's is new (spec/1.0.0-corrections.md C9).
//
// Each suffix is taken from what the command emitted rather than from the
// constant, so the equalities are a measurement of three emissions and not a
// restatement of two functions — and a body whose trailing newline survived the
// strip yields a suffix that is not the constant's, which fails here.
func TestEachCommandCarriesItsOwnSuffix(t *testing.T) {
	const head = "prompt body"
	const body = head + "\n"

	emitted := map[string]string{
		"review": appendClausesReview([]reviewPrompt{{Role: "r", OutputPath: "review-r1-r.ndjson", Prompt: body}})[0].Prompt,
		"audit":  appendClausesAudit([]auditPrompt{{Role: "r", OutputPath: "audit-r1-r.ndjson", Prompt: body}})[0].Prompt,
		"ground": appendClausesGround(body),
	}

	suffixes := make(map[string]string, len(emitted))
	for name, got := range emitted {
		require.True(t, strings.HasPrefix(got, head), "%s: the emission opens with the body it was given", name)
		suffixes[name] = strings.TrimPrefix(got, head)
	}

	assert.Equal(t, clauseSuffix(), suffixes["review"], "review carries §2.3's suffix byte for byte")
	assert.Equal(t, clauseSuffix(), suffixes["audit"], "audit carries the same one, because C9 says there is one")
	assert.Equal(t, groundClauseSuffix(), suffixes["ground"], "ground carries §4.2's suffix byte for byte")
	assert.NotEqual(t, suffixes["review"], suffixes["ground"],
		"row 18: ground's suffix differs from the one review and audit share")
}
