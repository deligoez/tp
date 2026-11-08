package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitEjectRolesGraph_WritesOnlyEpicReviewers ejects the three built-in
// graph roles into .tp/epic-reviewers/ and leaves .tp/reviewers/ untouched
// (spec/1.3.0.md §5, Roles).
func TestInitEjectRolesGraph_WritesOnlyEpicReviewers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	reviewers := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(reviewers, 0o755))
	custom := filepath.Join(reviewers, "custom.json")
	customBody := []byte(`{"id":"custom","title":"T","instructions":"I"}`)
	require.NoError(t, os.WriteFile(custom, customBody, 0o600))

	stdout, stderr, code := runTP(t, dir, "init", "--eject-roles", "--graph")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	require.Len(t, out["ejected"].([]any), 3, "the three built-in graph roles")

	for _, id := range []string{"dod-satisfiability", "why-auditor", "phase-economist"} {
		data, err := os.ReadFile(filepath.Join(dir, ".tp", "epic-reviewers", id+".json"))
		require.NoError(t, err)
		var role map[string]any
		require.NoError(t, json.Unmarshal(data, &role))
		assert.Equal(t, id, role["id"])
	}

	entries, err := os.ReadDir(reviewers)
	require.NoError(t, err)
	require.Len(t, entries, 1, ".tp/reviewers/ gains no file")
	got, err := os.ReadFile(custom)
	require.NoError(t, err)
	assert.Equal(t, customBody, got, "the existing reviewer file is byte-identical")
	_, err = os.Stat(filepath.Join(dir, ".tp", "auditors"))
	assert.True(t, os.IsNotExist(err), "no auditors directory is written")
}
