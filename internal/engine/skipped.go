package engine

import (
	"regexp"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// SkippedRole names a corpus role that produced no prompt, with the reason
// (§9.1). Every non-emitted corpus role is listed so a silent skip is
// distinguishable from a misconfigured corpus. Reason is one of:
//   - no-checklist-items: the role's focus/checklist was empty
//   - no-spec-change:     --diff-from is active and the role's focus is outside
//     the changed sections
//   - domain-mismatch:    the role's domains omit the spec's domain
//   - no-baseline:        the built-in regression role at round 1, which has no
//     snapshot-round-0.md to diff against
type SkippedRole struct {
	Role   string `json:"role"`
	Reason string `json:"reason"`
}

// Skipped reason codes (§9.1).
const (
	SkipNoChecklistItems = "no-checklist-items"
	SkipNoSpecChange     = "no-spec-change"
	SkipDomainMismatch   = "domain-mismatch"
	SkipNoBaseline       = "no-baseline"
)

// DomainSkippedRoles returns the user corpus roles for a phase that were dropped
// by domain filtering (§9.1, reason domain-mismatch). It loads the committed
// user role files and reports those whose domains omit the spec's domain. When
// no user corpus is present (embedded defaults apply) the result is empty: the
// embedded corpus is domain-selected, never domain-filtered, so no role is
// skipped for a domain mismatch.
func DomainSkippedRoles(startDir, domain, phase string) []SkippedRole {
	userRoles, populated, err := LoadRoleCorpus(startDir, phase)
	if err != nil || !populated {
		return []SkippedRole{}
	}
	out := make([]SkippedRole, 0)
	for i := range userRoles {
		if !roleAppliesToDomain(&userRoles[i], domain) {
			out = append(out, SkippedRole{Role: userRoles[i].ID, Reason: SkipDomainMismatch})
		}
	}
	return out
}

var (
	sectionRefRe = regexp.MustCompile(`§\s*(\d+(?:\.\d+)*)`)
	headingNumRe = regexp.MustCompile(`^\d+(?:\.\d+)*`)
)

// headingSectionNumber extracts the leading N(.M)* section number from a markdown
// heading ("### 9.1 Foo" -> "9.1"); "" when the heading is unnumbered.
func headingSectionNumber(heading string) string {
	h := strings.TrimSpace(strings.TrimLeft(heading, "#"))
	return headingNumRe.FindString(h)
}

// sectionRefs returns the N(.M)* section numbers referenced by § cross-references
// in a focus entry ("Review §9.1 and §2.3" -> ["9.1","2.3"]).
func sectionRefs(focus string) []string {
	matches := sectionRefRe.FindAllStringSubmatch(focus, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return out
}

// RoleFocusOutsideDiff reports whether a reviewer role's focus is entirely outside
// the changed sections under an explicit --diff-from baseline (§9.1, reason
// no-spec-change). A focus entry scopes to a section only via an explicit §N.M
// cross-reference (tp's canonical section syntax); generic questions with no §
// reference keep the role emitted, since the reviewer self-scopes against the
// changed-sections block injected into every prompt. The role is skipped when at
// least one focus entry carries a § reference and every referenced section is
// unchanged (none appears in the changed or removed set). An empty diff never
// triggers a skip, so this never fires when there is nothing to review.
func RoleFocusOutsideDiff(role model.Role, dr DiffResult) bool {
	if len(dr.Changed) == 0 && len(dr.Removed) == 0 {
		return false
	}
	changed := make(map[string]bool)
	for _, s := range dr.Changed {
		if n := headingSectionNumber(s.Heading); n != "" {
			changed[n] = true
		}
	}
	for _, s := range dr.Removed {
		if n := headingSectionNumber(s.Heading); n != "" {
			changed[n] = true
		}
	}
	if len(changed) == 0 {
		return false
	}
	scoped := false
	for _, f := range role.Focus {
		for _, n := range sectionRefs(f) {
			scoped = true
			if changed[n] {
				return false
			}
		}
	}
	return scoped
}
