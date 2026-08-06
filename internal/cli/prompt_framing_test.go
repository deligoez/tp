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

// TestUnreadableFileIsNeverToldComplete guards §10.7's honesty clause: when a
// listed source file cannot be read, NO role may be told the carried contents
// are complete and authoritative. fileSetRead used to drop every os.ReadFile
// error and return only a string, so both call sites set filesComplete
// unconditionally — a role received a body-less "complete" section for a path
// that was still listed as a file_check item, and reported on a file it never
// saw. Both phases are checked: the review path and the audit path each own a
// copy of the inliner decision.
func TestUnreadableFileIsNeverToldComplete(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0o000 file, so the read never fails")
	}
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n| Field | Type |\n|------|------|\n| id | int |\n"), 0o600))

	// stat succeeds on a 0o000 file (it reads the directory entry) while the
	// read fails — exactly the gap the framing used to paper over, and the
	// reason the fix cannot rely on the upstream --affected-files stat check.
	codePath := filepath.Join(dir, "code.go")
	require.NoError(t, os.WriteFile(codePath, []byte("package main\nfunc Foo() int { return 42 }\n"), 0o600))
	require.NoError(t, os.Chmod(codePath, 0o000))
	t.Cleanup(func() { _ = os.Chmod(codePath, 0o600) })

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"review", []string{"review", specPath, "--no-state", "--affected-files", codePath}},
		{"audit", []string{"audit", specPath, "--affected-files", codePath}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, exit := runTP(t, dir, tc.args...)
			require.Equal(t, 0, exit, "stderr: %s", stderr)
			var result map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &result))
			prompts := result["prompts"].([]any)
			require.NotEmpty(t, prompts)
			named := 0
			for _, p := range prompts {
				pm := p.(map[string]any)
				body := pm["prompt"].(string)
				assert.NotContains(t, body, "the source file contents carried in this prompt are complete",
					"role %s must not be told an unreadable file's contents are complete", pm["role"])
				if strings.Contains(body, "read these files yourself") {
					named++
					assert.Contains(t, body, "- "+codePath, "the unread path is named for the role to read")
				}
			}
			assert.Positive(t, named, "at least one role is told to read the unread path itself")
		})
	}
}
