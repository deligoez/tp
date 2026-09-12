package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// convergedDecomposeRepo is a decompose-phase project — no task, a converged
// review loop — whose workflow registers checksJSON (none when empty.)
func convergedDecomposeRepo(t *testing.T, checksJSON string) string {
	t.Helper()
	dir := suppressionFixture(t, checksJSON)
	writeConvergedRounds(t, dir, 2, 0)
	return dir
}

// TestResume_DecomposeNamesTheCheckGate: tp resume moved to decompose on the
// loop verdict alone, so it sent the agent decomposing while a registered
// review check could still fail — the state v1.2.1 closed for
// `tp review --status`, `--record` and `tp import` and left open here. With a
// check registered, next_action's command is the gate that runs the checks and
// its summary names it ahead of decomposing, in the wording `--status` uses.
func TestResume_DecomposeNamesTheCheckGate(t *testing.T) {
	t.Parallel()
	dir := convergedDecomposeRepo(t, `[{"class":"vague-number","cmd":"touch check-ran"}]`)

	res := resumeResult(t, dir)
	require.Equal(t, "decompose", res["phase"])
	na := res["next_action"].(map[string]any)

	assert.Equal(t, "tp review spec.md --status --check", na["command"],
		"the command is the gate that runs the registered checks")
	// §9.3 is a contract: a phase carrying a tp command carries the command
	// that briefs it. The gate is read-only and idempotent, so it is its own
	// brief — a unit spawned on brief_command gets the check result rather
	// than nothing.
	assert.Equal(t, na["command"], na["brief_command"],
		"a gated step carries the same read-only command in both fields")
	summary, _ := na["summary"].(string)
	gate := strings.Index(summary, "tp review spec.md --status --check")
	require.GreaterOrEqual(t, gate, 0, "the summary names the gate: %s", summary)
	at := strings.Index(summary, "decompos")
	require.GreaterOrEqual(t, at, 0, "and the decomposition it precedes: %s", summary)
	assert.Less(t, gate, at, "the check gate comes before decomposing: %s", summary)

	// resume names the gate; it never runs a check itself. The registered cmd
	// leaves a file behind, so running it would be visible.
	_, err := os.Stat(filepath.Join(dir, "check-ran"))
	assert.True(t, os.IsNotExist(err), "tp resume runs no registered check")
}

// TestResume_DecomposeWithoutARegisteredCheckIsUnchanged is the control: with
// no check registered there is nothing to gate, so decompose keeps its null
// command and its own summary.
func TestResume_DecomposeWithoutARegisteredCheckIsUnchanged(t *testing.T) {
	t.Parallel()
	res := resumeResult(t, convergedDecomposeRepo(t, ""))
	require.Equal(t, "decompose", res["phase"])
	na := res["next_action"].(map[string]any)

	assert.Nil(t, na["command"], "decompose is agent work with no tp command")
	assert.Nil(t, na["brief_command"])
	assert.Equal(t, "decompose the converged spec into tasks and tp import", na["summary"])
}
