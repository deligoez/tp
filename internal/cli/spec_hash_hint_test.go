package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// lastLine returns the final non-empty line of s. stderr may carry output.Notice
// advisories ahead of the error object, so the JSON payload is the last line.
func lastLine(t *testing.T, s string) string {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(s), "\n")
	require.NotEmpty(t, lines)
	return lines[len(lines)-1]
}

// TestSpecHashFailureNamesTheRealCause pins the post-stat half of the hint
// convention documented on specFileMissingHint: once a stat of the path has
// already succeeded, a later failure is a real I/O or permission problem, so
// the hint must name that cause rather than inheriting the code-3 default
// ("run 'tp use <file>' …"), which is TASK-file advice for a path that is
// correct. All four SpecHash call sites share the shape.
func TestSpecHashFailureNamesTheRealCause(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads regardless of mode bits")
	}

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"audit --record", []string{"audit", "spec.md", "--record", "results.ndjson"}},
		{"review --record", []string{"review", "spec.md", "--record", "results.ndjson"}},
		{"review --status", []string{"review", "spec.md", "--status"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			specPath := filepath.Join(dir, "spec.md")
			require.NoError(t, os.WriteFile(specPath, []byte("# S\n## A\n| c |\n|---|\n| x |\n"), 0o600))
			_, _, code := runTP(t, dir, "init", "spec.md")
			require.Equal(t, 0, code)

			results := filepath.Join(dir, "results.ndjson")
			require.NoError(t, os.WriteFile(results, []byte(`{"role":"r","item_id":"i","status":"PASS"}`+"\n"), 0o600))

			require.NoError(t, os.Chmod(specPath, 0o000))
			t.Cleanup(func() { _ = os.Chmod(specPath, 0o600) })

			_, stderr, code := runTP(t, dir, tc.args...)
			require.Equal(t, 3, code, "an unreadable spec is a file error")

			var payload struct {
				Error string `json:"error"`
				Hint  string `json:"hint"`
			}
			require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload))
			require.Contains(t, payload.Error, "spec.md", "the message names the path")
			require.Contains(t, payload.Hint, "permission denied", "the hint names the real cause")
			require.NotContains(t, payload.Hint, "tp use", "not the task-file default")
			require.NotContains(t, payload.Hint, "tp init", "not the task-file default")
		})
	}
}
