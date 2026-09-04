package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLintSpecNotFoundHint: `tp lint nope.md` landed on a hintless exit-3 site,
// so it inherited the code-3 default hint — TASK-file advice ("run 'tp use
// <file>' … 'tp init <spec>'") for a command that takes no task file and never
// reads one on this path (§9.3). It now passes specFileMissingHint, the same
// hint every pre-stat spec-path site across tp review and tp audit shares.
func TestLintSpecNotFoundHint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	missing := filepath.Join(dir, "nope.md")

	_, stderr, code := runTP(t, dir, "lint", missing)
	require.Equal(t, 3, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	message, _ := payload["error"].(string)
	assert.Contains(t, message, missing, "the message names the path tp could not read")
	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "spec path", "the hint names the spec path the caller typed")
	assert.Contains(t, hint, "not the task file", "the hint separates the spec from the task file")
	assert.NotContains(t, hint, "tp use", "task-file advice is the wrong object for a spec-path typo")
	assert.NotContains(t, hint, "tp init", "task-file advice is the wrong object for a spec-path typo")
}
