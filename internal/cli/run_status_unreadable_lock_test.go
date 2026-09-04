package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §3.5 makes the run-lock probe the only thing separating `in_flight` from
// `crashed`. A probe that cannot be performed is not evidence for either, and
// the two wrong answers are not symmetric: reporting a live run as `crashed`
// invites a second driver over a cycle that already has one, while reporting a
// dead run as `in_flight` costs a look. So an unreadable lock must not be read
// as an absent one.
//
// This is the sibling of the `fileExists` repair in validate_project.go — the
// same conflation of "no such file" with "cannot tell", reached at a different
// sink. The lock directory is made unreadable rather than the lock file itself,
// because stat consults the directory's permissions, not the file's.
func TestRunStatus_UnreadableLockIsNotReportedAsCrashed(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root can stat inside a 0000 directory, so the probe cannot be made to fail")
	}
	dir := runProject(t)
	writeRunState(t, dir, crashedRunState)

	locks := filepath.Join(dir, ".tp", "locks")
	require.NoError(t, os.MkdirAll(locks, 0o755))
	require.NoError(t, os.Chmod(locks, 0o000))
	t.Cleanup(func() { _ = os.Chmod(locks, 0o755) })

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "an unreadable lock is reported, not fatal: %s", stderr)

	assert.Equal(t, "in_flight", decodeStatus(t, stdout)["run_state"],
		"a probe that could not be performed is not evidence the driver died")
	assert.Contains(t, stderr, "run lock",
		"and the operator is told the classification rests on a failed probe")
}
