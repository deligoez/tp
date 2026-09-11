package cli

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// TestBuildFindingsSummary_ClassOnlyRowShowsItsClass: the cell after the tag
// is the row's category, or its class when it has no category, so a row that
// uses class is not printed with an empty cell. A row with neither leaves no
// double space before the dash.
func TestBuildFindingsSummary_ClassOnlyRowShowsItsClass(t *testing.T) {
	t.Parallel()
	got := buildFindingsSummary([]reviewFinding{
		{Severity: "high", Class: "exit-code-drift", Location: "spec.md:6", Finding: "Finding B about parser"},
		{Severity: "medium", Category: "ambiguity", Class: "vague-term", Location: "spec.md:7", Finding: "both set"},
		{Severity: "low", Location: "spec.md:8", Finding: "neither set"},
		{Severity: "low", Class: "wording", Location: "spec.md:9", Finding: "accepted row", Resolved: &resolvedStatus{Status: "wontfix", Evidence: "intended"}},
	})
	assert.Contains(t, got, "  [HIGH] exit-code-drift — spec.md:6: Finding B about parser\n")
	assert.Contains(t, got, "  [MED] ambiguity — spec.md:7: both set\n", "category wins when both are present")
	assert.Contains(t, got, "  [LOW] — spec.md:8: neither set\n")
	assert.Contains(t, got, "  [WONTFIX] wording — spec.md:9: accepted row (wontfix: intended)\n")
	assert.NotContains(t, got, "]  —", "no empty cell")
}

// TestBuildFindingsSummary_CutsOnARuneBoundary: the finding text (80 bytes),
// the accepted reason (40) and a resolved row's text (60) are cut on a rune
// boundary. A byte cut kept the lead byte of a rune that straddles the cap,
// and the prompt carried invalid UTF-8.
func TestBuildFindingsSummary_CutsOnARuneBoundary(t *testing.T) {
	t.Parallel()
	// 'ı' and 'ş' are two bytes each and straddle the caps.
	text := strings.Repeat("a", 79) + "ışık" + strings.Repeat("b", 10)
	evidence := strings.Repeat("e", 39) + "ğ the rest of the reason"
	resolvedText := strings.Repeat("r", 59) + "ş" + strings.Repeat("x", 10)
	got := buildFindingsSummary([]reviewFinding{
		{Severity: "high", Category: "c", Location: "L1", Finding: text},
		{Severity: "high", Category: "c", Location: "L2", Finding: text, Resolved: &resolvedStatus{Status: "wontfix", Evidence: evidence}},
		{Severity: "high", Category: "c", Location: "L3", Finding: resolvedText, Resolved: &resolvedStatus{Status: "fixed", Evidence: "done"}},
	})
	assert.True(t, utf8.ValidString(got), "every cut lands on a rune boundary")
	assert.Contains(t, got, "L1: "+strings.Repeat("a", 79)+"...\n")
	assert.Contains(t, got, "(wontfix: "+strings.Repeat("e", 39)+"...)")
	assert.Contains(t, got, "L3: "+strings.Repeat("r", 59)+"...\n")
}

// TestBuildFindingsSummary_WontfixWithoutEvidenceStaysUnresolved: a wontfix
// with blank evidence accepts nothing — the clean grading and tp resume both
// read it as open — so the carry lists it as unresolved, with its severity,
// and prints no accepted section.
func TestBuildFindingsSummary_WontfixWithoutEvidenceStaysUnresolved(t *testing.T) {
	t.Parallel()
	got := buildFindingsSummary([]reviewFinding{
		{Severity: "high", Category: "c", Location: "L1", Finding: "f", Resolved: &resolvedStatus{Status: "wontfix", Evidence: "  "}},
	})
	assert.Contains(t, got, "UNRESOLVED findings from previous rounds — DO NOT re-report:\n  [HIGH] c — L1: f\n")
	assert.NotContains(t, got, "ACCEPTED")
}

// TestBuildFindingsSummary_DuplicateFollowsFindingOpen: the carry sorts a
// duplicate the way findingOpen does. With evidence it is accepted, so it is
// listed under the accepted header with its reason; with blank evidence it is
// open, so it is unresolved. The resolved list keeps only fixed rows.
func TestBuildFindingsSummary_DuplicateFollowsFindingOpen(t *testing.T) {
	t.Parallel()
	got := buildFindingsSummary([]reviewFinding{
		{Severity: "high", Category: "c", Location: "L1", Finding: "evidenced duplicate", Resolved: &resolvedStatus{Status: "duplicate", Evidence: "same as L9"}},
		{Severity: "high", Category: "c", Location: "L2", Finding: "blank duplicate", Resolved: &resolvedStatus{Status: "duplicate", Evidence: " "}},
		{Severity: "high", Category: "c", Location: "L3", Finding: "fixed row", Resolved: &resolvedStatus{Status: "fixed", Evidence: "rewrote the section"}},
	})
	assert.Equal(t, "UNRESOLVED findings from previous rounds — DO NOT re-report:\n"+
		"  [HIGH] c — L2: blank duplicate\n\n"+
		"ACCEPTED findings from previous rounds — DO NOT re-report; each was accepted for the reason given:\n"+
		"  [DUPLICATE] c — L1: evidenced duplicate (duplicate: same as L9)\n\n"+
		"Additionally, 1 findings from previous rounds were RESOLVED (fixed).\n"+
		"Resolved high/critical (DO NOT regress):\n"+
		"  [RESOLVED] L3: fixed row\n\n"+
		"Do not re-report resolved issues. Focus ONLY on NEW issues in the current spec.\n", got)
}
