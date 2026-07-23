package engine

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
