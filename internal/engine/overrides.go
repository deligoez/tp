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
// overrides (§5.2). Returns the effective roles (copies) plus warnings.
func ResolveOverrideFocus(roles []model.Role, fm *Frontmatter, phase string) (effective []model.Role, warnings []string) {
	warnings = make([]string, 0)
	overrides := make(map[string][]string)
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

	effective = make([]model.Role, len(roles))
	for i := range roles {
		effective[i] = roles[i]
		if extra := overrides[roles[i].ID]; len(extra) > 0 {
			eff := make([]string, 0, len(roles[i].Focus)+len(extra))
			eff = append(eff, roles[i].Focus...)
			eff = append(eff, extra...)
			effective[i].Focus = eff
		}
	}
	return effective, warnings
}
