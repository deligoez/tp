package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// thresholdRewriteGraph is spec/1.8.0.md §7: P's members are done and its
// threshold TBD is a placeholder; W's members are done and its threshold is
// written; D's placeholder threshold carries a met record; Q's members are
// not done and its threshold [threshold] is a placeholder.
const thresholdRewriteGraph = `{
  "version": 1,
  "milestones": [["P", "W", "D", "Q"]],
  "gates": {
    "P": {"threshold": "TBD"},
    "W": {"threshold": "p95 under 200ms"},
    "D": {"threshold": "TBD", "met": {"at": "2026-01-03T03:04:05Z", "evidence": "measured", "threshold": "TBD"}},
    "Q": {"threshold": "[threshold]"}
  },
  "epics": [
    {"id": "p", "title": "P", "milestone": "P", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "w", "title": "W", "milestone": "W", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "d", "title": "D", "milestone": "D", "root": true, "done": {"at": "2026-01-03T03:04:05Z", "evidence": "shipped"}},
    {"id": "q", "title": "Q", "milestone": "Q", "root": true}
  ]
}
`

// operatorGateLists is an operator object reduced to its two gate lists.
type operatorGateLists struct {
	Gates []struct {
		Milestone string `json:"milestone"`
	} `json:"gates"`
	ThresholdRewrites []struct {
		Milestone string `json:"milestone"`
		Threshold string `json:"threshold"`
		Command   string `json:"command"`
	} `json:"threshold_rewrites"`
}

// add appends o's gate milestones and rewrite milestones to gates and
// rewrites, and each rewrite's command to commands.
func (o *operatorGateLists) add(gates, rewrites, commands *[]string) {
	for _, g := range o.Gates {
		*gates = append(*gates, g.Milestone)
	}
	for _, r := range o.ThresholdRewrites {
		*rewrites = append(*rewrites, r.Milestone)
		*commands = append(*commands, r.Command)
	}
}

// TestEpicOperator_PlaceholderGatesAreListedAsRewritesOnly is spec/1.8.0.md
// §7: operator.gates lists W alone, the gate whose members are done and whose
// threshold is written; P and Q, unmet with a placeholder threshold, are each
// listed once as a rewrite carrying tp epic set gates.<m>.threshold=, whether
// or not their members are done; D, met, is in neither. tp epic ready, tp
// epic status and tp epic export carry the same lists. Mutants keeping
// placeholder gates under gates, or listing a met placeholder gate as a
// rewrite, turn it red.
func TestEpicOperator_PlaceholderGatesAreListedAsRewritesOnly(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, thresholdRewriteGraph)

	stdout, stderr, code := runTP(t, dir, "epic", "ready")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var ready struct {
		Operator operatorGateLists `json:"operator"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &ready), "stdout: %s", stdout)
	gates, rewrites, commands := []string{}, []string{}, []string{}
	ready.Operator.add(&gates, &rewrites, &commands)
	assert.Equal(t, []string{"W"}, gates, "epic ready: gates")
	assert.Equal(t, []string{"P", "Q"}, rewrites, "epic ready: rewrites")
	require.Len(t, commands, 2)
	assert.Contains(t, commands[0], "tp epic set gates.P.threshold=")
	assert.Contains(t, commands[1], "tp epic set gates.Q.threshold=")
	assert.Equal(t, "TBD", ready.Operator.ThresholdRewrites[0].Threshold)

	for _, cmd := range []string{"status", "export"} {
		stdout, stderr, code := runTP(t, dir, "epic", cmd)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		var p struct {
			Milestones []struct {
				Operator operatorGateLists `json:"operator"`
			} `json:"milestones"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &p), "stdout: %s", stdout)
		g, r, c := []string{}, []string{}, []string{}
		for i := range p.Milestones {
			p.Milestones[i].Operator.add(&g, &r, &c)
		}
		assert.Equal(t, gates, g, "epic %s: gates", cmd)
		assert.Equal(t, rewrites, r, "epic %s: rewrites", cmd)
		assert.Equal(t, commands, c, "epic %s: commands", cmd)
	}
}
