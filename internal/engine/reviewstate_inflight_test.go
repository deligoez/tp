package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteSnapshotAtomic_AppliesAndLeavesNoTmp(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	require.NoError(t, WriteSnapshotAtomic(specPath, 1, []byte("# Spec\nround 1\n")))

	data, err := os.ReadFile(filepath.Join(ReviewStateDir(specPath), "snapshot-round-1.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Spec\nround 1\n", string(data), "snapshot is a byte copy of the spec")

	entries, err := os.ReadDir(ReviewStateDir(specPath))
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "no .tmp leftover after a successful atomic write: %s", e.Name())
	}
}

func TestInFlightRound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	assert.Equal(t, 0, InFlightRound(specPath, 0), "no state dir → no in-flight round")

	require.NoError(t, WriteSnapshotAtomic(specPath, 1, []byte("# Spec\n")))
	assert.Equal(t, 1, InFlightRound(specPath, 0), "snapshot-round-1.md with 0 recorded rounds → in-flight 1")
	assert.Equal(t, 0, InFlightRound(specPath, 1), "next round (2) has no snapshot → not in-flight")

	require.NoError(t, WriteSnapshotAtomic(specPath, 2, []byte("# Spec v2\n")))
	assert.Equal(t, 2, InFlightRound(specPath, 1), "snapshot-round-2.md with 1 recorded round → in-flight 2")
}

func TestHasStateArtifacts_IgnoresTmpLeftover(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "snapshot-round-1.md.tmp"), []byte("partial"), 0o600))
	assert.False(t, hasStateArtifacts(dir), "a lone .tmp crash-leftover is not a state artifact")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "snapshot-round-1.md"), []byte("full"), 0o600))
	assert.True(t, hasStateArtifacts(dir), "a real snapshot file is a state artifact")
}
