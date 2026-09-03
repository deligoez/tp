package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundRecordRow is a legal §7.2 row for unit u<n>: a `document` claim reached
// at `read`, the one tier §4.1 grants that kind, so the row satisfies the
// per-verdict tier rule as well as the field table. Each row's evidence differs,
// so a test can tell which rows reached the file.
func groundRecordRow(n int) string {
	return fmt.Sprintf(`{"unit_id":"u%d","anchor":"§1","text_sha":"0123456789ab","ordinal":1,`+
		`"verdict":"PASS","kind":"document","tier":"read","evidence":"read spec.md line %d"}`, n, n)
}

// writeGroundRows puts a --record payload in dir and returns the relative name
// tp is invoked with. The payload is newline-terminated, which is the shape a
// unit's writer produces and the one §7.1 says must not be read as a partial
// trailing line.
func writeGroundRows(t *testing.T, dir string, lines ...string) string {
	t.Helper()
	body := ""
	for _, line := range lines {
		body += line + "\n"
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "rows.ndjson"), []byte(body), 0o600))
	return "rows.ndjson"
}

// groundErrorEnvelope decodes the {error, code, hint} object tp writes to stderr
// under --json, so a test's verdict rests on the envelope's own fields rather
// than on a substring of a sentence.
func groundErrorEnvelope(t *testing.T, stderr string) map[string]any {
	t.Helper()
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &envelope),
		"stderr is one JSON error envelope under --json: %q", stderr)
	return envelope
}

// groundStatePath names a file inside the fixture's state directory.
func groundStatePath(dir, name string) string {
	return filepath.Join(dir, ".tp-review", "spec", name)
}

// TestGroundRecordWritesTheRoundFileBesideTheEmission is the acceptance's third
// clause: --record writes ground-round-N.ndjson.
//
// The listing is asserted as a SET, for the reason the emit test gives: the
// negative half of §11 row 12 — no state.json — is invisible to a pair of Stat
// calls, and the round file must land BESIDE the two artifacts emit wrote
// rather than in place of them.
func TestGroundRecordWritesTheRoundFileBesideTheEmission(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	rows := writeGroundRows(t, dir, groundRecordRow(1), groundRecordRow(2))

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)

	assert.Equal(t, []string{
		"floor-ground-round-1.txt",
		"ground-round-1.ndjson",
		"snapshot-ground-round-1.md",
	}, stateDirNames(t, dir), "--record adds the round file beside the emission and writes no state.json")

	payload, err := os.ReadFile(filepath.Join(dir, rows))
	require.NoError(t, err)
	written, err := os.ReadFile(groundStatePath(dir, "ground-round-1.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, string(payload), string(written),
		"the recorded round holds the payload's bytes, not a re-serialisation of the parsed rows (§7.1)")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(2), out["rows"])
	assert.Equal(t, filepath.Join(".tp-review", "spec", "ground-round-1.ndjson"), out["file"])
	assert.Equal(t, filepath.Join(".tp-review", "spec", "floor-ground-round-1.txt"), out["floor"],
		"the result names the floor the round was recorded against")
}

// TestGroundRecordReadsTheEmittedFloorAndNotTheCurrentSpec is the acceptance's
// first clause, asserted in the two directions that separate reading the
// recorded floor from re-deriving one.
//
// Neither half is decidable from the other. A re-deriving implementation
// SUCCEEDS on the first — the spec is still there, so it can build a floor for a
// round whose emission is gone — and FAILS on the second, where the artifact it
// would derive from no longer exists. Together they pin that the floor on disk
// is the input and the spec at record time is not one.
func TestGroundRecordReadsTheEmittedFloorAndNotTheCurrentSpec(t *testing.T) {
	t.Run("the emitted floor is gone", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NoError(t, os.Remove(groundStatePath(dir, "floor-ground-round-1.txt")))
		require.FileExists(t, groundStatePath(dir, "snapshot-ground-round-1.md"),
			"only the floor is removed, so a refusal here is about the floor and not about the directory")
		require.FileExists(t, filepath.Join(dir, "spec.md"),
			"the spec remains, which is what a re-deriving implementation would floor from")

		rows := writeGroundRows(t, dir, groundRecordRow(1))
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)

		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(3), envelope["code"])
		assert.Contains(t, fmt.Sprint(envelope["error"], " ", envelope["hint"]), "tp ground spec.md",
			"the refusal names the emit command (§7.3)")
	})

	t.Run("the spec is gone", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NoError(t, os.Remove(filepath.Join(dir, "spec.md")),
			"nothing --record needs is in the spec: the floor and the snapshot are the round's record of it")

		rows := writeGroundRows(t, dir, groundRecordRow(1))
		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "stdout: %s stderr: %s", stdout, stderr)
		require.FileExists(t, groundStatePath(dir, "ground-round-1.ndjson"),
			"the round records against the floor the emission froze")
	})
}

