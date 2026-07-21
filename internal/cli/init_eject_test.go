package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitEjectRoles_Output writes the default software corpus into
// .tp/reviewers and .tp/auditors as editable files (§5.3).
func TestInitEjectRoles_Output(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	stdout, stderr, code := runTP(t, dir, "init", "--eject-roles")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "software", out["domain"])
	require.Len(t, out["ejected"].([]any), 6, "3 reviewers + 3 auditors")

	// The reviewer and auditor files exist and parse as their ids.
	implData, err := os.ReadFile(filepath.Join(dir, ".tp", "reviewers", "implementer.json"))
	require.NoError(t, err)
	var role map[string]any
	require.NoError(t, json.Unmarshal(implData, &role))
	assert.Equal(t, "implementer", role["id"])

	_, err = os.Stat(filepath.Join(dir, ".tp", "auditors", "security.json"))
	require.NoError(t, err, "auditors are ejected too")
}

// TestInitEjectRoles_UnknownDomain is a usage error (exit 2) listing the known
// domains (§5.3).
func TestInitEjectRoles_UnknownDomain(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	_, stderr, code := runTP(t, dir, "init", "--eject-roles", "--domain", "nonsense")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "unknown domain")
	assert.Contains(t, stderr, "software")
}

// TestInitEjectRoles_ProseDomain scaffolds the prose corpus (§5.3, §6.1).
func TestInitEjectRoles_ProseDomain(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	stdout, stderr, code := runTP(t, dir, "init", "--eject-roles", "--domain", "prose")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "prose", out["domain"])

	_, err := os.Stat(filepath.Join(dir, ".tp", "reviewers", "coherence.json"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(dir, ".tp", "reviewers", "implementer.json"))
	assert.Error(t, err, "prose corpus has no implementer role")
}

// TestInitEjectRoles_ForceOverwrite refuses to overwrite an existing role file
// unless --force, which overwrites regardless of the existing file's validity
// (§5.3, §3.6).
func TestInitEjectRoles_ForceOverwrite(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))

	_, _, code := runTP(t, dir, "init", "--eject-roles")
	require.Equal(t, 0, code)

	// A second eject without --force fails because the files already exist.
	_, stderr, code := runTP(t, dir, "init", "--eject-roles")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "already exists")

	// Corrupt a role file; --force overwrites it regardless of validity.
	impl := filepath.Join(dir, ".tp", "reviewers", "implementer.json")
	require.NoError(t, os.WriteFile(impl, []byte("garbage not json"), 0o600))
	_, stderr, code = runTP(t, dir, "init", "--eject-roles", "--force")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	data, err := os.ReadFile(impl)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"id": "implementer"`, "--force restored the embedded byte-identical file")
}
