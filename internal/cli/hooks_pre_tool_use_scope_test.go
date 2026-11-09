package cli

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPreToolUseHookAllowsReadsOfTpState: the fence stops hand-edits, so a
// call that only reads tp's own state is not one, whichever route brings it
// to the hook. A reader that names a round file, a task file or the project
// config passes; judged by its path alone it was refused with the hand-edit
// message, and an agent reading a round snapshot had to fall back to a shell.
func TestPreToolUseHookAllowsReadsOfTpState(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	fenced := []string{
		"spec/.tp-review/0.35.0/round-3/merged.ndjson",
		filepath.Join(root, "spec", ".tp-review", "0.35.0", "state.json"),
		"spec/0.35.0.tasks.json",
		".tp/config.json",
	}
	readers := map[string]string{
		"Read":                          "file_path",
		"Grep":                          "path",
		"Glob":                          "path",
		"mcp__codedbpro__read":          "file",
		"mcp__codedbpro__faster_search": "path",
		"mcp__codedbpro__meta_search":   "path",
		"mcp__codedbpro__diff":          "file",
		"mcp__codedbpro__lint":          "file",
	}
	for tool, key := range readers {
		for _, path := range fenced {
			t.Run(tool+" "+path, func(t *testing.T) {
				t.Parallel()
				run := runPreToolUseHook(t, tool, map[string]any{key: path})
				assert.Zero(t, run.exitCode, "a read is not a hand-edit; stderr=%q", run.stderr)
				assert.Empty(t, run.stderr)
			})
		}
	}
}
