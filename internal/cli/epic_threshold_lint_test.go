package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// thresholdLintGraph holds two unmet gates: A's threshold is TBD, B's a draft.
const thresholdLintGraph = `{
  "version": 1,
  "milestones": [["A", "B"]],
  "gates": {"A": {"members": [], "threshold": "TBD"}, "B": {"members": [], "threshold": "draft: 80% pass"}},
  "epics": [
    {"id": "a", "title": "A", "milestone": "A", "root": true},
    {"id": "b", "title": "B", "milestone": "B", "root": true}
  ]
}
`

// unwrittenWarnings is the messages of tp epic lint's gate-threshold-unwritten findings.
func unwrittenWarnings(t *testing.T, dir string) []string {
	t.Helper()
	out, stderr, code := runEpicLint(t, dir)
	require.Equal(t, 0, code, "a warning exits 0: %s", stderr)
	var messages []string
	for _, f := range out.Findings {
		if f.Rule == "gate-threshold-unwritten" {
			assert.Equal(t, "warning", f.Severity)
			messages = append(messages, f.Message)
		}
	}
	return messages
}

// TestEpicLint_WarnsOnAPlaceholderThresholdUnderTheProjectPattern is §8 at
// tp epic lint: under the default pattern TBD warns and the draft does not;
// with epic.threshold_placeholder set to ^draft: only the draft does, and
// the export's lint summary counts the same warning. A writer setting a
// placeholder threshold is not refused.
func TestEpicLint_WarnsOnAPlaceholderThresholdUnderTheProjectPattern(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, thresholdLintGraph)
	byDefault := unwrittenWarnings(t, dir)
	require.Len(t, byDefault, 1)
	assert.Contains(t, byDefault[0], `"A"`)

	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"), []byte(`{"workflow":{},"epic":{"threshold_placeholder":"^draft:"}}`), 0o600))
	byProject := unwrittenWarnings(t, dir)
	require.Len(t, byProject, 1)
	assert.Contains(t, byProject[0], `"B"`)

	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var export struct {
		Lint struct {
			Warnings int `json:"warnings"`
		} `json:"lint"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &export))
	assert.Equal(t, 1, export.Lint.Warnings, "the export counts the warning tp epic lint prints")

	_, stderr, code = runTP(t, dir, "epic", "set", "gates.A.threshold=[threshold]")
	assert.Equal(t, 0, code, "a writer never refuses on a warning: %s", stderr)
}
