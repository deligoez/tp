package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicFileRevision is the documented contract: the lowercase hex SHA-256 of
// the graph file's bytes, computed here from the bytes the test read.
func epicFileRevision(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp", "epics.json"))
	require.NoError(t, err)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func epicRevisionOf(t *testing.T, stdout string) string {
	t.Helper()
	var out struct {
		Revision string `json:"revision"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out.Revision
}

// TestEpicWriters_ARevisionGuardRefusesAStaleRead: a client computes the
// revision from the bytes it read; another write lands; a write carrying the
// stale revision exits 4 and leaves the file as the other write left it. The
// revision a writer prints is the file's, so it lets the next write through.
// A guard compared against the revision after the write fails the first half.
func TestEpicWriters_ARevisionGuardRefusesAStaleRead(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	read := epicFileRevision(t, dir)

	stdout, stderr, code := runTP(t, dir, "epic", "set", "a", "title=A first")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	printed := epicRevisionOf(t, stdout)
	assert.Equal(t, epicFileRevision(t, dir), printed, "a writer prints the revision of what it wrote")

	_, stderr, code = runTP(t, dir, "epic", "set", "b", "title=B second", "--if-revision", read)
	assert.Equal(t, 4, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "revision")
	assert.Equal(t, printed, epicFileRevision(t, dir), "a stale write changes nothing")

	stdout, stderr, code = runTP(t, dir, "epic", "set", "b", "title=B second", "--if-revision", printed)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, epicFileRevision(t, dir), epicRevisionOf(t, stdout))
}

// TestEpicImport_ARevisionGuardRefusesAStaleRead: import takes the same
// guard; an absent file's revision is the empty string.
func TestEpicImport_ARevisionGuardRefusesAStaleRead(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, "")
	payload := epicImportPayload(t, epicChain)

	_, stderr, code := runTP(t, dir, "epic", "import", payload, "--if-revision", "deadbeef")
	assert.Equal(t, 4, code, "an absent file's revision is the empty string: stderr: %s", stderr)
	_, err := os.Stat(filepath.Join(dir, ".tp", "epics.json"))
	assert.True(t, os.IsNotExist(err), "a refused import writes nothing")

	stdout, stderr, code := runTP(t, dir, "epic", "import", payload, "--if-revision", "")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Equal(t, epicFileRevision(t, dir), epicRevisionOf(t, stdout))
}

// TestEpicLint_PrintsTheRevisionItRead: lint's revision is a convenience
// equal to the contract's formula over the file it read.
func TestEpicLint_PrintsTheRevisionItRead(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicChain)
	stdout, _, _ := runTP(t, dir, "epic", "lint")
	assert.Equal(t, epicFileRevision(t, dir), epicRevisionOf(t, stdout))
}
