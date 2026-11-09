package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestPreToolUseHookFencesOnlyTheSessionRepository: the fence guards the
// repository the session works in, so a fixture an auditor writes into a
// mktemp -d directory outside it is not tp's own state, whatever it is named.
// Matched by name alone, a *.tasks.json fixture under a temp directory was
// refused as a hand-edit of the session's task file.
func TestPreToolUseHookFencesOnlyTheSessionRepository(t *testing.T) {
	t.Parallel()
	scratch := t.TempDir()
	outside := []string{
		filepath.Join(scratch, "spec.tasks.json"),
		filepath.Join(scratch, ".tp", "config.json"),
		filepath.Join(scratch, "spec", ".tp-review", "1.0.0", "state.json"),
	}
	writers := map[string]string{"Write": "file_path", "mcp__codedbpro__create": "file"}
	for tool, key := range writers {
		for _, path := range outside {
			t.Run(tool+" "+path, func(t *testing.T) {
				t.Parallel()
				run := runPreToolUseHook(t, tool, map[string]any{key: path, "content": "{}"})
				assert.Zero(t, run.exitCode, "%s is outside the session's repository; stderr=%q", path, run.stderr)
				assert.Empty(t, run.stderr)
			})
		}
	}
}

// TestPreToolUseHookFencesTheRepositoryThroughASymlink: leaving the fence takes
// a path outside the session's repository in every spelling. A path that
// reaches the repository's own task file through a symlink elsewhere is that
// file, so it stays refused; a fence that compared spellings only would let
// it through.
func TestPreToolUseHookFencesTheRepositoryThroughASymlink(t *testing.T) {
	t.Parallel()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(repoRoot(t), link))

	path := filepath.Join(link, "spec", "0.35.0.tasks.json")
	run := runPreToolUseHook(t, "Write", map[string]any{"file_path": path, "content": "{}"})
	assert.True(t, run.denied(), "%s is the repository's task file; exit=%d stderr=%q", path, run.exitCode, run.stderr)
	assert.Contains(t, run.stderr, "hand-edited")
}
