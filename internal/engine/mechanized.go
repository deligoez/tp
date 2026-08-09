package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// checkIsValid reports whether a single workflow `checks` entry is valid in the
// sense of §3.1: ValidateChecks accepts it *on its own* — the same single-entry
// call the mechanical-check runner already makes for each entry it runs.
//
// Validity is judged per entry, never over the slice, so one invalid entry never
// changes the treatment of another entry's class, in either direction. The
// validator's duplicate-class rule is cross-entry and is therefore structurally
// unreachable in this form: a class named by two entries is simply registered,
// and each list that names a class names it once.
func checkIsValid(c model.Check) bool {
	return ValidateChecks([]model.Check{c}) == nil
}

// IsMechanizedClass reports whether a finding class is mechanized by the
// effective workflow's checks (§3.1): a valid entry's `class` equals it exactly —
// byte-for-byte, with no trimming and no case folding, unlike AuditRowRole's
// role rule and like AuditRowIsPass's status rule.
//
// The two sides of that comparison are not symmetric, and that is why exactness
// matters. A registered class is constrained: ValidateChecks accepts only
// ^[a-z0-9]+(-[a-z0-9]+)*$, so an entry with an uppercase letter or surrounding
// whitespace is invalid and mechanizes nothing. A candidate class is not
// constrained at all — it is whatever a reviewer sub-agent wrote in a finding
// row — so "Duplicate-Line" and " duplicate-line " are reachable candidate
// classes. A lenient comparison would let a registered "duplicate-line" silently
// suppress them, retiring a suggestion for a class no check covers.
//
// An entry the validator rejects does not mechanize its class: registration is
// evidence that the class is mechanically checked, and an entry tp will never run
// is not such evidence.
//
// OverSpecificationClass is mechanized here like any other class. Its exemption
// is scoped to the reviewer exclusion list alone; see ReviewerExclusionClasses.
func IsMechanizedClass(checks []model.Check, class string) bool {
	for i := range checks {
		if checks[i].Class == class && checkIsValid(checks[i]) {
			return true
		}
	}
	return false
}

// ReviewerExclusionClasses returns the classes prompt emission lists under
// "Mechanically checked classes — do NOT report findings of these classes:"
// (§3.2). Membership takes three changes in this order and no others: entries the
// validator rejects are dropped, then OverSpecificationClass is dropped under
// §3.1's exemption, then the survivors collapse by class, keeping the first
// surviving occurrence.
//
// The order matters — collapsing first would let an invalid entry shadow a valid
// one naming the same class and remove a class §3.1 calls mechanized. The
// retained order is otherwise unchanged: registration order rather than sorted,
// and not the ascending mechanized_classes order of §3.3.
//
// OverSpecificationClass is dropped because tp appends a canonical-class
// instruction to every review prompt telling reviewers to raise it where the spec
// pins mechanism that belongs in a task's acceptance; listing it here would ship
// one prompt that both demands and forbids the same finding. That reason covers
// exactly this sink — the class stays mechanized for candidate suppression, so
// the register-a-check suggestion retires for it as it does for everything else.
//
// The result is always a non-nil slice. An empty one means the sentence is not
// appended at all, rather than one ending in an empty list.
func ReviewerExclusionClasses(checks []model.Check) []string {
	out := make([]string, 0, len(checks))
	seen := make(map[string]bool, len(checks))
	for i := range checks {
		class := checks[i].Class
		if !checkIsValid(checks[i]) || class == OverSpecificationClass || seen[class] {
			continue
		}
		seen[class] = true
		out = append(out, class)
	}
	return out
}
