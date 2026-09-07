package engine

import (
	"strconv"
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

// §6 row 8 — the exclusion is scoped to the one rule whose subject is prose, and
// the four tests above cannot see that scoping: none of them calls a sibling rule
// at all, so "only CheckVagueLanguage blanks inline spans" held by construction
// and was pinned by nothing.
//
// The sibling has to be CheckBrokenCrossRefs, and the referenced section has to
// hold a numbered list. Neither is a stylistic choice, and both cost this row a
// round when it was written the other way: blankInlineCode rewrites each line the
// same way, so equal lines stay equal and every duplicate rule is invariant under
// a mutant that blanks spans for it (measured 1→1 for duplicate-line,
// duplicate-paragraph and orphan-list-item, while numbering-gap takes []*Heading
// and never sees a line); and sectionStepCounts admits only sections that hold a
// numbered list, so a listless target gives broken-cross-ref 0 at HEAD and 0
// under the mutant and separates nothing.
//
// The mutant this test was observed failing against is blankInlineCode inserted
// into CheckBrokenCrossRefs, mirroring its use in CheckVagueLanguage. Under it,
// the backticked reference is blanked before the cross-ref patterns run and the
// Len assertion below fails 0 against 1, while CheckVagueLanguage's own count is
// 0 either way. The three require calls before it are fixture preconditions and
// stay green under that mutant on purpose: what fails is the requirement.
func TestBrokenCrossRefStillReportsWhereVagueLanguageIsSilent(t *testing.T) {
	t.Parallel()
	doc := "# Doc\n" +
		"\n" +
		"## 1.1 Retry policy\n" +
		"\n" +
		"1. Open the file\n" +
		"2. Close the file\n" +
		"\n" +
		"## 2. Notes\n" +
		"\n" +
		"See `§1.1 step 5` and treat `appropriate` as a word being named.\n"

	lines, headings := linesAndHeadings(t, doc)

	refIdx := -1
	for i, l := range lines {
		if strings.Contains(l, "`§1.1 step 5`") {
			refIdx = i
		}
	}
	require.GreaterOrEqual(t, refIdx, 0, "the reference must be present and inside backticks")

	// The fixture's separating properties are derived from the fixture, not
	// asserted about it: the referenced step must exceed a step count that is
	// itself non-zero. A row that skips the second half prescribes a listless
	// target, which is how this row was wrong for two rounds.
	m := crossRefSectionStep.FindStringSubmatch(lines[refIdx])
	require.NotNil(t, m, "the fixture line must carry a §X.Y step N reference")
	step, err := strconv.Atoi(m[2])
	require.NoError(t, err)
	steps := sectionStepCounts(lines, headings)[m[1]]
	require.Positive(t, steps, "the target section must hold a numbered list, or nothing separates")
	require.Greater(t, step, steps, "the referenced step must exceed that list's length")

	// And the vague word must be a real trigger, or the silence below measures
	// the absence of a trigger rather than the presence of an exclusion.
	require.NotEmpty(t, CheckVagueLanguage([]string{strings.ReplaceAll(lines[refIdx], "`", "")}),
		"the same line without its backticks must be reported")

	assert.Empty(t, CheckVagueLanguage(lines),
		"the trigger word is inside an inline span, so the prose rule stays silent")

	refs := CheckBrokenCrossRefs(lines, headings)
	require.Len(t, refs, 1,
		"a backticked line is still a broken reference: the structural sibling reports where the prose rule does not; got %+v", refs)
	assert.Equal(t, "broken-cross-ref", refs[0].Rule)
	assert.Equal(t, refIdx+1, refs[0].Line)
}

// Context quotes the spec, not the matcher's working copy. blankInlineCode exists
// so a trigger word inside an inline span is not matched; when its result was
// assigned back over the loop variable, the blanked text also reached
// Finding.Context, and every vague-language finding on a line carrying any inline
// span quoted a line that appears nowhere in the file. §3 is about what matches
// and says nothing about Context, so the blanking must not outlive the match.
//
// The fixture carries both halves on one line: a trigger word inside a span, which
// stays unreported, and a different trigger word in prose, which is reported
// quoting the line as written.
func TestVagueLanguageContextQuotesTheOriginalLine(t *testing.T) {
	t.Parallel()
	const line = "Handle `some cases` and various inputs here today."

	// Preconditions derived from the fixture rather than asserted about it: the
	// span must be one blankInlineCode actually changes, or Context is trivially
	// equal and the assertion below measures nothing; and stripping the backticks
	// must yield two findings, or the span is not suppressing a real trigger.
	require.NotEqual(t, line, blankInlineCode(line),
		"the fixture must carry an inline span blankInlineCode rewrites")
	require.Len(t, CheckVagueLanguage([]string{strings.ReplaceAll(line, "`", "")}), 2,
		"without its backticks the line carries two triggers, so the span suppresses exactly one")

	got := CheckVagueLanguage([]string{line})
	require.Len(t, got, 1, "only the prose trigger is reported: %+v", got)
	assert.Equal(t, line, got[0].Context,
		"Context must be the line as the document has it, not the blanked copy the matcher used")
}
