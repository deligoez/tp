package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CheckVagueLanguage was the only rule in vague.go that did not skip fenced code
// blocks. Its four siblings — CheckDuplicateConsecutiveLines, CheckOrphanListItems,
// CheckDuplicateParagraphs and CheckBrokenCrossRefs — each track ``` delimiters;
// this one scanned every line.
//
// The consequence is not hypothetical and it is self-demonstrating: the rule fired
// on documentation *of itself*. spec/0.12.0-review-rounds.md shows a sample review
// finding whose text is `'appropriate' is vague`, inside a fenced block, and the
// rule reported it. A rule that cannot appear in a document describing it is a rule
// whose own docs carry a permanent warning.
//
// The fixture below is that block, reduced to the shape that matters.
func TestVagueLanguageSkipsFencedBlocks(t *testing.T) {
	t.Parallel()
	doc := `# Round input

Input NDJSON (same format as review output):

` + "```" + `
{"severity":"medium","finding":"'appropriate' is vague","suggestion":"Specify exact behavior"}
` + "```" + `

The round records what it read.
`
	got := CheckVagueLanguage(strings.Split(doc, "\n"))
	assert.Empty(t, got,
		"a vague word inside a fenced block is an example, not a requirement; got %+v", got)
}

// The same rule also scanned inline code spans, and that half is sharper because
// it makes the trigger list unquotable: a document that writes `appropriate` in
// backticks — which is how one names the word one is discussing — was reported.
// Measured on this repository: writing the refuted-candidates record produced a
// warning on its own citation of the rule's trigger word.
//
// Every word below is drawn from vagueWords rather than invented. A first draft of
// this fixture used "if necessary", which reads like a trigger and is not one — the
// list is eight words and that draft asserted membership without checking it.
func TestVagueLanguageSkipsInlineCodeSpans(t *testing.T) {
	t.Parallel()
	doc := "The trigger list covers `appropriate` and `various`, not every hedge.\n"

	got := CheckVagueLanguage(strings.Split(doc, "\n"))
	assert.Empty(t, got,
		"a vague word inside an inline code span is being named, not used; got %+v", got)
}

// The fixtures above and below name words this rule actually flags. Binding that
// to the artifact rather than to a memory of it is the point: if vagueWords loses
// a word these fixtures use, this test fails rather than quietly measuring nothing.
func TestVagueFixtureWordsAreRealTriggers(t *testing.T) {
	t.Parallel()
	for _, w := range []string{"appropriate", "various", "some"} {
		found := false
		for _, vw := range vagueWords {
			if vw.Word == w {
				found = true
				break
			}
		}
		assert.True(t, found, "%q is used as a fixture trigger and must be in vagueWords", w)
	}
}

// The other direction, and the half a skip-everything implementation would pass:
// prose outside any code construct must still be reported, and a fence that opens
// must not swallow the rest of the file.
func TestVagueLanguageStillReportsProse(t *testing.T) {
	t.Parallel()
	doc := `# Heading

Retry as appropriate.

` + "```" + `
handle(err) // various cases
` + "```" + `

Retry various times.
`
	lines := strings.Split(doc, "\n")
	got := CheckVagueLanguage(lines)
	require.Len(t, got, 2, "both prose lines are reported and the fenced one is not: %+v", got)

	// The expected line numbers are derived from the fixture rather than written
	// down. A first draft hardcoded them and was off by one; a wrong constant here
	// fails for a reason that has nothing to do with the rule under test.
	want := make([]int, 0, 2)
	for i, l := range lines {
		if strings.HasPrefix(l, "Retry ") || strings.HasPrefix(l, "Fail ") {
			want = append(want, i+1)
		}
	}
	require.Len(t, want, 2, "the fixture must contain exactly the two prose lines")
	assert.Equal(t, want[0], got[0].Line, "the line before the fence")
	assert.Equal(t, want[1], got[1].Line, "the line after the fence closes")
}

// A fence that never closes must not silently disable the rule for the rest of the
// file in a way the siblings do not: they treat an unterminated fence as open to
// EOF, and this rule must agree rather than invent a third behaviour.
func TestVagueLanguageTreatsUnterminatedFenceLikeItsSiblings(t *testing.T) {
	t.Parallel()
	doc := `Retry as appropriate.

` + "```" + `
this block never closes, and mentions as appropriate
`
	got := CheckVagueLanguage(strings.Split(doc, "\n"))
	require.Len(t, got, 1, "only the prose line before the fence: %+v", got)
	assert.Equal(t, 1, got[0].Line)
}
