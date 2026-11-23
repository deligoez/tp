package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWaiting_SetOnWipClearedByClose: waiting is said of a claim in progress
// or, on an open task, of a hold; a claim ends the hold, and a close clears
// it.
func TestWaiting_SetOnWipClearedByClose(t *testing.T) {
	t.Parallel()
	dir := initEntryProject(t)
	_, stderr, code := runTP(t, dir, "add", validEntryTask)
	require.Equal(t, 0, code, stderr)

	_, stderr, code = runTP(t, dir, "set", "t1", "waiting=a background job")
	assert.Equal(t, 0, code, "waiting on an open task holds it: %s", stderr)

	_, stderr, code = runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "set", "t1", "waiting=a background job")
	require.Equal(t, 0, code, stderr)
	assert.Contains(t, string(readRawTaskFile(t, dir)), `"waiting": "a background job"`)

	_, stderr, code = runTP(t, dir, "close", "t1", "implemented and verified against the acceptance")
	require.Equal(t, 0, code, stderr)
	assert.NotContains(t, string(readRawTaskFile(t, dir)), `"waiting"`)
}
