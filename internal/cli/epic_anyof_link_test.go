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

// TestEpicUnlink_AnyOfRemovesOnlyTheExactSet: unlink --any-of removes the
// group whose member set it names, in any order; a set no group has, even a
// superset of one, exits 4 and writes nothing.
func TestEpicUnlink_AnyOfRemovesOnlyTheExactSet(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfGraph)
	for _, args := range [][]string{
		{"epic", "link", "w", "--any-of", "x,y", "either opens the API"},
		{"epic", "link", "w", "--any-of", "x,z", "either feeds the report"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}
	before := readEpicGraphFile(t, dir)
	_, stderr, code := runTP(t, dir, "epic", "unlink", "w", "--any-of", "x,y,z")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Equal(t, before, readEpicGraphFile(t, dir), "a refused unlink writes nothing")

	stdout, stderr, code := runTP(t, dir, "epic", "unlink", "w", "--any-of", "y,x")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, "removed", epicActionOf(t, stdout))
	assert.Equal(t, []map[string]any{anyOfEntry("either feeds the report", "x", "z")}, storedDeps(t, dir, "w"))
}

// TestEpicUnlink_APlainUnlinkOfAGroupMemberNamesTheGroup: x appears only
// inside w's group, so unlink w x finds no edge (exit 4) and its hint names
// the group and the unlink that removes it.
func TestEpicUnlink_APlainUnlinkOfAGroupMemberNamesTheGroup(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, anyOfGraph)
	_, stderr, code := runTP(t, dir, "epic", "link", "w", "--any-of", "x,y", "either opens the API")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	before := readEpicGraphFile(t, dir)

	_, stderr, code = runTP(t, dir, "epic", "unlink", "w", "x")
	require.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "one of {x, y}")
	assert.Contains(t, stderr, "tp epic unlink w --any-of x,y")
	assert.Equal(t, before, readEpicGraphFile(t, dir))
}

