package engine

import (
	"fmt"
	"sort"

	"github.com/deligoez/tp/internal/model"
)

// ResolveOverrideFocus returns the active roles with their effective focus for a
// phase, applying the spec-frontmatter role overrides with read-time layering
// (§10.2, §10.3): effective focus = project-corpus focus ⊕ spec-override focus
// (project first). For review the override source is tp.review_roles, or — when
// that and tp.audit_roles are absent — the legacy tp: lens back-compat shim
// (§10.4); for audit it is tp.audit_roles. An override whose id matches no active
// role in the phase is ignored with a warning. The built-in regression role is
// appended to emission separately and never passed here, so it accepts no
// overrides (§5.2). Returns the effective roles (copies), the warnings, and the
// sorted ids of active roles this spec deactivated with an enabled: false
// override (§2.3); applying that drop is the caller's job.
func ResolveOverrideFocus(roles []model.Role, fm *Frontmatter, phase string) (effective []model.Role, warnings, disabled []string) {
	warnings = make([]string, 0)
	overrides := make(map[string]RoleOverride)
	fieldName := "audit_roles"

	if phase == PhaseReviewers {
		fieldName = "review_roles"
		switch {
		case len(fm.ReviewRoles) > 0:
			overrides = fm.ReviewRoles
		default:
			// No new review overrides: apply the legacy lens shim, which fans out
			// to the active review roles and warns about unknown lens keys itself.
			ids := make([]string, 0, len(roles))
			for i := range roles {
				ids = append(ids, roles[i].ID)
			}
			var lensWarnings []string
			overrides, lensWarnings = TranslateLegacyLens(fm, ids)
			warnings = append(warnings, lensWarnings...)
		}
	} else if len(fm.AuditRoles) > 0 {
		overrides = fm.AuditRoles
	}

	active := make(map[string]bool, len(roles))
	for i := range roles {
		active[roles[i].ID] = true
	}

	unknown := make([]string, 0)
	for id := range overrides {
		if !active[id] {
			unknown = append(unknown, id)
		}
	}
	sort.Strings(unknown)
	for _, id := range unknown {
		warnings = append(warnings, fmt.Sprintf("tp.%s override for %q matches no active %s role; ignored", fieldName, id, phase))
	}

	// The drop set: every active role this spec deactivated with enabled: false.
	// An id matching no active role takes the "matches no active role" path above
	// and never lands here, so a corpus without the role produces no drop (§2.3).
	// "Active" is post-domain-filter, so a role removed by domains and one whose
	// file does not exist are both excluded: §2.5's empty-phase message carries
	// only the ids this spec deactivated.
	disabled = make([]string, 0)
	for id, ov := range overrides {
		if active[id] && ov.Enabled != nil && !*ov.Enabled {
			disabled = append(disabled, id)
		}
	}
	sort.Strings(disabled)

	effective = make([]model.Role, len(roles))
	for i := range roles {
		effective[i] = roles[i]
		if extra := overrides[roles[i].ID].Focus; len(extra) > 0 {
			eff := make([]string, 0, len(roles[i].Focus)+len(extra))
			eff = append(eff, roles[i].Focus...)
			eff = append(eff, extra...)
			effective[i].Focus = eff
		}
	}
	return effective, warnings, disabled
}

// DropDisabledRoles removes the roles a spec deactivated with enabled: false
// from an already domain-filtered panel, returning the survivors (§2.3).
//
// The drop is deliberately applied here — outside ResolveActiveCorpus and after
// its domain filtering — so a spec that deactivates every user role empties the
// panel instead of tripping ResolveActiveCorpus's empty-panel fallback to the
// embedded default corpus. The caller decides what an empty panel means (§2.5).
func DropDisabledRoles(roles []model.Role, disabled []string) []model.Role {
	if len(disabled) == 0 {
		return roles
	}
	drop := make(map[string]bool, len(disabled))
	for _, id := range disabled {
		drop[id] = true
	}
	kept := make([]model.Role, 0, len(roles))
	for i := range roles {
		if !drop[roles[i].ID] {
			kept = append(kept, roles[i])
		}
	}
	return kept
}
