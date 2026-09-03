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

