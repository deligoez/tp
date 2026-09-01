package engine

import "github.com/deligoez/tp/internal/model"

// RoleIsRecognised reports whether tp knows the name anywhere, which is what
// v0.36.0 §4.2.1 uses to separate an exit-0 `--role` from an exit-2 one.
//
// Recognition deliberately spans BOTH phases. `tp review` neither emits nor
// skips an auditor id, so a set built from one phase's emission would make an
// auditor's own `tp review <spec> --role <id>` a usage error for a name the
// repository's own corpus defines — and §4.2.3 makes exactly that command a
// unit's first call.
//
// The user corpus is consulted unfiltered by domain: a role the spec's domain
// drops is still a name tp knows, and emission already names it in
// skipped_roles. The embedded default corpus is consulted too, because a repo
// with no role files runs on it, and `regression` because it is built in and
// belongs to no corpus at all.
func RoleIsRecognised(startDir, domain, name string) bool {
	if name == "" {
		return false
	}
	if name == RegressionRoleID {
		return true
	}
	for _, phase := range []string{PhaseReviewers, PhaseAuditors} {
		if roleNamedIn(userCorpusOrNil(startDir, phase), name) {
			return true
		}
		if roleNamedIn(defaultCorpusOrNil(domain, phase), name) {
			return true
		}
	}
	return false
}

// userCorpusOrNil returns the phase's user role files, or nothing when the repo
// has none or they cannot be read. A malformed corpus is not this predicate's
// error to report: emission has already failed on it by the time a caller here
// would care.
func userCorpusOrNil(startDir, phase string) []model.Role {
	roles, populated, err := LoadRoleCorpus(startDir, phase)
	if err != nil || !populated {
		return nil
	}
	return roles
}

// defaultCorpusOrNil returns the embedded panel for a phase, falling back to the
// software corpus for a domain that has none — the same fallback
// ResolveActiveCorpus applies, so the two agree on what a name can be.
func defaultCorpusOrNil(domain, phase string) []model.Role {
	if !HasDefaultCorpus(domain) {
		domain = "software"
	}
	roles, err := DefaultCorpus(domain, phase)
	if err != nil {
		return nil
	}
	return roles
}

func roleNamedIn(roles []model.Role, name string) bool {
	for i := range roles {
		if roles[i].ID == name {
			return true
		}
	}
	return false
}
