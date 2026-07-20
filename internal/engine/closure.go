package engine

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ParseAcceptanceCriteria splits the acceptance field into individual criteria.
// Supports three delimiters: ". " (period), "; " (semicolon), "\n- " (bullet list).
func ParseAcceptanceCriteria(acceptance string) []string {
	// First split on bullet list delimiter
	parts := strings.Split(acceptance, "\n- ")
	var result []string
	for _, p := range parts {
		// Then split on ". " and "; "
		for _, sub := range strings.Split(p, ". ") {
			for _, sub2 := range strings.Split(sub, "; ") {
				trimmed := strings.TrimSpace(sub2)
				// Remove trailing period and leading "- " prefix
				trimmed = strings.TrimRight(trimmed, ".")
				trimmed = strings.TrimPrefix(trimmed, "- ")
				trimmed = strings.TrimSpace(trimmed)
				if trimmed != "" {
					result = append(result, trimmed)
				}
			}
		}
	}
	return result
}

// CountEvidenceLines counts reason lines that start with "- " at column 0.
// Indented sub-bullets do not count, preserving one top-level line per criterion.
func CountEvidenceLines(reason string) int {
	n := 0
	for _, line := range strings.Split(reason, "\n") {
		if strings.HasPrefix(line, "- ") {
			n++
		}
	}
	return n
}

// ClosureError reports an evidence-line shortfall against the parsed criteria.
type ClosureError struct {
	Criteria      []string
	EvidenceLines int
}

func (e *ClosureError) Error() string {
	return fmt.Sprintf("acceptance has %d criteria but reason has %d evidence line(s)", len(e.Criteria), e.EvidenceLines)
}

// Hint enumerates the parsed criteria so the agent can write one evidence
// line per criterion without re-reading the task.
func (e *ClosureError) Hint() string {
	var b strings.Builder
	b.WriteString(`write one "- " evidence line at column 0 per criterion:`)
	for i, c := range e.Criteria {
		fmt.Fprintf(&b, " (%d) %s", i+1, c)
	}
	return b.String()
}

// ClosureHint returns the criteria-enumerating hint when err is a
// ClosureError, or fallback otherwise.
func ClosureHint(err error, fallback string) string {
	var ce *ClosureError
	if errors.As(err, &ce) {
		return ce.Hint()
	}
	return fallback
}

// VerifyClosure checks the closure reason against the acceptance criteria
// using the evidence-line rule: with N >= 2 criteria the reason must contain
// at least N lines starting with "- " at column 0; with N <= 1 any non-empty
// reason passes. When coveredBy is true, verification is skipped entirely
// (the referenced task already carries verified evidence).
// Returns nil if valid, or an error describing what's missing.
func VerifyClosure(acceptance, reason string, coveredBy bool) error {
	if strings.TrimSpace(reason) == "" {
		return fmt.Errorf("closure reason is required")
	}

	// covered-by skips all checks (referenced task already verified)
	if coveredBy {
		return nil
	}

	// Check forbidden patterns first
	if err := checkForbiddenPatterns(reason); err != nil {
		return err
	}

	criteria := ParseAcceptanceCriteria(acceptance)
	if len(criteria) <= 1 {
		return nil
	}

	if n := CountEvidenceLines(reason); n < len(criteria) {
		return &ClosureError{Criteria: criteria, EvidenceLines: n}
	}

	return nil
}

var (
	patDeferred        = regexp.MustCompile(`(?i)\bdeferred\b`)
	patWillBeDoneLater = regexp.MustCompile(`(?i)\bwill be done later\b`)
	patCoveredByExist  = regexp.MustCompile(`(?i)\bcovered by existing\b`)
	patNotNeeded       = regexp.MustCompile(`(?i)\bnot needed\b`)
	patBecause         = regexp.MustCompile(`(?i)\bbecause\b`)
)

func checkForbiddenPatterns(reason string) error {
	trimmed := strings.TrimSpace(reason)

	// Single-word reason
	if !strings.Contains(trimmed, " ") {
		return fmt.Errorf("closure reason must address each acceptance criterion with evidence")
	}

	if patDeferred.MatchString(reason) {
		return fmt.Errorf("deferral is forbidden. Leave the task open or complete it")
	}
	if patWillBeDoneLater.MatchString(reason) {
		return fmt.Errorf("deferral is forbidden. Leave the task open or complete it")
	}
	if patCoveredByExist.MatchString(reason) && !strings.Contains(reason, "/") {
		return fmt.Errorf("claim requires evidence. Include file path and line numbers")
	}
	if patNotNeeded.MatchString(reason) && !patBecause.MatchString(reason) {
		return fmt.Errorf("explain why the task is no longer needed")
	}

	return nil
}
