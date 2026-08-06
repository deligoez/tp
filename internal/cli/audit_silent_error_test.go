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

// A corrupt (existing but unparseable) spec.tasks.json must not be silently
// treated as "no tasks": the run warns on stderr naming the task file, then
// proceeds (exit 0, best-effort checklist construction) — consistent with the
// scanner-warning paths. ONE advisory, not one per reader: the file feeds both
// the task-acceptance checklist and the per-file task mapping, and two
// differently worded lines for one unreadable file read as two problems.
func TestAuditCorruptTaskFileWarns(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))

	// Adjacent task file that exists but is not valid JSON.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte("not valid json {{{"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code, "corrupt task file is a warning, not a hard error: %s", stderr)
	assert.Contains(t, stderr, "warning: cannot read task file")
	assert.Contains(t, stderr, "spec.tasks.json", "warning must name the task file")
	assert.Equal(t, 1, strings.Count(stderr, "warning: cannot read task file"),
		"one unreadable task file is one condition, so it costs one advisory: %q", stderr)
}

// A real (non-absent) read error on the task file — here a directory at the
// tasks path, which os.ReadFile cannot read as a regular file — must warn
// rather than being silently swallowed as "absent = optional".
func TestAuditUnreadableTaskFileWarns(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))

	// A directory where the task file is expected: exists, but unreadable.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "spec.tasks.json"), 0o755))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath)
	require.Equal(t, 0, code, "unreadable task file is a warning, not a hard error: %s", stderr)
	assert.Contains(t, stderr, "warning: cannot read task file")
	assert.Contains(t, stderr, "is a directory", "the advisory carries the underlying read error")
}

// A malformed (invalid JSON) line in a findings file is skipped with a stderr
// warning naming the file, while valid lines are still picked up — representative
// of the sweep that surfaces every silently-dropped parse error.
func TestAuditFindingsMalformedLineWarns(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n## Table\n| Col |\n|-----|\n| a |\n"), 0o600))

	aPath := filepath.Join(dir, "a.go")
	require.NoError(t, os.WriteFile(aPath, []byte("package main\n"), 0o600))

	// One valid finding line followed by a malformed line.
	findingsPath := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findingsPath,
		[]byte("{\"finding\":\"valid finding text\",\"location\":\"audit.go:1\"}\nthis is not valid json\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-files", aPath, "--findings", findingsPath)
	require.Equal(t, 0, code, "malformed findings line is a warning, not a hard error: %s", stderr)
	assert.Contains(t, stderr, "warning: skipping malformed line (invalid JSON)")
	assert.Contains(t, stderr, findingsPath, "warning must name the findings file")

	// The valid line survived into the checklist (only the malformed one was skipped).
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	count := 0
	for _, e := range result["checklist"].([]any) {
		if e.(map[string]any)["type"].(string) == "finding" {
			count++
		}
	}
	assert.Equal(t, 1, count, "exactly the one valid finding line should become a checklist entry")
}
