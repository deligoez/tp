package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// ResolveActiveCorpus resolves the active role panel for a phase, applying §6
// domain selection and filtering:
//
//   - No user role files: the embedded default corpus selected by the spec's
//     domain (§6.2a). An unknown domain falls back to the software corpus with a
//     lint warning (§6.1); prose selects its leaner two-reviewer panel (§6.3).
//   - User role files present: the corpus is those files filtered by each role's
//     domains field (§6.2b) — a role omitting the spec's domain is dropped, a role
//     with no domains applies everywhere. If filtering empties the panel, tp falls
//     back to that phase's full domain-selected embedded corpus (not re-filtered)
//     with a warning, so a project never reviews with zero roles.
//
// It returns the roles to emit plus any lint warnings. A malformed user role file
// propagates as an error (the caller aborts the phase, §3.6).
func ResolveActiveCorpus(startDir, domain, phase string) (roles []model.Role, warnings []string, err error) {
	warnings = make([]string, 0)
	embeddedDomain := domain
	domainUnknown := !HasDefaultCorpus(domain)
	if domainUnknown {
		embeddedDomain = "software"
	}

	userRoles, populated, err := LoadRoleCorpus(startDir, phase)
	if err != nil {
		return nil, warnings, err
	}

	var fallbackReason string
	if populated {
		filtered := make([]model.Role, 0, len(userRoles))
		for i := range userRoles {
			if roleAppliesToDomain(&userRoles[i], domain) {
				filtered = append(filtered, userRoles[i])
			}
		}
		if len(filtered) > 0 {
			return filtered, warnings, nil
		}
		fallbackReason = fmt.Sprintf("domain %q filtered out every %s role; using the embedded default panel", domain, phase)
	}

	// Fall back to the domain-selected embedded corpus: either no user files, or
	// an empty post-filter panel.
	if domainUnknown {
		warnings = append(warnings, fmt.Sprintf("unknown domain %q; using the software default corpus", domain))
	}
	if fallbackReason != "" {
		warnings = append(warnings, fallbackReason)
	}
	roles, err = DefaultCorpus(embeddedDomain, phase)
	return roles, warnings, err
}

// roleAppliesToDomain reports whether a role applies to the spec's domain (§6.2):
// a role with no domains applies to every domain; otherwise its domains must
// include the spec's domain.
func roleAppliesToDomain(r *model.Role, domain string) bool {
	if len(r.Domains) == 0 {
		return true
	}
	for _, d := range r.Domains {
		if d == domain {
			return true
		}
	}
	return false
}
