package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// determineAuditFiles routes an --affected-files failure to exit 3 and every
// other failure to exit 4. It used to classify by message substring, so the
// moment an error was reworded to carry its cause the routing silently
// changed: a permission error started exiting 4 with a "pass --affected-files"
// hint. These tests pin the sentinels the routing now depends on.

func TestResolveAuditFiles_MissingCarriesSentinel(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))

	_, _, err := resolveAuditFiles(spec, []string{filepath.Join(dir, "ghost.go")}, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errAffectedFileMissing), "a missing file carries errAffectedFileMissing")
	assert.False(t, errors.Is(err, errAffectedFileUnreadable))
}

func TestResolveAuditFiles_DirCarriesSentinel(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	_, _, err := resolveAuditFiles(spec, []string{sub}, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errAffectedPathIsDir), "a directory carries errAffectedPathIsDir")
}

func TestResolveAuditFiles_UnreadableCarriesSentinelAndCause(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root traverses a 0o000 directory, so the stat never fails")
	}
	dir := t.TempDir()
	spec := filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))

	// stat fails on a path whose parent cannot be traversed, not on a file
	// whose own mode is 0o000 — stat reads the directory entry, not the file.
	locked := filepath.Join(dir, "locked")
	require.NoError(t, os.Mkdir(locked, 0o755))
	target := filepath.Join(locked, "code.go")
	require.NoError(t, os.WriteFile(target, []byte("package main\n"), 0o600))
	require.NoError(t, os.Chmod(locked, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	_, _, err := resolveAuditFiles(spec, []string{target}, "")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errAffectedFileUnreadable), "an unreadable path carries errAffectedFileUnreadable")
	assert.False(t, errors.Is(err, errAffectedFileMissing), "it is not reported as missing")
	assert.True(t, errors.Is(err, os.ErrPermission), "the underlying cause survives the wrap")
}
