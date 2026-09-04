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

// TestEmittedPromptsCarryFraming verifies the tp-owned framing wrapped around
// every emitted review/audit prompt (§10.4–§10.7): the output_path field and
// the prompt text naming it, the reset discipline, the loop budget, and the
// file-reading statement. For audit it also checks the per-role inliner (§10.7):
// the first role whose files fit under the 12 KB budget inlines complete
// contents, and every later role gets named paths only.
func TestEmittedPromptsCarryFraming(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n| Field | Type |\n|------|------|\n| id | int |\n"), 0o600))
	codePath := filepath.Join(dir, "code.go")
	require.NoError(t, os.WriteFile(codePath, []byte("package main\nfunc Foo() int { return 42 }\n"), 0o600))

	t.Run("review prompts carry framing", func(t *testing.T) {
		stdout, _, exit := runTP(t, dir, "review", specPath, "--no-state")
		require.Equal(t, 0, exit)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		prompts := result["prompts"].([]any)
		require.NotEmpty(t, prompts)
		for _, p := range prompts {
			pm := p.(map[string]any)
			role := pm["role"].(string)
			outputPath := pm["output_path"].(string)
			assert.Equal(t, "review-r1-"+role+".ndjson", outputPath, "output_path is review-r<N>-<role>.ndjson")
			body := pm["prompt"].(string)
			assert.Contains(t, body, "Write this round's findings to: "+outputPath, "§10.4 text names the file")
			assert.Contains(t, body, "Produce findings for this round only, write them to that file, then stop.", "§10.5 reset discipline")
			assert.Contains(t, body, "Loop budget: review round 1", "§10.6 round number")
			assert.Contains(t, body, "consecutive clean round(s) required", "§10.6 required clean count")
			assert.Contains(t, body, "File reading:", "§10.7 file-reading statement")
		}
	})

	t.Run("audit prompts carry framing and per-role inliner", func(t *testing.T) {
		stdout, _, exit := runTP(t, dir, "audit", specPath, "--affected-files", codePath)
		require.Equal(t, 0, exit)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		prompts := result["prompts"].([]any)
		require.NotEmpty(t, prompts)
		inliners := 0
		for _, p := range prompts {
			pm := p.(map[string]any)
			role := pm["role"].(string)
			outputPath := pm["output_path"].(string)
			assert.Equal(t, "audit-r1-"+role+".ndjson", outputPath, "output_path is audit-r<N>-<role>.ndjson")
			body := pm["prompt"].(string)
			assert.Contains(t, body, "Write this round's findings to: "+outputPath, "§10.4 text names the file")
			assert.Contains(t, body, "Produce findings for this round only, write them to that file, then stop.", "§10.5 reset discipline")
			assert.Contains(t, body, "Loop budget: audit round 1", "§10.6 round number")
			assert.Contains(t, body, "File reading:", "§10.7 file-reading statement")
			switch {
			case strings.Contains(body, "the source file contents carried in this prompt are complete"):
				inliners++
				assert.Contains(t, body, "func Foo()", "inliner carries the file body")
			case strings.Contains(body, "does NOT inline"):
				assert.Contains(t, body, "read these files yourself", "non-inliner must read")
				assert.Contains(t, body, "- "+codePath, "non-inliner names the file")
			default:
				t.Errorf("role %s prompt states neither complete nor must-read", role)
			}
		}
		assert.Equal(t, 1, inliners, "§10.7: exactly one role inlines contents under the budget")
	})
}

// TestUnreadableFileIsNeverToldComplete guards §10.7's honesty clause AND its
// price. fileSetRead used to drop every os.ReadFile error, so a role received a
// body-less "complete" section for a file it never saw. The first repair went
// too far the other way: one unreadable path made both call sites discard the
// ENTIRE affected-files section, so a readable file's body was withheld from
// every role and each was sent back to disk for the whole set. The invariant is
// honesty at the lowest cost that preserves it — inline what tp could read,
// under the "(incomplete)" header, and name only what it could not. Both phases
// are checked: the review path and the audit path each own a copy of the
// inliner decision.
func TestUnreadableFileIsNeverToldComplete(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the read never fails")
	}
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n| Field | Type |\n|------|------|\n| id | int |\n"), 0o600))

	readablePath := filepath.Join(dir, "readable.go")
	require.NoError(t, os.WriteFile(readablePath, []byte("package main\nfunc Bar() int { return 7 }\n"), 0o600))

	// stat succeeds on a 0o000 file (it reads the directory entry) while the
	// read fails — exactly the gap the framing used to paper over, and the
	// reason the fix cannot rely on the upstream --affected-files stat check.
	lockedPath := filepath.Join(dir, "locked.go")
	require.NoError(t, os.WriteFile(lockedPath, []byte("package main\nfunc Foo() int { return 42 }\n"), 0o600))
	require.NoError(t, os.Chmod(lockedPath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(lockedPath, 0o600) })

	const mustRead = "read these files yourself before judging them:"

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"review", []string{"review", specPath, "--no-state", "--affected-files", readablePath, "--affected-files", lockedPath}},
		{"audit", []string{"audit", specPath, "--affected-files", readablePath, "--affected-files", lockedPath}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exit := runTP(t, dir, tc.args...)
			require.Equal(t, 0, exit, "stderr: %s", stderr)
			var result map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &result))
			prompts := result["prompts"].([]any)
			require.NotEmpty(t, prompts)
			partial, named := 0, 0
			for _, p := range prompts {
				pm := p.(map[string]any)
				body := pm["prompt"].(string)
				assert.NotContains(t, body, "the source file contents carried in this prompt are complete",
					"role %s must not be told an unreadable file's contents are complete", pm["role"])
				if strings.Contains(body, "## Affected Files (incomplete)") {
					partial++
					assert.Contains(t, body, "func Bar()",
						"the readable file's body is still inlined — one locked file costs only that file")
					assert.Contains(t, body, "are INCOMPLETE",
						"role %s is told the carried contents are incomplete", pm["role"])
				}
				if i := strings.Index(body, mustRead); i >= 0 {
					named++
					tail := body[i:]
					assert.Contains(t, tail, "  - "+lockedPath, "the unread path is named for the role to read")
					if partial > 0 && strings.Contains(body, "## Affected Files (incomplete)") {
						assert.NotContains(t, tail, "  - "+readablePath,
							"a file tp inlined is not also handed back to the role to read")
					}
				}
			}
			assert.Equal(t, 1, partial, "§10.7: exactly one role inlines, and its section is marked incomplete")
			assert.Positive(t, named, "at least one role is told to read the unread path itself")
		})
	}
}
