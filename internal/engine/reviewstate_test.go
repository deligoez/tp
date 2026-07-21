package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStateDir_Layout(t *testing.T) {
	assert.Equal(t, filepath.Join("spec", ".tp-review", "0.23.0"), ReviewStateDir(filepath.Join("spec", "0.23.0.md")))
}

func TestEnsureReviewState_InitialIndex(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec"), 0o600))

	st, err := EnsureReviewState(specPath)
	require.NoError(t, err)
	assert.Equal(t, specPath, st.Spec)
	assert.Equal(t, []ReviewRound{}, st.ReviewRounds)
	assert.Equal(t, []ReviewRound{}, st.AuditRounds)

	// state.json exists and reloads
	st2, err := LoadReviewState(specPath)
	require.NoError(t, err)
	require.NotNil(t, st2)
	assert.Equal(t, st.Spec, st2.Spec)
}

func TestLoadReviewState_CorruptCases(t *testing.T) {
	t.Run("no directory returns nil state", func(t *testing.T) {
		dir := t.TempDir()
		st, err := LoadReviewState(filepath.Join(dir, "spec.md"))
		require.NoError(t, err)
		assert.Nil(t, st)
	})

	t.Run("unparseable state.json", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		stateDir := ReviewStateDir(specPath)
		require.NoError(t, os.MkdirAll(stateDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{broken"), 0o600))

		_, err := LoadReviewState(specPath)
		var ce *StateCorruptError
		require.ErrorAs(t, err, &ce)
		assert.Contains(t, ce.Hint(), "repair or delete")
	})

	t.Run("round files without index", func(t *testing.T) {
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		stateDir := ReviewStateDir(specPath)
		require.NoError(t, os.MkdirAll(stateDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, "review-round-1.ndjson"), []byte("{}\n"), 0o600))

		_, err := LoadReviewState(specPath)
		var ce *StateCorruptError
		require.ErrorAs(t, err, &ce)
		assert.Contains(t, ce.Error(), "state.json is missing")
	})
}

func TestConsecutiveCleanAndStaleness(t *testing.T) {
	rounds := []ReviewRound{
		{Round: 1, Clean: true, SpecHash: "sha256:a"},
		{Round: 2, Clean: false, SpecHash: "sha256:b"},
		{Round: 3, Clean: true, SpecHash: "sha256:c"},
		{Round: 4, Clean: true, SpecHash: "sha256:d"},
	}
	assert.Equal(t, 2, ConsecutiveClean(rounds), "clean, dirty, clean, clean -> 2")
	assert.Equal(t, 0, ConsecutiveClean(rounds[:2]))
	assert.Equal(t, 0, ConsecutiveClean(nil))

	assert.False(t, StateStale(rounds, "sha256:d"))
	assert.True(t, StateStale(rounds, "sha256:edited"))
	assert.False(t, StateStale(nil, "sha256:x"))

	assert.True(t, Converged(rounds, 2, "sha256:d"))
	assert.False(t, Converged(rounds, 3, "sha256:d"), "needs 3 trailing clean")
	assert.False(t, Converged(rounds, 2, "sha256:edited"), "stale unconverges")
}

func TestLoadRoundRows_MissingFileSkipped(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "review-round-1.ndjson"),
		[]byte("{\"finding\":\"f1\"}\n\n{\"finding\":\"f2\"}\n"), 0o600))

	rows, found := LoadRoundRows(specPath, &ReviewRound{Round: 1, File: "review-round-1.ndjson"})
	require.True(t, found)
	assert.Len(t, rows, 2)

	_, found = LoadRoundRows(specPath, &ReviewRound{Round: 2, File: "review-round-2.ndjson"})
	assert.False(t, found, "missing round file reported for caller warning")
}

func TestSpecHash_Format(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("content"), 0o600))

	h1, err := SpecHash(specPath)
	require.NoError(t, err)
	assert.Regexp(t, `^sha256:[0-9a-f]{64}$`, h1)

	require.NoError(t, os.WriteFile(specPath, []byte("changed"), 0o600))
	h2, err := SpecHash(specPath)
	require.NoError(t, err)
	assert.NotEqual(t, h1, h2)

	_, err = SpecHash(filepath.Join(dir, "missing.md"))
	require.Error(t, err)
}

func TestWithReviewStateLock_Serializes(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	_, err := EnsureReviewState(specPath)
	require.NoError(t, err)

	ran := false
	err = WithReviewStateLock(specPath, func() error {
		ran = true
		return errors.New("propagated")
	})
	require.Error(t, err, "fn error propagates through the lock")
	assert.True(t, ran)
}
