package cli_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExportList_ContentHashMovesWithTheSpecAndTaskText is §13's Read view
// on tp export --list: an edit to the spec's text, or to a task's acceptance
// or description, changes nothing else in the file's row once the review
// reads stale, so the row carries a content hash over the task file and its
// spec, and each such edit moves it while an unchanged repository keeps it.
func TestExportList_ContentHashMovesWithTheSpecAndTaskText(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	spec := filepath.Join(dir, "spec.md")
	writeSpec := func(text string) {
		require.NoError(t, os.WriteFile(spec, []byte("# S\n\n## 1. X\n\n"+text+"\n\n## 2. Y\n\ny.\n"), 0o600))
	}
	row := func() exportListRow {
		_, rows := exportList(t, dir, false)
		require.Len(t, rows, 1)
		return rows[0]
	}
	// The first edit makes the recorded review stale; the edits after it
	// change no field of the row but the hash.
	writeSpec("x edited once.")
	before := row()
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{64}$`), before.ContentHash)
	assert.Equal(t, before.ContentHash, row().ContentHash, "an unchanged repository keeps the hash")

	writeSpec("x edited twice.")
	afterSpec := row()
	assert.NotEqual(t, before.ContentHash, afterSpec.ContentHash, "a spec text edit moves the hash")
	same := afterSpec
	same.ContentHash = before.ContentHash
	assert.Equal(t, before, same, "the spec text edit changes nothing else in the row")

	for _, set := range []string{"acceptance=x done well", "description=the only task"} {
		prev := row()
		_, stderr, code := runTP(t, dir, "set", "t1", set)
		require.Equal(t, 0, code, stderr)
		next := row()
		assert.NotEqual(t, prev.ContentHash, next.ContentHash, "tp set t1 %s moves the hash", set)
		next.ContentHash = prev.ContentHash
		assert.Equal(t, prev, next, "tp set t1 %s changes nothing else in the row", set)
	}
}
