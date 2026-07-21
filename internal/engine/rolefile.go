package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// roleIDRe is the §3.2 kebab-case id pattern: lowercase alphanumerics in
// hyphen-separated groups.
var roleIDRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// allowedRoleKeys is the exact §3.3 top-level key set; any other key is a
// validation error (§3.4) — a role file can never redefine the output contract.
var allowedRoleKeys = map[string]bool{
	"id":           true,
	"title":        true,
	"instructions": true,
	"focus":        true,
	"domains":      true,
}

// RoleStem returns a role file's id-defining stem: the basename without its
// .json extension (security.json -> "security").
func RoleStem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// ParseRoleFile reads and validates a role JSON file at path. It is the single
// shared parser/validator for both the reviewer and auditor corpora (§3.4,
// Principle 1) — the phase is the caller's concern (directory), never the file's.
func ParseRoleFile(path string) (model.Role, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Role{}, fmt.Errorf("cannot read role file %s: %w", path, err)
	}
	role, err := ParseRoleBytes(data, RoleStem(path))
	if err != nil {
		return model.Role{}, fmt.Errorf("%s: %w", path, err)
	}
	return role, nil
}

// ParseRoleBytes validates raw role JSON whose filename stem is stem. The id MUST
// equal stem, match the kebab-case pattern, and not be the reserved regression id
// (§3.2); every top-level key must be in the §3.3 schema; domains (when present)
// must be a string array; and id/title/instructions are required.
func ParseRoleBytes(data []byte, stem string) (model.Role, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return model.Role{}, fmt.Errorf("invalid role JSON: %w", err)
	}

	// Reject any top-level key outside the schema, naming the offender (§3.4).
	for key := range raw {
		if !allowedRoleKeys[key] {
			return model.Role{}, fmt.Errorf("unknown top-level key %q (allowed: id, title, instructions, focus, domains)", key)
		}
	}

	// domains must be a string array when present.
	if rawDomains, ok := raw["domains"]; ok {
		var arr []string
		if err := json.Unmarshal(rawDomains, &arr); err != nil {
			return model.Role{}, fmt.Errorf("field \"domains\" must be a string array: %w", err)
		}
	}

	var role model.Role
	if err := json.Unmarshal(data, &role); err != nil {
		return model.Role{}, fmt.Errorf("invalid role JSON: %w", err)
	}

	if err := validateRole(&role, stem); err != nil {
		return model.Role{}, err
	}
	return role, nil
}

// validateRole enforces the §3.2 id rules and the §3.3 required fields on a
// parsed role given its expected filename stem.
func validateRole(role *model.Role, stem string) error {
	if role.ID == "" {
		return fmt.Errorf("role is missing required field \"id\"")
	}
	if role.ID != stem {
		return fmt.Errorf("role id %q must equal the filename stem %q", role.ID, stem)
	}
	if role.ID == RegressionRoleID {
		return fmt.Errorf("role id %q is reserved for the built-in regression role and cannot be a corpus file", RegressionRoleID)
	}
	if !roleIDRe.MatchString(role.ID) {
		return fmt.Errorf("role id %q must match ^[a-z0-9]+(-[a-z0-9]+)*$ (lowercase kebab-case)", role.ID)
	}
	if strings.TrimSpace(role.Title) == "" {
		return fmt.Errorf("role %q is missing required field \"title\"", role.ID)
	}
	if strings.TrimSpace(role.Instructions) == "" {
		return fmt.Errorf("role %q is missing required field \"instructions\"", role.ID)
	}
	return nil
}
