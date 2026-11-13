package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// lintSummaryGraph holds one finding per severity: a is listed both as a
// member and as deferrable of A's gate (gate-member-deferrable, an error),
// w's edge to c gives "c" as its why (thin-why, a warning), and a, a member
// of A's gate, waits through w on the condition c of the earlier P
// (gate-held-by-condition, info). P's gate names c, so it is not empty.
const lintSummaryGraph = `{
  "version": 1,
  "milestones": [["P", "A"]],
  "gates": {"P": {"members": ["c"]}, "A": {"members": ["a", "w"], "deferrable": ["a"]}},
  "epics": [
    {"id": "c", "title": "Board approval", "milestone": "P", "condition": true, "acceptance": "the board signs off", "root": true},
    {"id": "w", "title": "Wiring", "milestone": "A", "depends_on": [{"id": "c", "why": "c"}]},
    {"id": "a", "title": "Assembly", "milestone": "A", "depends_on": [{"id": "w", "why": "assembly mounts the wiring harness"}]}
  ]
}
`

// TestEpicExport_LintSummaryEqualsTheLintCounts is spec/1.8.0.md §2: on a
// graph holding one error, one warning and one info finding, tp epic
// export's lint equals tp epic lint's errors, warnings and info count, with
// --compact as without it. An export that leaves the key out under
// --compact, or counts info as a warning, turns it red.
func TestEpicExport_LintSummaryEqualsTheLintCounts(t *testing.T) {
	t.Parallel()
	dir := boundEpicDir(t, lintSummaryGraph)

	stdout, stderr, code := runTP(t, dir, "epic", "lint")
	require.Equal(t, 1, code, "stderr: %s", stderr)
	var lint struct {
		Errors   int `json:"errors"`
		Warnings int `json:"warnings"`
		Findings []struct {
			Rule     string `json:"rule"`
			Severity string `json:"severity"`
		} `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &lint), "stdout: %s", stdout)
	info := 0
	for _, f := range lint.Findings {
		if f.Severity == "info" {
			info++
		}
	}
	require.Equal(t, []int{1, 1, 1}, []int{lint.Errors, lint.Warnings, info}, "the fixture holds one finding per severity: %+v", lint.Findings)

	for _, args := range [][]string{{"epic", "export"}, {"epic", "export", "--compact"}} {
		stdout, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var p struct {
			Lint *struct {
				Errors   int `json:"errors"`
				Warnings int `json:"warnings"`
				Info     int `json:"info"`
			} `json:"lint"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
		require.NotNil(t, p.Lint, "%v prints lint", args)
		assert.Equal(t, []int{lint.Errors, lint.Warnings, info}, []int{p.Lint.Errors, p.Lint.Warnings, p.Lint.Info}, "%v", args)
	}
}
