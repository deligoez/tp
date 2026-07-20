package engine

import (
	"fmt"
	"strings"
)

// Audit finding categories: mutually exclusive, no other/misc.
const (
	CategorySecurity      = "security"
	CategoryConcurrency   = "concurrency"
	CategoryErrorHandling = "error-handling"
	CategoryCorrectness   = "correctness"
	CategoryContract      = "contract"
)

// auditCategoryPrecedence lists the categories in resolution order, highest
// precedence first. It is the single source for both ResolveAuditCategory
// and the enum text RenderAuditCategoryText embeds in prompts.
var auditCategoryPrecedence = []string{
	CategorySecurity,
	CategoryConcurrency,
	CategoryErrorHandling,
	CategoryCorrectness,
	CategoryContract,
}

// IsValidCategory reports whether c is one of the five audit categories.
func IsValidCategory(c string) bool {
	for _, v := range auditCategoryPrecedence {
		if c == v {
			return true
		}
	}
	return false
}

// ResolveAuditCategory picks the single category for a finding to which
// several categories apply, using the resolution precedence
// security > concurrency > error-handling > correctness > contract.
// Unknown values are ignored; returns "" when no valid category is given.
func ResolveAuditCategory(candidates ...string) string {
	for _, p := range auditCategoryPrecedence {
		for _, c := range candidates {
			if c == p {
				return p
			}
		}
	}
	return ""
}

// RenderAuditCategoryText renders the category enum, the resolution
// precedence, and the presence rules for embedding in prompt output schemas.
func RenderAuditCategoryText() string {
	quoted := make([]string, len(auditCategoryPrecedence))
	for i, c := range auditCategoryPrecedence {
		quoted[i] = fmt.Sprintf("%q", c)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "category: one of %s — exactly one per finding.\n", strings.Join(quoted, " | "))
	fmt.Fprintf(&b, "When several apply, pick ONE by precedence: %s (do not split into two findings).\n", strings.Join(auditCategoryPrecedence, " > "))
	b.WriteString("No \"other\"/\"misc\": unknown values are rejected; if you cannot pick, the finding is too vague — rewrite it.\n")
	b.WriteString("Presence: category MUST be present in every row. status PASS -> category: null (explicit null, not omitted). status PARTIAL or FAIL -> category MUST be one of the enum values (not null).")
	return b.String()
}
