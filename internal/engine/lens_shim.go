package engine

import (
	"fmt"
	"sort"
)

// LegacyLensSentinelAll is the reserved key in a legacy tp: lens block that fans
// its questions out to every active review role (§10.4). It is a sentinel, never
// a role id.
const LegacyLensSentinelAll = "all"

// TranslateLegacyLens applies the v0.25.0 back-compat shim (§10.4) for the
// retired tp: lens frontmatter. Given a spec's parsed frontmatter and the ids of
// the review roles active this round (the built-in regression role is excluded
// by the caller, and auditors are never in this list), it returns the focus
// questions the legacy lens implies keyed by active review role id, plus the
// warnings to surface.
//
// Precedence (§10.4):
//   - New form wins: when the spec carries tp.review_roles or tp.audit_roles, the
//     legacy lens is ignored with a single warning and no lens-derived overrides
//     are returned — never a three-way merge.
//   - Otherwise a non-empty lens auto-translates with a deprecation warning:
//     lens.all appends to every active review role's focus; lens.<role-id>
//     appends to that active review role, or warns when the id is not an active
//     review role. Order within a role is lens.all first, then the role-specific
//     questions. Auditors and the built-in regression role are never touched.
func TranslateLegacyLens(fm *Frontmatter, activeReviewRoleIDs []string) (overrides map[string][]string, warnings []string) {
	overrides = make(map[string][]string)
	warnings = make([]string, 0)
	if fm == nil || len(fm.Lens) == 0 {
		return overrides, warnings
	}

	// New role-override form wins; the legacy lens is ignored (never merged).
	if len(fm.ReviewRoles) > 0 || len(fm.AuditRoles) > 0 {
		warnings = append(warnings, "legacy tp: lens is ignored because tp.review_roles/tp.audit_roles is present; the new form wins")
		return overrides, warnings
	}

	warnings = append(warnings, "tp: lens is deprecated; migrate to tp.review_roles with a focus list — auto-translating for this run")

	active := make(map[string]bool, len(activeReviewRoleIDs))
	for _, id := range activeReviewRoleIDs {
		active[id] = true
	}

	// lens.all fans out to every active review role first (order: all then role-specific).
	if allQs := fm.Lens[LegacyLensSentinelAll]; len(allQs) > 0 {
		for _, id := range activeReviewRoleIDs {
			overrides[id] = append(overrides[id], allQs...)
		}
	}

	// Deterministically translate each role-specific lens key.
	keys := make([]string, 0, len(fm.Lens))
	for k := range fm.Lens {
		if k == LegacyLensSentinelAll {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, id := range keys {
		if !active[id] {
			warnings = append(warnings, fmt.Sprintf("tp: lens.%s targets %q which is not an active review role; ignored", id, id))
			continue
		}
		overrides[id] = append(overrides[id], fm.Lens[id]...)
	}

	return overrides, warnings
}
