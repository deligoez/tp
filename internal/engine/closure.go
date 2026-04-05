package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// ParseAcceptanceCriteria splits the acceptance field into individual criteria.
func ParseAcceptanceCriteria(acceptance string) []string {
	// Split on ". " and "; "
	parts := strings.Split(acceptance, ". ")
	var result []string
	for _, p := range parts {
		for _, sub := range strings.Split(p, "; ") {
			trimmed := strings.TrimSpace(sub)
			// Remove trailing period
			trimmed = strings.TrimRight(trimmed, ".")
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
	}
	return result
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"at": true, "in": true, "with": true, "and": true, "or": true,
	"for": true, "to": true, "of": true, "be": true, "by": true,
	"on": true, "it": true, "has": true, "was": true,
}

var technicalTermRegex = regexp.MustCompile(`[A-Z][a-z]+[A-Z]|[a-z]+_[a-z]+|[A-Z]{2,}`)
var filePathRegex = regexp.MustCompile(`\S+/\S+|\S+\.\w{1,5}$`)

// ExtractKeywords extracts significant words from a criterion.
func ExtractKeywords(criterion string) []string {
	words := strings.Fields(criterion)
	keywords := make([]string, 0, len(words))
	for _, w := range words {
		clean := strings.Trim(w, ".,;:()\"'")
		if clean == "" {
			continue
		}
		// Keep file paths intact
		if filePathRegex.MatchString(clean) {
			keywords = append(keywords, clean)
			continue
		}
		// Keep technical terms
		if technicalTermRegex.MatchString(clean) {
			keywords = append(keywords, clean)
			continue
		}
		// Skip stop words
		if stopWords[strings.ToLower(clean)] {
			continue
		}
		keywords = append(keywords, clean)
	}
	return keywords
}

// VerifyClosure checks that the closure reason addresses all acceptance criteria.
// When gatePassed is true, keyword matching is relaxed (agent attests quality gate passed).
// When coveredBy is true, forbidden patterns and keyword matching are skipped
// (task is covered by another done task — not a deferral).
// Returns nil if valid, or an error describing what's missing.
func VerifyClosure(acceptance, reason string, gatePassed, coveredBy bool) error {
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

	// When gate-passed is attested, skip length check and keyword matching.
	// The agent already verified the quality gate; demanding long reasons
	// or exact keyword overlap wastes tokens for simple tasks.
	if gatePassed {
		return nil
	}

	criteria := ParseAcceptanceCriteria(acceptance)
	if len(criteria) == 0 {
		return nil
	}

	// Check 2: Minimum reason length
	if len(reason) < len(acceptance)/2 {
		return fmt.Errorf("closure reason too short (%d chars). Must be at least %d chars (half of acceptance text)", len(reason), len(acceptance)/2)
	}

	// Check 1: Per-criterion keyword match
	reasonLower := strings.ToLower(reason)
	for i, criterion := range criteria {
		keywords := ExtractKeywords(criterion)
		found := false
		for _, kw := range keywords {
			if strings.Contains(reasonLower, strings.ToLower(kw)) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("closure reason does not address criterion %d: %q", i+1, criterion)
		}
	}

	return nil
}

var (
	patDeferred         = regexp.MustCompile(`(?i)\bdeferred\b`)
	patWillBeDoneLater  = regexp.MustCompile(`(?i)\bwill be done later\b`)
	patCoveredByExist   = regexp.MustCompile(`(?i)\bcovered by existing\b`)
	patNotNeeded        = regexp.MustCompile(`(?i)\bnot needed\b`)
	patBecause          = regexp.MustCompile(`(?i)\bbecause\b`)
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
