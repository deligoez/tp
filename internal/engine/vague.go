package engine

import (
	"fmt"
	"regexp"
	"strings"
)

// VagueWord defines a flagged word and its suggested replacement.
type VagueWord struct {
	Pattern    *regexp.Regexp
	Word       string
	Suggestion string
}

var vagueWords = []VagueWord{
	{regexp.MustCompile(`(?i)\bappropriate\b`), "appropriate", "Specify the exact condition"},
	{regexp.MustCompile(`(?i)\brelevant\b`), "relevant", "List the specific items"},
	{regexp.MustCompile(`(?i)\bas needed\b`), "as needed", "Define when it's needed"},
	{regexp.MustCompile(`(?i)\betc\.\b`), "etc.", "List all items explicitly"},
	{regexp.MustCompile(`(?i)\bvarious\b`), "various", "Enumerate the specific items"},
	{regexp.MustCompile(`(?i)\bsome\b`), "some", "Specify which ones or how many"},
	{regexp.MustCompile(`(?i)\bproper\b`), "proper", "Define what \"proper\" means"},
	{regexp.MustCompile(`(?i)\bproperly\b`), "properly", "Define what \"properly\" means"},
}

// Finding represents a single lint finding.
type Finding struct {
	Line     int    `json:"line"`
	Severity string `json:"severity"` // "error", "warning", "info"
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Context  string `json:"context,omitempty"`
}

// CheckVagueLanguage scans lines for vague words and returns findings.
func CheckVagueLanguage(lines []string) []Finding {
	var findings []Finding
	for i, line := range lines {
		for _, vw := range vagueWords {
			if vw.Pattern.MatchString(line) {
				findings = append(findings, Finding{
					Line:     i + 1,
					Severity: "warning",
					Rule:     "vague-language",
					Message:  fmt.Sprintf("%q is vague — %s", vw.Word, vw.Suggestion),
					Context:  strings.TrimSpace(line),
				})
			}
		}
	}
	return findings
}
