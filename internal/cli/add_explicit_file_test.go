package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddWithMissingExplicitFileNamesThePath is the BUGS.md reproduction: with
// --file naming a path that is not there, `tp add` answered "no task file
// found. Use --spec to create one, or run tp init first" — advice that neither
// names the path the caller gave nor helps, because --spec creates a file named
// after the spec, not the file that was asked for. The error must name the path
// it could not open, exactly as the read commands do.
func TestAddWithMissingExplicitFileNamesThePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "a.md")

	_, statusErr, statusCode := runTP(t, dir, "--file", "missing.tasks.json", "status")
	require.Equal(t, 3, statusCode, statusErr)

	_, stderr, code := runTP(t, dir, "--file", "missing.tasks.json", "add", writeTargetTask("a1"))
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "missing.tasks.json", "the error names the path it could not open")
	assert.NotContains(t, stderr, "--spec", "--spec cannot create the file the caller named")
	assert.Equal(t, statusErr, stderr, "add reports the same discovery error as the read commands")
}

// TestAddWithMissingTPFileNamesThePath is the same defect through the
// environment: TP_FILE is as explicit a choice as --file, so a path that is not
// there is reported, not replaced with --spec advice.
func TestAddWithMissingTPFileNamesThePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "a.md")
	env := []string{"TP_FILE=missing.tasks.json"}

	_, statusErr, statusCode := runTPEnv(t, dir, env, "status")
	require.Equal(t, 3, statusCode, statusErr)

	_, stderr, code := runTPEnv(t, dir, env, "add", writeTargetTask("a1"))
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "missing.tasks.json", "the error names the path it could not open")
	assert.NotContains(t, stderr, "--spec", "--spec cannot create the file the caller named")
	assert.Equal(t, statusErr, stderr, "add reports the same discovery error as the read commands")
}

// TestAddWithMissingExplicitFileIgnoresSpec pins the corollary: --spec is an
// offer to create the auto-detected file, so it must not quietly create a
// DIFFERENT file when the caller already named one that is missing.
func TestAddWithMissingExplicitFileIgnoresSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "a.md")

	_, stderr, code := runTP(t, dir, "--file", "missing.tasks.json", "add", writeTargetTask("a1"), "--spec", "a.md")
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "missing.tasks.json", "the error names the path it could not open")
	assert.NoFileExists(t, dir+"/a.tasks.json", "--spec must not create a file under another name")
}
