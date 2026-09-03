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
// §7.1 names two inputs for exit 2 and only one of them exists yet: `--record`
// with no path argument is asserted below, while `--check` without `--status`
// cannot be, because neither flag is registered until the status tasks land.
// `tp ground <spec> --check` today exits 2 as an UNKNOWN FLAG, so a test written
// now would pass without ever reaching the rule it claims to pin.

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

