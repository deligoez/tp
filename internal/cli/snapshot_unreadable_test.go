package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tpError is the JSON error object tp writes to stderr: {code, error, hint}.
type tpError struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
	Hint  string `json:"hint"`
}

// lastJSONError parses the last JSON object line of stderr — warnings tp emits
// before the refusal share the stream.
func lastJSONError(t *testing.T, stderr string) tpError {
	t.Helper()
	var line string
	for l := range strings.SplitSeq(stderr, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "{") {
			line = strings.TrimSpace(l)
		}
	}
	require.NotEmpty(t, line, "no JSON error object on stderr: %s", stderr)
	var e tpError
	require.NoError(t, json.Unmarshal([]byte(line), &e), "stderr: %s", stderr)
	return e
}

// breakSnapshot replaces a round's snapshot with a directory of the same name,
// which every reader of it then fails to read.
//
// A directory rather than a 0o000 file: both reach the one os.ReadFile error
// branch — tp keys nothing on EISDIR versus EACCES — and the directory case
// runs everywhere, where the mode-bit case has to skip itself under root (the
// reason v0.34.0 kept the directory half of that pair and dropped the twin).
// It returns the snapshot's path as tp names it — relative to the working
// directory, the way the spec was named on the command line.
func breakSnapshot(t *testing.T, dir, name string) string {
	t.Helper()
	rel := filepath.Join(".tp-review", "spec", name)
	path := filepath.Join(dir, rel)
	require.FileExists(t, path, "the emission must really have written the snapshot")
	require.NoError(t, os.Remove(path))
	require.NoError(t, os.Mkdir(path, 0o755))
	return rel
}

// TestReviewRecord_UnreadableSnapshotNamesItAndTheWayOut: a round's snapshot is
// what its recorded spec_hash is taken from, so a snapshot that exists and
// cannot be read leaves "which text did this round read" unanswerable and
// --record refuses. The refusal has to say which file and what to do about it —
// the generic state-directory advice sends a reader to repair a directory that
// is fine.
func TestReviewRecord_UnreadableSnapshotNamesItAndTheWayOut(t *testing.T) {
	t.Parallel()
	dir, _ := specProject(t)
	empty := emptyRecordFile(t, dir)

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emission: %s", stderr)
	snapshot := breakSnapshot(t, dir, "snapshot-round-1.md")

	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", empty)
	require.Equal(t, 3, code, "an unreadable snapshot is a state error, not a guess at the spec")

	e := lastJSONError(t, stderr)
	assert.Equal(t, 3, e.Code)
	assert.Contains(t, e.Error, snapshot, "the message names the file it could not read")
	assert.Contains(t, e.Error, "round 1", "and which round that file belongs to")
	assert.Contains(t, e.Hint, "tp review spec.md", "the hint names the way out: re-emit the round")
	assert.NotContains(t, e.Hint, "writable",
		"not the generic state-write advice — the state directory is not what is broken")
}

// TestAuditRecord_UnreadableSnapshotNamesItAndTheWayOut is the same refusal on
// the audit side, whose snapshot is namespaced per phase.
func TestAuditRecord_UnreadableSnapshotNamesItAndTheWayOut(t *testing.T) {
	t.Parallel()
	dir, _ := specProject(t)
	empty := emptyRecordFile(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "plain.go")
	require.Equal(t, 0, code, "audit emission: %s", stderr)
	snapshot := breakSnapshot(t, dir, "snapshot-audit-round-1.md")

	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--record", empty)
	require.Equal(t, 3, code, "an unreadable snapshot is a state error, not a guess at the spec")

	e := lastJSONError(t, stderr)
	assert.Equal(t, 3, e.Code)
	assert.Contains(t, e.Error, snapshot, "the message names the file it could not read")
	assert.Contains(t, e.Error, "round 1", "and which round that file belongs to")
	assert.Contains(t, e.Hint, "tp audit spec.md", "the hint names the audit round's own re-emission")
	assert.NotContains(t, e.Hint, "writable",
		"not the generic state-write advice — the state directory is not what is broken")
}
