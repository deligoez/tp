package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigration_NoRoleFilesIdenticalPanel: with no role files, the embedded
// default panel matches today's — software keeps implementer/tester/architect,
// and the only visible change is the prose default becoming the two prose lenses
// (§13.1).
func TestMigration_NoRoleFilesIdenticalPanel(t *testing.T) {
	sdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(sdir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	stdout, _, code := runTP(t, sdir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, code)
	byRole := reviewPromptsByRole(t, stdout)
	assert.Contains(t, byRole, "implementer")
	assert.Contains(t, byRole, "tester")
	assert.Contains(t, byRole, "architect")

	pdir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(pdir, "spec.md"),
		[]byte("---\ntp:\n  domain: prose\n---\n# Spec\ncontent\n"), 0o600))
	pstdout, _, pcode := runTP(t, pdir, "review", "spec.md", "--no-state")
	require.Equal(t, 0, pcode)
	pByRole := reviewPromptsByRole(t, pstdout)
	assert.Contains(t, pByRole, "coherence")
	assert.Contains(t, pByRole, "soundness")
	assert.NotContains(t, pByRole, "implementer", "prose no longer emits the swapped software personas")
}

// TestMigration_PreV0250RoundCarriesForward: an in-flight pre-v0.25.0 round has no
// stored role hash and is treated as matching, so upgrading tp never forces a
// re-review even when the current corpus differs (§13.2, §9.4).
func TestMigration_PreV0250RoundCarriesForward(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	// A user corpus so the CURRENT reviewer hash is not builtin.
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "r.json"),
		[]byte(`{"id":"r","title":"T","instructions":"I"}`), 0o600))

	// Hand-write a pre-v0.25.0 state.json: a recorded round with NO roles_hash.
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "review-round-1.ndjson"), []byte(""), 0o600))
	specHash, err := engine.SpecHash(specPath)
	require.NoError(t, err)
	state := `{"spec":"spec.md","review_rounds":[{"round":1,"findings":0,"clean":true,"recorded_at":"2026-01-01T00:00:00Z","file":"review-round-1.ndjson","spec_hash":"` + specHash + `"}],"audit_rounds":[]}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["roles_stale"], "a pre-v0.25.0 round (no stored role hash) is treated as matching")
	assert.Equal(t, false, out["stale"], "the spec is unchanged")
	assert.Equal(t, float64(1), out["consecutive_clean"], "the in-flight round carries forward")
}

// TestMigration_TPActiveNeedsNoAction: v0.24.0 already migrated active pointers to
// .tp/local.json, so a leftover .tp-active needs no user action — it is ignored
// and discovery uses the migrated pointer (§13.4).
func TestMigration_TPActiveNeedsNoAction(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.tasks.json"),
		[]byte(`{"version":1,"spec":"s.md","tasks":[]}`), 0o600))

	tpDir := filepath.Join(dir, ".tp")
	require.NoError(t, os.MkdirAll(tpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tpDir, "local.json"), []byte(`{"active":"app.tasks.json"}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp-active"), []byte("ghost.tasks.json\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "use")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, "local", out["source"], "discovery uses the migrated .tp/local.json pointer, not .tp-active")
	af, _ := out["active_file"].(string)
	assert.Contains(t, af, "app.tasks.json")
}
