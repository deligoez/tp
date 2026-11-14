package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_UnparseableTaskFileExits3: a task file that does not parse made
// tp config exit 0 with the project and default values, as if the file had no
// workflow override. It must exit 3 with the parse error, as tp list does.
func TestConfig_UnparseableTaskFileExits3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	full := `{"spec":"s.md","tasks":[],"workflow":{"review_max_rounds":7}}`
	// Truncated mid-object: the workflow override is unreadable.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"), []byte(full[:len(full)-12]), 0o600))

	_, listErr, listCode := runTP(t, dir, "--file", "s.tasks.json", "list")
	require.Equal(t, 3, listCode, listErr)

	out, stderr, code := runTP(t, dir, "--file", "s.tasks.json", "config")
	assert.Equal(t, 3, code, "stdout: %s stderr: %s", out, stderr)
	assert.Contains(t, stderr, "parse task file", "the error names the parse failure")
	assert.Contains(t, stderr, "unexpected end of JSON input")
	assert.Empty(t, out, "no workflow is reported for a task file that was not read")
}

// TestConfig_QuietSilencesConfigWarnings: tp config printed .tp/config.json
// warnings under --quiet with a raw stderr write, where tp validate --project
// --quiet prints nothing.
func TestConfig_QuietSilencesConfigWarnings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
		[]byte(`{"workflow":{"review_max_rounds":"seven"}}`), 0o600))

	_, loudErr, loudCode := runTP(t, dir, "config")
	require.Equal(t, 0, loudCode, loudErr)
	require.Contains(t, loudErr, "review_max_rounds", "the fixture must raise a config warning")

	_, stderr, code := runTP(t, dir, "config", "--quiet")
	assert.Equal(t, 0, code, stderr)
	assert.Empty(t, stderr, "--quiet silences config warnings")
}
