package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAddWithSeveralTaskFilesNamesTheCandidates is the BUGS.md reproduction:
// with a.tasks.json and b.tasks.json and no active pointer, `tp add` said "no
// task file found. Use --spec to create one", and following that advice failed
// with "task file already exists". It must report the discovery error the read
// commands give — the candidates and how to pick one — and no --spec advice.
func TestAddWithSeveralTaskFilesNamesTheCandidates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "a.md")
	writeTargetSpec(t, dir, "b.md")
	for _, spec := range []string{"a.md", "b.md"} {
		_, stderr, code := runTP(t, dir, "init", spec)
		require.Equal(t, 0, code, stderr)
	}

	_, statusErr, statusCode := runTP(t, dir, "status")
	require.Equal(t, 3, statusCode, statusErr)

	_, stderr, code := runTP(t, dir, "add", writeTargetTask("a1"))
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "multiple task files", "add must report the discovery error")
	assert.Contains(t, stderr, "a.tasks.json", "the error names the first candidate")
	assert.Contains(t, stderr, "b.tasks.json", "the error names the second candidate")
	assert.Contains(t, stderr, "--file", "the error names how to pick one")
	assert.Contains(t, stderr, "TP_FILE", "the error names how to pick one")
	assert.NotContains(t, stderr, "--spec", "--spec cannot help when task files exist")
	assert.Equal(t, statusErr, stderr, "add reports the same discovery error as the read commands")
}

// TestAddWithNoTaskFileKeepsTheSpecAdvice pins the other side: where no task
// file exists at all, --spec is the way to create one, so the advice stays.
func TestAddWithNoTaskFileKeepsTheSpecAdvice(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTargetSpec(t, dir, "a.md")

	_, stderr, code := runTP(t, dir, "add", writeTargetTask("a1"))
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "no task file found. Use --spec to create one, or run tp init first")
	assert.NotContains(t, stderr, "multiple task files")
}
