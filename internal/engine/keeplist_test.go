package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeKeepPath_RelativeStartOutsideAnyRepository: outside a git
// repository and with no .tp/, the project root is the start directory, so
// a path given from "." is stored as written. NormalizeKeepPath errs only
// when a path cannot be resolved; a root left relative while the path is
// made absolute could not be related to it and would err here.
func TestNormalizeKeepPath_RelativeStartOutsideAnyRepository(t *testing.T) {
	dir := t.TempDir()
	if FindGitBoundary(dir) != "" {
		t.Skip("the temporary directory sits inside a git repository")
	}
	t.Chdir(dir)

	rel, err := NormalizeKeepPath(".", "notes/todo.md")

	require.NoError(t, err)
	assert.Equal(t, "notes/todo.md", rel)
}
