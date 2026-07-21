package engine

import (
	"regexp"
	"strings"
)

// locTokenRe matches a section-anchor token: § followed by one or more digits
// and zero or more dotted sub-numbers (§8, §8.2, §8.2.1). It is anchored to the
// first occurrence via FindString.
var locTokenRe = regexp.MustCompile(`§\d+(\.\d+)*`)

// LocationKey reduces a finding's location to its first §<n>(.<n>)* token,
// ignoring any trailing text and any later § tokens (§8.2). The match is exact
// on the token, so §8 and §8.2 are distinct keys and never collapse together. A
// location with no § token uses its trimmed string verbatim as the key.
func LocationKey(location string) string {
	trimmed := strings.TrimSpace(location)
	if tok := locTokenRe.FindString(trimmed); tok != "" {
		return tok
	}
	return trimmed
}
