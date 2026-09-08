package cli

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMergeWriteFailure_NamesTheTemporaryOnlyWhenItSurvives covers the one
// outcome writeMergeOutput cannot clean up after: the rename fails AND the
// removal of the temporary fails too.
//
// Field report: with the temporary locked mid-write, `rename` failed EPERM and
// `os.Remove` failed EPERM as well. The removal error was discarded, so tp
// exited 3 reporting only the rename, and a 135 MB file called
// `out.ndjson.tp-merge-…` was left beside `-o` — a name the operator never
// chose, named nowhere in the message, and cleaned up by nothing.
//
// The EPERM pair is not reachable from a portable test (it needs the temporary
// to be made immutable in the window between create and rename), so what is
// tested is the composition itself: given a removal error, the message names
// the file left behind; given none, the message is the write error unchanged.
// The write error here deliberately does NOT contain the temporary path, or
// both cases would satisfy the same assertion and the test would pin nothing.
func TestMergeWriteFailure_NamesTheTemporaryOnlyWhenItSurvives(t *testing.T) {
	t.Parallel()
	const tmp = "/round/out.ndjson.tp-merge-1877366751"
	writeErr := errors.New("operation not permitted")
	require.NotContains(t, writeErr.Error(), tmp,
		"the write error must not name the temporary, or neither assertion below discriminates")

	t.Run("the temporary was removed: the write error stands alone", func(t *testing.T) {
		t.Parallel()
		got := mergeWriteFailure(writeErr, nil, tmp)
		assert.Equal(t, writeErr.Error(), got.Error(),
			"nothing is left behind, so there is nothing to add")
	})

	t.Run("the temporary survived: the message names it", func(t *testing.T) {
		t.Parallel()
		got := mergeWriteFailure(writeErr, errors.New("operation not permitted"), tmp)
		assert.Contains(t, got.Error(), tmp,
			"the operator cannot delete a file tp never names")
		assert.ErrorIs(t, got, writeErr,
			"the cause of the failed write survives the wrapping")
	})
}
