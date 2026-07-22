package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResume_CompactStripsHumanFieldsKeepsData(t *testing.T) {
	dir := newResumeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("k"), 0o600))
	_, _, code := runTP(t, dir, "keep", "keep.txt", "kept reason")
	require.Equal(t, 0, code)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("s"), 0o600))

	// Full output carries the human-facing fields.
	full := resumeResult(t, dir)
	assert.NotEmpty(t, full["next_action"].(map[string]any)["summary"])
	assert.NotEmpty(t, full["kept"].([]any)[0].(map[string]any)["reason"])
	assert.NotEmpty(t, blockerByCode(full, "unexplained-changes")["message"])

	// --compact strips them but keeps every machine-actionable field.
	out, stderr, code := runTP(t, dir, "resume", "--compact")
	require.Equal(t, 0, code, "resume --compact: %s", stderr)
	var compact map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &compact))

	na := compact["next_action"].(map[string]any)
	_, hasSummary := na["summary"]
	assert.False(t, hasSummary, "compact strips next_action.summary")
	assert.NotNil(t, na["payload"], "compact keeps next_action.payload")
	assert.Contains(t, na, "command")

	kept := compact["kept"].([]any)[0].(map[string]any)
	_, hasReason := kept["reason"]
	assert.False(t, hasReason, "compact strips kept reason")
	assert.NotNil(t, kept["path"], "compact keeps kept path")

	b := blockerByCode(compact, "unexplained-changes")
	_, hasMessage := b["message"]
	assert.False(t, hasMessage, "compact strips blocker message")
	assert.NotNil(t, b["data"], "compact keeps blocker data")
	assert.Equal(t, "unexplained-changes", b["code"])
}

func TestResume_WritesNoFile(t *testing.T) {
	dir := newResumeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("s"), 0o600))

	taskFile := filepath.Join(dir, "spec.tasks.json")
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	out, _, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), "tp resume emits JSON when piped")

	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "tp resume leaves the task file byte-identical")

	for _, dirName := range []string{".tp-review", ".tp"} {
		_, statErr := os.Stat(filepath.Join(dir, dirName))
		assert.True(t, os.IsNotExist(statErr), "tp resume creates no %s", dirName)
	}
}
