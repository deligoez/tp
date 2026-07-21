package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitignore_NoTPActive verifies the repo-root .gitignore no longer lists the
// removed .tp-active marker (§11.2).
func TestGitignore_NoTPActive(t *testing.T) {
	root := locateRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err, "repo-root .gitignore is readable")
	for _, line := range strings.Split(string(data), "\n") {
		assert.NotEqual(t, ".tp-active", strings.TrimSpace(line), ".tp-active must not be gitignored after v0.25.0")
	}
}

// locateRepoRoot walks up from the test working directory until it finds go.mod.
func locateRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from the test directory")
		}
		dir = parent
	}
}
