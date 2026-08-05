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

// §7 item 14: the §3.2 and §3.3 rewrites actually landed. Item 13's deny-list
// alone passes against an emptied focus array, so the content is pinned here.
func TestDefaultAuditorCorpus_RewritesLanded(t *testing.T) {
	roles := embeddedAuditorRoles(t)

	security, ok := roles["corpus/software/auditors/security.json"]
	require.True(t, ok, "embedded software security auditor must exist")
	assert.Equal(t, []string{
		"Every acquired lock, transaction, or handle is released on all paths, including error and early-return paths.",
		"No swallowed failure: an ignored error return, an empty catch, or an error discarded instead of propagated with context is a finding.",
		"Values from user input, request parameters, or the environment are validated before they reach a query, a file path, or a command.",
		"Queries, paths, and commands are built with the platform's parameterizing or joining API, never by string concatenation from input.",
		"Authorization is checked at the point of the write, not only at the entry point, and never from a value the caller supplied.",
		"Files and directories the tool creates use restrictive permissions, and secrets never reach logs, error messages, or committed files.",
	}, security.Focus, "security.json focus must equal the six §3.2 lines verbatim")

	maint, ok := roles["corpus/software/auditors/maintainability-conventions.json"]
	require.True(t, ok, "embedded software maintainability auditor must exist")
	assert.Equal(t, []string{
		"Errors are propagated with context naming the failing operation, not rethrown bare and not flattened into a lossy string.",
		"Publicly reachable symbols carry a doc comment explaining intent rather than restating the signature.",
		"Functions stay short enough to read in one pass (roughly 80 lines); a longer one needs a stated reason.",
		"Naming follows the conventions already present in the surrounding code rather than introducing a competing style.",
		"No leftover TODO or FIXME without a ticket reference, no commented-out code, and no debug output left behind.",
	}, maint.Focus, "maintainability-conventions.json focus must equal the five §3.3 lines verbatim")
	assert.Contains(t, maint.Instructions,
		"only whether the code is maintainable and consistent with the conventions already present in this project")
}
