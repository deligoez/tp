package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInitAlreadyExists_HintDoesNotRecommendInit guards a shape worth naming:
// a hint that recommends the command that just refused, for the reason it
// refused. An agent following it loops.
//
// `tp init` on an existing task file used to take the exit-3 default, which
// ends "or 'tp init <spec>' to create one". The hint enumeration cannot catch
// this — it exempts init.go wholesale as a task-file command, which is right
// for most of what that file reports and wrong for exactly this one.
func TestInitAlreadyExists_HintDoesNotRecommendInit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\ncontent\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "first init: %s", stderr)

	_, stderr, code = runTP(t, dir, "init", "spec.md")
	require.Equal(t, 3, code, "a second init must refuse")

	var obj map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &obj), "stderr must be one JSON object: %s", stderr)

	hint, _ := obj["hint"].(string)
	require.NotEmpty(t, hint, "the refusal must carry a hint")
	assert.NotContains(t, hint, "tp init",
		"the hint must not name the command that just refused for this exact reason")
	assert.True(t, strings.Contains(hint, "tp use") || strings.Contains(hint, "tp status"),
		"the hint should point at the file that already exists, got %q", hint)
}
