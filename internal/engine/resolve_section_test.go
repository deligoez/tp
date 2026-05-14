package engine

import (
	"reflect"
	"testing"
)

func makeHeadings(entries ...struct {
	Level int
	Text  string
}) []*Heading {
	out := make([]*Heading, len(entries))
	for i, e := range entries {
		out[i] = &Heading{Level: e.Level, Text: e.Text, Line: i + 1}
	}
	return out
}

func TestResolveSection_CanonicalExactMatch(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "4. Backend Migration"},
	)
	resolved, ambiguous, candidates := ResolveSection("## 4. Backend Migration", headings)
	if resolved != "## 4. Backend Migration" || ambiguous || candidates != nil {
		t.Fatalf("unexpected result: resolved=%q ambiguous=%v candidates=%v", resolved, ambiguous, candidates)
	}
}

func TestResolveSection_PlainTextSingleMatch(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "4. Backend Migration"},
		struct {
			Level int
			Text  string
		}{3, "4.1 Schema"},
	)
	resolved, ambiguous, candidates := ResolveSection("4. Backend Migration", headings)
	if resolved != "## 4. Backend Migration" || ambiguous || candidates != nil {
		t.Fatalf("unexpected result: resolved=%q ambiguous=%v candidates=%v", resolved, ambiguous, candidates)
	}
}

func TestResolveSection_PlainTextAmbiguous(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "Setup"},
		struct {
			Level int
			Text  string
		}{3, "Setup"},
	)
	resolved, ambiguous, candidates := ResolveSection("Setup", headings)
	if resolved != "" || !ambiguous {
		t.Fatalf("expected ambiguous, got resolved=%q ambiguous=%v", resolved, ambiguous)
	}
	want := []string{"## Setup", "### Setup"}
	if !reflect.DeepEqual(candidates, want) {
		t.Fatalf("candidates mismatch: got %v want %v", candidates, want)
	}
}

func TestResolveSection_NoMatch(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "Setup"},
	)
	resolved, ambiguous, candidates := ResolveSection("Nonexistent", headings)
	if resolved != "" || ambiguous || candidates != nil {
		t.Fatalf("expected no match, got resolved=%q ambiguous=%v candidates=%v", resolved, ambiguous, candidates)
	}
}

func TestResolveSection_WhitespaceTrim(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "Setup"},
	)
	cases := []string{"  Setup  ", "##  Setup", "## Setup  ", "  ## Setup"}
	for _, c := range cases {
		resolved, ambiguous, _ := ResolveSection(c, headings)
		if resolved != "## Setup" || ambiguous {
			t.Fatalf("input %q: got resolved=%q ambiguous=%v want ## Setup", c, resolved, ambiguous)
		}
	}
}

func TestResolveSection_EmptyInput(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "Setup"},
	)
	resolved, ambiguous, candidates := ResolveSection("", headings)
	if resolved != "" || ambiguous || candidates != nil {
		t.Fatalf("expected zero result for empty input, got resolved=%q ambiguous=%v candidates=%v", resolved, ambiguous, candidates)
	}
	resolved, _, _ = ResolveSection("    ", headings)
	if resolved != "" {
		t.Fatalf("whitespace-only input should not resolve, got %q", resolved)
	}
}

func TestResolveSection_CanonicalMissing(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "Setup"},
	)
	resolved, ambiguous, _ := ResolveSection("### Setup", headings)
	if resolved != "" || ambiguous {
		t.Fatalf("level-mismatched canonical should not resolve, got resolved=%q ambiguous=%v", resolved, ambiguous)
	}
}

func TestSuggestSimilarSections_TypoReturnsClosest(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "4. Backend Migration"},
		struct {
			Level int
			Text  string
		}{2, "4. Frontend Migration"},
		struct {
			Level int
			Text  string
		}{3, "4.1 Backend Schema"},
		struct {
			Level int
			Text  string
		}{2, "Unrelated"},
	)
	suggestions := SuggestSimilarSections("Bcakend Migration", headings)
	if len(suggestions) == 0 {
		t.Fatalf("expected suggestions for typo, got none")
	}
	found := false
	for _, s := range suggestions {
		if s == "## 4. Backend Migration" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected '## 4. Backend Migration' in suggestions, got %v", suggestions)
	}
	if len(suggestions) > 3 {
		t.Fatalf("expected max 3 suggestions, got %d: %v", len(suggestions), suggestions)
	}
}

func TestSuggestSimilarSections_NoMatchesFarApart(t *testing.T) {
	headings := makeHeadings(
		struct {
			Level int
			Text  string
		}{2, "Apple"},
	)
	suggestions := SuggestSimilarSections("Zucchini", headings)
	if suggestions != nil {
		t.Fatalf("expected nil for far-apart input, got %v", suggestions)
	}
}
