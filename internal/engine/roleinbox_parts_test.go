package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoleInboxFiles_ReadsOnlyPartsTheFenceLetsThrough: a part is spelled
// like a role id, the name the plugin's write fence lets a role write, so a
// file beside the role's whose part is spelled otherwise is not the role's.
func TestRoleInboxFiles_ReadsOnlyPartsTheFenceLetsThrough(t *testing.T) {
	t.Parallel()
	inbox := t.TempDir()
	dir := RoleInboxDirAt(inbox, "", PhaseAudit)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	for _, name := range []string{"round-1-security.ndjson", "round-1-security.a.ndjson", "round-1-security.A_1.ndjson", "round-1-security.b.c.ndjson"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o600))
	}
	got := RoleInboxFiles(inbox, "", PhaseAudit, 1, "security")
	assert.Equal(t, []string{filepath.Join(dir, "round-1-security.ndjson"), filepath.Join(dir, "round-1-security.a.ndjson")}, got)
}
