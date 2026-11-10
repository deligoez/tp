package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicLink_AnyOfAddsAGroupAndReplacesItsWhy (spec/1.7.0.md §7): linking
// a member set adds a group; the same set in another order replaces that
// group's why, as a plain link does; a different set is a new group.
func TestEpicLink_AnyOfAddsAGroupAndReplacesItsWhy(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfGraph)

	stdout, stderr, code := runTP(t, dir, "epic", "link", "w", "--any-of", "x,y", "either opens the API")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "added", epicActionOf(t, stdout))
	assert.Equal(t, []map[string]any{anyOfEntry("either opens the API", "x", "y")}, storedDeps(t, dir, "w"))

	stdout, stderr, code = runTP(t, dir, "epic", "link", "w", "--any-of", "y, x", "either one ships the API")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "replaced", epicActionOf(t, stdout))
	assert.Equal(t, []map[string]any{anyOfEntry("either one ships the API", "x", "y")}, storedDeps(t, dir, "w"))

	stdout, stderr, code = runTP(t, dir, "epic", "link", "w", "--any-of", "x,z", "either feeds the report")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "added", epicActionOf(t, stdout))
	assert.Equal(t, []map[string]any{
		anyOfEntry("either one ships the API", "x", "y"),
		anyOfEntry("either feeds the report", "x", "z"),
	}, storedDeps(t, dir, "w"))
}

