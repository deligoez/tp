package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEject_EmissionNeutral: ejecting writes byte-identical role files, so each
// role's emitted prompt is unchanged from the embedded corpus (§5.4).
func TestEject_EmissionNeutral(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. X\ncontent\n"), 0o600))

	before, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)

	_, ejectStderr, code := runTP(t, dir, "init", "--eject-roles")
	require.Equal(t, 0, code, "eject: %s", ejectStderr)

	after, _, code := runTP(t, dir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)

	beforeByRole := reviewPromptsByRole(t, before)
	afterByRole := reviewPromptsByRole(t, after)
	for _, role := range []string{"implementer", "tester", "architect"} {
		assert.Equal(t, beforeByRole[role], afterByRole[role], "ejecting is emission-neutral: %s prompt is byte-identical", role)
	}
}

// TestEject_FlipsHashAndStalesOnce: a project on zero role files stays on the
// "builtin" sentinel and is not roles-stale; ejecting flips the reviewer hash
// from builtin to a file hash, staling the recorded round exactly once (§5.4).
func TestEject_FlipsHashAndStalesOnce(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))

	rolesStale := func() bool {
		stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
		require.Equal(t, 0, code, "status: %s", stderr)
		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		return out["roles_stale"].(bool)
	}

	// Record a review round while on the builtin corpus.
	_, _, code := runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	assert.False(t, rolesStale(), "zero role files stays on the builtin sentinel — no maintenance, not stale")

	// Eject flips the reviewer hash builtin -> file hash: the recorded round is
	// now stale once.
	_, ejectStderr, code := runTP(t, dir, "init", "--eject-roles")
	require.Equal(t, 0, code, "eject: %s", ejectStderr)
	assert.True(t, rolesStale(), "ejecting flips builtin->file hash, staling the recorded round once")

	// Recording a fresh round re-baselines the hash: no longer stale.
	_, _, code = runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	assert.False(t, rolesStale(), "the stale-once cost is a single re-confirming round")
}
