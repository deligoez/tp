package engine

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// checkClassRegex anchors the kebab-case class slug for workflow checks.
var checkClassRegex = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// ValidateChecks validates workflow.checks entries: class non-empty, matching
// the full-string kebab-case regex, unique within the array; cmd non-empty.
// The returned error names the first offending index.
func ValidateChecks(checks []model.Check) error {
	seen := make(map[string]int)
	for i := range checks {
		c := &checks[i]
		if c.Class == "" {
			return fmt.Errorf("checks[%d]: class must be non-empty", i)
		}
		if !checkClassRegex.MatchString(c.Class) {
			return fmt.Errorf("checks[%d]: class %q must match ^[a-z0-9]+(-[a-z0-9]+)*$", i, c.Class)
		}
		if prev, dup := seen[c.Class]; dup {
			return fmt.Errorf("checks[%d]: duplicate class %q (first at index %d)", i, c.Class, prev)
		}
		seen[c.Class] = i
		if strings.TrimSpace(c.Cmd) == "" {
			return fmt.Errorf("checks[%d]: cmd must be non-empty", i)
		}
	}
	return nil
}

// TaskAuditRolesHint is the hint a refused task_audit_roles write carries.
const TaskAuditRolesHint = `task_audit_roles is a JSON array of distinct kebab-case code role ids, e.g. ["go-safety"]; only code roles may audit a task, never spec-coverage`

// ValidateTaskAuditRoles validates workflow.task_audit_roles entries: each a
// non-empty kebab-case role id, unique within the array, and never
// spec-coverage, which audits a spec rather than a task's own files. Whether a
// role exists in the auditor corpus is not checked here. The returned error
// names the first offending index.
func ValidateTaskAuditRoles(roles []string) error {
	seen := make(map[string]int, len(roles))
	for i, role := range roles {
		if role == "" {
			return fmt.Errorf("task_audit_roles[%d]: role must be non-empty", i)
		}
		if !checkClassRegex.MatchString(role) {
			return fmt.Errorf("task_audit_roles[%d]: role %q must match ^[a-z0-9]+(-[a-z0-9]+)*$", i, role)
		}
		if role == RoleSpecCoverage {
			return fmt.Errorf("task_audit_roles[%d]: %s is not a code role; only code roles may audit a task", i, role)
		}
		if prev, dup := seen[role]; dup {
			return fmt.Errorf("task_audit_roles[%d]: duplicate role %q (first at index %d)", i, role, prev)
		}
		seen[role] = i
	}
	return nil
}
