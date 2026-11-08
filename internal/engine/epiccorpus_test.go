package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicRoleIDs returns the ids of the active graph roles resolved from root.
func epicRoleIDs(t *testing.T, root string) []string {
	t.Helper()
	roles, err := ResolveEpicRoleCorpus(root)
	require.NoError(t, err)
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	return ids
}

// TestResolveEpicRoleCorpus_NoDirectoryYieldsBuiltIns is §9 row 22 at the
// corpus level: with no .tp/epic-reviewers/ the three built-in graph roles run,
// in the §5 table order.
func TestResolveEpicRoleCorpus_NoDirectoryYieldsBuiltIns(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))

	assert.Equal(t, []string{"dod-satisfiability", "why-auditor", "phase-economist"}, epicRoleIDs(t, root))
}

// TestResolveEpicRoleCorpus_OneFileReplacesBuiltIns is §9 row 22's second
// half: one file in .tp/epic-reviewers/ is the whole panel, never merged with
// the built-ins (the row's mutant).
func TestResolveEpicRoleCorpus_OneFileReplacesBuiltIns(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	writeRole(t, filepath.Join(root, ".tp", PhaseEpicReviewers), "why-auditor")

	assert.Equal(t, []string{"why-auditor"}, epicRoleIDs(t, root))
}
