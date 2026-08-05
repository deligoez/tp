package engine

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// languageSpecificTokens is the §3.1 deny-list: tokens that name a Go idiom or
// tp's own Go rather than a property that holds in any language. Matched
// case-insensitively against every embedded auditor role's focus lines and
// instructions. Reviewer roles are deliberately out of scope — §3.1 establishes
// that implementer, tester, and architect are already neutral.
var languageSpecificTokens = []string{
	"%w",
	"0o600",
	"0o644",
	"camelCase",
	"_ = err",
	"goroutine",
	"lowercase packages",
	"files written by the tool",
	"idiomatic",
}

// embeddedAuditorRoles parses every embedded auditor role file, keyed by its
// corpus path.
func embeddedAuditorRoles(t *testing.T) map[string]model.Role {
	t.Helper()
	paths, err := fs.Glob(defaultCorpusFS, "corpus/*/auditors/*.json")
	require.NoError(t, err)
	require.NotEmpty(t, paths, "embedded auditor corpus must not be empty")

	roles := make(map[string]model.Role, len(paths))
	for _, path := range paths {
		data, err := defaultCorpusFS.ReadFile(path)
		require.NoError(t, err, "read %s", path)
		var role model.Role
		require.NoError(t, json.Unmarshal(data, &role), "parse %s", path)
		roles[path] = role
	}
	return roles
}

// §7 item 13: no embedded auditor prompt names a Go-specific idiom.
func TestDefaultAuditorCorpus_NoLanguageSpecificTokens(t *testing.T) {
	for path, role := range embeddedAuditorRoles(t) {
		lines := append([]string{role.Instructions}, role.Focus...)
		for _, line := range lines {
			lower := strings.ToLower(line)
			for _, token := range languageSpecificTokens {
				assert.NotContains(t, lower, strings.ToLower(token),
					"%s: language-specific token %q in %q", path, token, line)
			}
		}
	}
}
