package engine

import "strings"

// SuggestSimilarIDs returns up to 3 task IDs similar to the given ID.
// Uses prefix matching and word overlap (split on hyphens, 3+ char words).
// Returns nil if no matches found.
func SuggestSimilarIDs(givenID string, allIDs []string) []string {
	type match struct {
		id   string
		tier int // 0=prefix, 1=word overlap
	}

	var matches []match
	givenLower := strings.ToLower(givenID)
	givenWords := splitWords(givenLower)

	for _, id := range allIDs {
		if id == givenID {
			continue
		}
		idLower := strings.ToLower(id)

		// Tier 0: prefix match (given starts with id or vice versa)
		if strings.HasPrefix(givenLower, idLower) || strings.HasPrefix(idLower, givenLower) {
			matches = append(matches, match{id: id, tier: 0})
			continue
		}

		// Tier 1: word overlap (any word 3+ chars from given appears in id)
		idWords := splitWords(idLower)
		if hasWordOverlap(givenWords, idWords) {
			matches = append(matches, match{id: id, tier: 1})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort: tier 0 before tier 1, within same tier shorter IDs first
	for i := 0; i < len(matches); i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[j].tier < matches[i].tier ||
				(matches[j].tier == matches[i].tier && len(matches[j].id) < len(matches[i].id)) {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	// Return top 3
	result := make([]string, 0, 3)
	for i := 0; i < len(matches) && i < 3; i++ {
		result = append(result, matches[i].id)
	}
	return result
}

func splitWords(id string) []string {
	parts := strings.Split(id, "-")
	var words []string
	for _, p := range parts {
		if len(p) >= 3 {
			words = append(words, p)
		}
	}
	return words
}

func hasWordOverlap(a, b []string) bool {
	for _, wa := range a {
		for _, wb := range b {
			if wa == wb {
				return true
			}
		}
	}
	return false
}
