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
