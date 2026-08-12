package engine

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnsureTPGitignore_PreservesCustomEntriesUnderConcurrency guards a defect
// this release introduced and then had to take back out of the hot path.
//
// EnsureTPGitignore is a lock-free read-modify-write, and WithFileLock started
// calling it on every locked write. A process reading while another was
// mid-rewrite wrote back only the two entries tp wants, permanently dropping
// whatever else the user had in the file — measured at 7 losses in 25
// concurrent tp set runs, and silent, because every caller discards the error.
//
// The fix is the early return when nothing is missing, so the steady state
// performs no write at all. This test fails without it: with both wanted
// entries already present, any write is a bug, and concurrent writes lose data.
func TestEnsureTPGitignore_PreservesCustomEntriesUnderConcurrency(t *testing.T) {
	tpDir := t.TempDir()
	path := filepath.Join(tpDir, ".gitignore")
	custom := []string{"local.json", "locks/", "my-scratch/", "*.bak"}
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(custom, "\n")+"\n"), 0o600))

	before, err := os.Stat(path)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, EnsureTPGitignore(tpDir))
		}()
	}
	wg.Wait()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	for _, entry := range custom {
		assert.Contains(t, string(data), entry,
			"a user's own .gitignore entry must survive every call")
	}

	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime(),
		"with nothing missing there is nothing to write, so the file is never touched")
}

// TestEnsureTPGitignore_AppendsMissingEntryOnce: the one-time upgrade path still
// works, and having run once it stops writing.
func TestEnsureTPGitignore_AppendsMissingEntryOnce(t *testing.T) {
	tpDir := t.TempDir()
	path := filepath.Join(tpDir, ".gitignore")
	require.NoError(t, os.WriteFile(path, []byte("local.json\nmy-scratch/\n"), 0o600))

	require.NoError(t, EnsureTPGitignore(tpDir))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "locks/", "the missing entry is appended")
	assert.Contains(t, string(data), "my-scratch/", "and the user's entry is kept")

	stamped, err := os.Stat(path)
	require.NoError(t, err)
	require.NoError(t, EnsureTPGitignore(tpDir))
	again, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, stamped.ModTime(), again.ModTime(), "the second call writes nothing")
}
