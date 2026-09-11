package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// reemitSpec and reemitEdit are the text an in-flight round was emitted over
// and the edit made before it was recorded. The edit adds a heading and a
// measured figure so it moves ground's floor as well as every snapshot.
const (
	reemitSpec = "# Reemit\n\n## 1. Numbers\n\nThe gate runs 4 steps.\n"
	reemitEdit = "\n## 2. Later\n\nA second unit measured 9 things.\n"
)

// reemitCase is one phase's emission: the arguments that emit a round, and the
// state files that emission writes for round 1.
type reemitCase struct {
	args  []string
	files []string
}

// stateSnapshot reads every named state file, so a refusal can be asserted to
// have left each one byte-identical rather than merely present.
func stateSnapshot(t *testing.T, dir string, names []string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(names))
	for _, name := range names {
		data, err := os.ReadFile(groundStatePath(dir, name))
		require.NoError(t, err, "the emission must have written %s", name)
		out[name] = string(data)
	}
	return out
}

// assertReemitGuard runs the whole contract for one phase.
//
// Four emissions, each deciding one clause. The second, over the UNCHANGED
// spec, is the idempotence control: it must still exit 0, or the refusal below
// would be a refusal of every re-emission rather than of a changed one. The
// third is the bug: the spec moved while round 1 was in flight, so emitting
// again would silently replace the text the round's units were graded
// against. It must exit 3, name round 1 and every file it would overwrite, and
// leave the directory byte-identical — an exit 3 after writing passes on the
// code alone. The fourth is the explicit discard: --force emits over the
// current spec and says so on stderr.
func assertReemitGuard(t *testing.T, dir string, c reemitCase) {
	t.Helper()
	_, stderr, code := runTP(t, dir, c.args...)
	require.Equal(t, 0, code, "first emission: %s", stderr)
	emitted := stateSnapshot(t, dir, c.files)
	listing := stateDirNames(t, dir)

	_, stderr, code = runTP(t, dir, c.args...)
	require.Equal(t, 0, code, "re-emitting over an unchanged spec stays idempotent: %s", stderr)
	assert.Equal(t, emitted, stateSnapshot(t, dir, c.files))

	edited := reemitSpec + reemitEdit
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(edited), 0o600))

	stdout, stderr, code := runTP(t, dir, c.args...)
	require.Equal(t, 3, code, "a re-emission over a changed in-flight round must refuse (stdout %d bytes) stderr: %s", len(stdout), stderr)
	assert.Empty(t, stdout, "a refused emission prints no prompt")
	envelope := groundErrorEnvelope(t, stderr)
	assert.Equal(t, float64(3), envelope["code"])
	msg, _ := envelope["error"].(string)
	assert.Contains(t, msg, "round 1", "the refusal names the in-flight round")
	for _, name := range c.files {
		assert.Contains(t, msg, name, "the refusal names every file it would overwrite")
	}
	assert.Contains(t, envelope["hint"], "--force", "the hint names the explicit discard")
	assert.Equal(t, emitted, stateSnapshot(t, dir, c.files), "a refused emission writes nothing")
	assert.Equal(t, listing, stateDirNames(t, dir), "and adds no file beside them")

	// Under TP_UNATTENDED the discard is the operator's: sibling role units
	// grade one emission concurrently, and a unit's --force would pull the
	// text out from under them. Exit 2, nothing written, and the hint names
	// the decision to escalate under.
	forced := append(append([]string{}, c.args...), "--force")
	_, stderr, code = runTPFence(t, dir, true, forced...)
	require.Equal(t, 2, code, "--force discarding an in-flight round is fenced under TP_UNATTENDED: %s", stderr)
	assert.Contains(t, groundErrorEnvelope(t, stderr)["hint"], "--decision discard-emission")
	assert.Equal(t, emitted, stateSnapshot(t, dir, c.files), "a fenced --force writes nothing")
	assert.Equal(t, listing, stateDirNames(t, dir))

	// Outside a run, --force is the explicit discard. runTPFence pins the
	// variable off, so an ambient TP_UNATTENDED=1 cannot flip this arm.
	_, stderr, code = runTPFence(t, dir, false, forced...)
	require.Equal(t, 0, code, "--force discards the in-flight emission: %s", stderr)
	assert.Contains(t, stderr, "discard", "--force says on stderr that it discarded round 1's emission")
	after := stateSnapshot(t, dir, c.files)
	snapshotName := c.files[len(c.files)-1]
	assert.Equal(t, edited, after[snapshotName], "the forced emission snapshots the spec as it now stands")
}

// writeReemitFixture puts reemitSpec in a fresh directory.
func writeReemitFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(reemitSpec), 0o600))
	return dir
}

func TestGroundReemitOverAChangedInFlightRoundRefuses(t *testing.T) {
	t.Parallel()
	dir := writeReemitFixture(t)
	assertReemitGuard(t, dir, reemitCase{
		args:  []string{"ground", "spec.md"},
		files: []string{"floor-ground-round-1.txt", "snapshot-ground-round-1.md"},
	})
}

// TestGroundRefusesForceOutsideTheEmission: --force discards an emission, and
// --status, --record and --units write none, so each refuses it at exit 2
// rather than accepting a discard that never happened.
func TestGroundRefusesForceOutsideTheEmission(t *testing.T) {
	t.Parallel()
	dir := writeReemitFixture(t)
	for _, mode := range [][]string{{"--status"}, {"--units"}, {"--record", "rows.ndjson"}} {
		args := append([]string{"ground", "spec.md", "--force"}, mode...)
		_, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 2, code, "%v: %s", mode, stderr)
		assert.Contains(t, stderr, "--force applies only to the emission", "%v", mode)
	}
	_, err := os.Stat(filepath.Join(dir, ".tp-review"))
	assert.True(t, os.IsNotExist(err), "a refused --force writes no state")
}

func TestReviewReemitOverAChangedInFlightRoundRefuses(t *testing.T) {
	t.Parallel()
	dir := writeReemitFixture(t)
	assertReemitGuard(t, dir, reemitCase{
		args:  []string{"review", "spec.md"},
		files: []string{"snapshot-round-1.md"},
	})
}

func TestAuditReemitOverAChangedInFlightRoundRefuses(t *testing.T) {
	t.Parallel()
	dir := writeReemitFixture(t)
	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))
	assertReemitGuard(t, dir, reemitCase{
		args:  []string{"audit", "spec.md", "--affected-files", aPath},
		files: []string{"snapshot-audit-round-1.md"},
	})
}
