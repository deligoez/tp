package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComputeRolesHash_BuiltinSentinel returns "builtin" when a phase has no user
// role files (§9.1).
func TestComputeRolesHash_BuiltinSentinel(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	// No .tp/ at all.
	h, err := ComputeRolesHash(root, PhaseReviewers)
	require.NoError(t, err)
	assert.Equal(t, RolesHashBuiltin, h)

	// Present-but-empty phase directory is still builtin.
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp", "reviewers"), 0o755))
	h, err = ComputeRolesHash(root, PhaseReviewers)
	require.NoError(t, err)
	assert.Equal(t, RolesHashBuiltin, h)

	// A populated phase is not builtin.
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "reviewers", "x.json"),
		[]byte(`{"id":"x","title":"T","instructions":"I"}`), 0o600))
	h, err = ComputeRolesHash(root, PhaseReviewers)
	require.NoError(t, err)
	assert.NotEqual(t, RolesHashBuiltin, h)
	assert.Contains(t, h, "sha256:")
}

// TestComputeRolesHash_CrossPathStability keeps the hash identical across two
// clones at different checkout paths with the same repo-relative files (§9.1).
func TestComputeRolesHash_CrossPathStability(t *testing.T) {
	makeRepo := func(dir string) {
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		revDir := filepath.Join(dir, ".tp", "reviewers")
		require.NoError(t, os.MkdirAll(revDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(revDir, "a.json"),
			[]byte(`{"id":"a","title":"A","instructions":"IA"}`), 0o600))
		require.NoError(t, os.WriteFile(filepath.Join(revDir, "b.json"),
			[]byte(`{"id":"b","title":"B","instructions":"IB"}`), 0o600))
	}
	root1 := t.TempDir()
	root2 := t.TempDir()
	makeRepo(root1)
	makeRepo(root2)

	h1, err := ComputeRolesHash(root1, PhaseReviewers)
	require.NoError(t, err)
	h2, err := ComputeRolesHash(root2, PhaseReviewers)
	require.NoError(t, err)
	assert.Equal(t, h1, h2, "same repo-relative files hash identically regardless of checkout path")

	// Editing one file changes the hash.
	require.NoError(t, os.WriteFile(filepath.Join(root2, ".tp", "reviewers", "a.json"),
		[]byte(`{"id":"a","title":"A EDITED","instructions":"IA"}`), 0o600))
	h2b, err := ComputeRolesHash(root2, PhaseReviewers)
	require.NoError(t, err)
	assert.NotEqual(t, h1, h2b, "a role edit flips the hash")
}

// TestComputeRolesHash_CRLFNormalized makes CRLF and LF content hash identically
// (§9.1), so git autocrlf/eol never changes the hash.
func TestComputeRolesHash_CRLFNormalized(t *testing.T) {
	lf := "{\n  \"id\": \"a\",\n  \"title\": \"A\",\n  \"instructions\": \"I\"\n}\n"
	crlf := "{\r\n  \"id\": \"a\",\r\n  \"title\": \"A\",\r\n  \"instructions\": \"I\"\r\n}\r\n"

	makeRepo := func(dir, content string) {
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
		revDir := filepath.Join(dir, ".tp", "reviewers")
		require.NoError(t, os.MkdirAll(revDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(revDir, "a.json"), []byte(content), 0o600))
	}
	rootLF := t.TempDir()
	rootCRLF := t.TempDir()
	makeRepo(rootLF, lf)
	makeRepo(rootCRLF, crlf)

	hLF, err := ComputeRolesHash(rootLF, PhaseReviewers)
	require.NoError(t, err)
	hCRLF, err := ComputeRolesHash(rootCRLF, PhaseReviewers)
	require.NoError(t, err)
	assert.Equal(t, hLF, hCRLF, "CRLF is normalized to LF before hashing")
}

// TestComputeRolesHash_PhaseIndependent confirms editing a reviewer never changes
// the auditor hash — the phases are independent (§9.2).
func TestComputeRolesHash_PhaseIndependent(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp", "reviewers"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "reviewers", "r.json"),
		[]byte(`{"id":"r","title":"T","instructions":"I"}`), 0o600))

	auditBefore, err := ComputeRolesHash(root, PhaseAuditors)
	require.NoError(t, err)
	assert.Equal(t, RolesHashBuiltin, auditBefore, "no auditor files -> builtin, unaffected by reviewers")
}

// TestRolesStale covers roles_stale detection (§9.3) and the pre-v0.25.0
// matching rule (§9.4).
func TestRolesStale(t *testing.T) {
	assert.False(t, RolesStale(nil, "sha256:x"), "no rounds -> not stale")
	assert.False(t, RolesStale([]ReviewRound{{RolesHash: "sha256:a"}}, "sha256:a"), "matching hash -> not stale")
	assert.True(t, RolesStale([]ReviewRound{{RolesHash: "sha256:a"}}, "sha256:b"), "differing hash -> stale")
	assert.False(t, RolesStale([]ReviewRound{{RolesHash: ""}}, "sha256:b"), "pre-v0.25.0 round (no stored hash) is treated as matching")
	assert.False(t, RolesStale([]ReviewRound{{RolesHash: "sha256:a"}, {RolesHash: ""}}, "sha256:b"), "only the latest round matters; a pre-v0.25.0 latest matches")
}
