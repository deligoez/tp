package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// §11 row 14: each of 0/1/2/3/4 is produced by one of §7.1's named inputs, one
// test per code. The mapping is read from exitStateError
// (internal/cli/review_record.go:342) rather than invented, so the two codes it
// separates — 3 for corrupt state, 4 for a write lock it could not take — are
// asserted on inputs that reach it by different routes.
//
// §7.1 names two inputs for exit 2 and both are asserted, in different files:
// `--record` with no path argument below, and `--check` without `--status` in
// TestCheckWithoutStatusIsAUsageErrorByTheRuleAndNotAnUnknownFlag
// (ground_check_test.go). The second lives there because exit 2 alone cannot
// decide it — cobra's unknown-flag path returns the same code — so its verdict
// rests on `--status --check` exiting 0 first, which is that file's subject.

// TestGroundExitsZeroOnAnInvocationThatCompletes is §7.1's exit-0 row: any
// invocation in the table completing. Both of the two that exist are run, since
// a record cannot complete without the emission that froze its floor.
func TestGroundExitsZeroOnAnInvocationThatCompletes(t *testing.T) {
	dir := writeGroundFixture(t)

	_, stderr, code := runTP(t, dir, "ground", "spec.md")
	require.Equal(t, 0, code, "an emission completes: %s", stderr)

	rows := writeGroundRows(t, dir, groundRecordRow(1))
	_, stderr, code = runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "a record of a valid payload completes: %s", stderr)
	require.FileExists(t, groundStatePath(dir, "ground-round-1.ndjson"))
}

// TestGroundExitsOneOnARecordItWillNotValidate is §7.1's exit-1 row over the two
// inputs row 14 names for it: a file whose second line is `{`, and an empty
// file. They fail for different reasons — one row that is not JSON, and a
// payload holding no rows at all — and tp tells them apart by the error's type
// rather than by its wording, so both are asserted here.
func TestGroundExitsOneOnARecordItWillNotValidate(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"a second line that is not JSON", groundRecordRow(1) + "\n{\n"},
		{"an empty file", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeGroundFixture(t)
			groundEmit(t, dir)
			before := stateDirNames(t, dir)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "rows.ndjson"), []byte(tc.body), 0o600))

			stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", "rows.ndjson")
			require.Equal(t, 1, code, "stdout: %s stderr: %s", stdout, stderr)
			assert.Equal(t, float64(1), groundErrorEnvelope(t, stderr)["code"])
			assert.Equal(t, before, stateDirNames(t, dir), "a refused round writes no round file")
		})
	}
}

// TestGroundExitsTwoOnUsage is §7.1's exit-2 row on the one input of its two
// that is reachable today: `--record` with no path argument.
func TestGroundExitsTwoOnUsage(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record")
	require.Equal(t, 2, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Equal(t, float64(2), groundErrorEnvelope(t, stderr)["code"])
}

// TestGroundExitsThreeOnFileOrCorruptState is §7.1's exit-3 row over both of
// row 14's named inputs, plus the control the row's own caveat demands.
//
// The two inputs reach exit 3 by different routes: a missing --record path is a
// file tp could not open, while an unparseable state.json is what exitStateError
// returns for a StateCorruptError. Each is emitted FIRST, so the round has a
// floor and the refusal is about the input under test rather than about §7.3's
// no-prior-emit rule, which exits 3 too.
//
// The third subtest is the caveat: **not** a ground-only directory. A directory
// holding a ground round and its emission and nothing else must load cleanly
// (§11 row 11), so an implementation that read the state directory's mere
// existence as corruption — refusing every record on a spec that has never been
// reviewed — is what this control fails.
func TestGroundExitsThreeOnFileOrCorruptState(t *testing.T) {
	t.Run("a --record path that does not exist", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NoFileExists(t, filepath.Join(dir, "absent.ndjson"))

		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", "absent.ndjson")
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(3), envelope["code"])
		assert.Contains(t, fmt.Sprint(envelope["error"]), "absent.ndjson",
			"the refusal names the file tp could not open, not the floor it already read")
	})

	t.Run("an unparseable state.json", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NoError(t, os.WriteFile(groundStatePath(dir, "state.json"), []byte("{not json"), 0o600))
		before := stateDirNames(t, dir)
		rows := writeGroundRows(t, dir, groundRecordRow(1))

		stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
		envelope := groundErrorEnvelope(t, stderr)
		assert.Equal(t, float64(3), envelope["code"],
			"a corrupt state directory is exitStateError's code 3, never its code 4")
		assert.Contains(t, fmt.Sprint(envelope["error"]), "state.json",
			"the refusal names the file that is unreadable")
		assert.Equal(t, before, stateDirNames(t, dir),
			"tp adds no round to a state directory it cannot read")
	})

	t.Run("a ground-only directory is not corrupt", func(t *testing.T) {
		dir := writeGroundFixture(t)
		groundEmit(t, dir)
		require.NotContains(t, stateDirNames(t, dir), "state.json",
			"the emission wrote no index, which is the condition this control is about")
		rows := writeGroundRows(t, dir, groundRecordRow(1))

		_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
		require.Equal(t, 0, code, "a ground-only directory loads cleanly (§11 row 11): %s", stderr)
	})
}

// groundLockFixture is writeGroundFixture with the two things a contention test
// needs: a symlink-resolved directory, so this process and the tp subprocess
// hash the same absolute path into the same lock file (macOS maps /var to
// /private/var), and a 1s lock_timeout_seconds, so the assertion costs a second
// rather than the built-in five.
func groundLockFixture(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(groundFixtureSpec), 0o600))

	_, stderr, code := runTP(t, dir, "set", "--workflow", "--project", "lock_timeout_seconds=1")
	require.Equal(t, 0, code, "set lock_timeout_seconds: %s", stderr)
	return dir
}

// TestGroundExitsFourWhenTheWriteLockIsHeld is §7.1's exit-4 row: a concurrent
// --record holding .tp/locks/<target>.lock.
//
// The lock this test holds is named by the one path the command itself is
// invoked with — the spec — and NOT by calling whatever wrapper ground calls: a
// shared wrapper would move both sides at once, so a --record that locked some
// other target would still contend with it and the test would stay green. Named
// independently, that mutant finds no contention here and exits 0. Before this
// task nothing in the ground flow locked anything and exit 4 was unreachable.
func TestGroundExitsFourWhenTheWriteLockIsHeld(t *testing.T) {
	dir := groundLockFixture(t)
	groundEmit(t, dir)
	rows := writeGroundRows(t, dir, groundRecordRow(1))
	before := stateDirNames(t, dir)

	acquired := make(chan struct{})
	release := make(chan struct{})
	held := make(chan struct{})
	go func() {
		defer close(held)
		_ = engine.WithFileLock(filepath.Join(dir, "spec.md"), func() error {
			close(acquired)
			<-release
			return nil
		})
	}()
	<-acquired
	defer func() {
		close(release)
		<-held
	}()

	start := time.Now()
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	elapsed := time.Since(start)
	require.Equal(t, 4, code, "stdout: %s stderr: %s", stdout, stderr)

	// The window pins that the timeout resolved: the project config says 1s,
	// so the record retried for about that long — it neither gave up on the
	// first failed TryLock nor fell back to the 5s built-in default.
	assert.GreaterOrEqual(t, elapsed, 900*time.Millisecond, "the record retried for the configured 1s, not less")
	assert.Less(t, elapsed, 4*time.Second, "the record used the configured 1s, not the 5s built-in default")

	assertLockTimeoutErrorObject(t, stderr)
	assert.Equal(t, before, stateDirNames(t, dir), "a record that never took the lock writes nothing")
}
