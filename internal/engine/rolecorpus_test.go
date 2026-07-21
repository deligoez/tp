package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeRole is a test helper that writes a minimal valid role file whose id
// equals the given stem.
func writeRole(t *testing.T, dir, stem string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := `{"id":"` + stem + `","title":"T","instructions":"I"}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, stem+".json"), []byte(body), 0o600))
}

// TestLoadRoleCorpus_PerPhaseReplacement loads reviewers from .tp/reviewers and
// treats an absent auditors directory as unpopulated (§4.4).
func TestLoadRoleCorpus_PerPhaseReplacement(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	writeRole(t, filepath.Join(root, ".tp", "reviewers"), "transaction-integrity")
	writeRole(t, filepath.Join(root, ".tp", "reviewers"), "idempotency")

	roles, populated, err := LoadRoleCorpus(root, PhaseReviewers)
	require.NoError(t, err)
	assert.True(t, populated, "a directory with >=1 .json is populated")
	got := []string{roles[0].ID, roles[1].ID}
	assert.Equal(t, []string{"idempotency", "transaction-integrity"}, got, "sorted by file name")

	// Auditors directory is absent -> unpopulated, embedded default applies.
	auditRoles, auditPopulated, err := LoadRoleCorpus(root, PhaseAuditors)
	require.NoError(t, err)
	assert.False(t, auditPopulated)
	assert.Empty(t, auditRoles)
}

// TestLoadRoleCorpus_EmptyDirectoryFallback treats a present-but-empty phase
// directory as unpopulated (§4.4).
func TestLoadRoleCorpus_EmptyDirectoryFallback(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp", "reviewers"), 0o755))

	roles, populated, err := LoadRoleCorpus(root, PhaseReviewers)
	require.NoError(t, err)
	assert.False(t, populated, "an empty directory is not populated")
	assert.Empty(t, roles)
}

// TestLoadRoleCorpus_NonJSONIgnored ignores non-JSON files: a directory holding
// only non-JSON files is unpopulated, and a mix loads only the .json roles.
func TestLoadRoleCorpus_NonJSONIgnored(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	revDir := filepath.Join(root, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "README.md"), []byte("notes"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, ".keep"), []byte(""), 0o600))

	roles, populated, err := LoadRoleCorpus(root, PhaseReviewers)
	require.NoError(t, err)
	assert.False(t, populated, "non-JSON files alone do not populate a phase")
	assert.Empty(t, roles)

	writeRole(t, revDir, "pedagogy")
	roles, populated, err = LoadRoleCorpus(root, PhaseReviewers)
	require.NoError(t, err)
	assert.True(t, populated)
	require.Len(t, roles, 1)
	assert.Equal(t, "pedagogy", roles[0].ID, "only the .json role is loaded")
}

// TestLoadRoleCorpus_NoTPDir returns unpopulated when there is no .tp/ at all.
func TestLoadRoleCorpus_NoTPDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	roles, populated, err := LoadRoleCorpus(root, PhaseReviewers)
	require.NoError(t, err)
	assert.False(t, populated)
	assert.Empty(t, roles)
}

// TestLoadRoleCorpus_MalformedFileErrors surfaces a parse/validation error so the
// caller can abort the phase (§3.6).
func TestLoadRoleCorpus_MalformedFileErrors(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	revDir := filepath.Join(root, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	// id does not equal the filename stem -> validation error.
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "bad.json"), []byte(`{"id":"other","title":"T","instructions":"I"}`), 0o600))

	_, populated, err := LoadRoleCorpus(root, PhaseReviewers)
	require.Error(t, err)
	assert.True(t, populated, "the directory is populated even though a file is invalid")
	assert.Contains(t, err.Error(), "must equal the filename stem")
}
