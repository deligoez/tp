package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicReviewLoopStatus_AnUnparseableRoundFileNamesItsLine: the error names
// the file, the line that does not parse and why, not only that it failed.
func TestEpicReviewLoopStatus_AnUnparseableRoundFileNamesItsLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	content := `{"severity":"low","finding":"ok"}` + "\n" + `{"severity":` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, epicRoundFile(1)), []byte(content), 0o600))
	_, err := EpicReviewLoopStatus(dir, nil, EpicReviewPolicy{CleanRounds: 2})
	require.Error(t, err)
	assert.Contains(t, err.Error(), epicRoundFile(1))
	assert.Contains(t, err.Error(), "line 2")
	assert.Contains(t, err.Error(), "unexpected end of JSON input")
}

// TestReadEpicRoundRows_AMissingFileKeepsTheReadError: a file that cannot be
// read says so, with the read error wrapped.
func TestReadEpicRoundRows_AMissingFileKeepsTheReadError(t *testing.T) {
	t.Parallel()
	_, err := readEpicRoundRows(filepath.Join(t.TempDir(), "round-9.ndjson"))
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}
