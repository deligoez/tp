package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §4.2.2's table, per command. The two commands do not have the same flag sets,
// so the lists are separate rather than "and audit's equivalents" — tp audit
// registers no --perspective, --diff-from, --verify or --report at all, and a
// shared list would silently test nothing for four of review's six rows.
var (
	reviewLegalModes   = []string{"", "--perspective", "--diff-from", "--verify"}
	reviewRefusedModes = []string{"--merge", "--record", "--status", "--report", "--resolve", "--resolve-all"}
	auditLegalModes    = []string{""}
	auditRefusedModes  = []string{"--merge", "--record", "--status", "--resolve", "--resolve-all"}
)

// TestEveryLegalModeEmitsExactlyOnePrompt is the acceptance column of §4.2.2,
// run per command against that command's own list.
//
// Exit 0 alone is not the property. --role selects one prompt, so a mode that
// accepted the flag and emitted the whole panel would pass an exit-code check
// while breaking the thing the flag exists for. Every case asserts the count.
func TestEveryLegalModeEmitsExactlyOnePrompt(t *testing.T) {
	dir, spec, ndjson := roleModeFixture(t)
	baseline := filepath.Join(dir, "baseline.md")
	require.NoError(t, os.WriteFile(baseline,
		[]byte("# tp v0.36.0 — The emitted round\n\n## 1. Overview\n\nold\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "subject.go"), []byte("package subject\n"), 0o600))

	cases := map[string][]string{
		// tp review — four legal modes. The role name per case is whatever
		// that mode emits: --perspective testing emits `test-planner`, which
		// belongs to no corpus, so a single name would not do for all of them.
		"review/default": {"review", spec, "--role", "architect"},
		// No --round: the sandbox carries the spec's recorded state, so the
		// round is state-derived and a literal would conflict with it.
		"review/diff-from":                 {"review", spec, "--diff-from", "baseline.md", "--role", "architect"},
		"review/verify":                    {"review", "--verify", spec, "--findings", ndjson, "--role", "verifier"},
		"review/perspective/regression":    {"review", spec, "--perspective", "regression", "--role", "regression"},
		"review/perspective/code-audit":    {"review", spec, "--perspective", "code-audit", "--affected-files", "subject.go", "--role", "code-auditor"},
		"review/perspective/documentation": {"review", spec, "--perspective", "documentation", "--docs-path", ".", "--role", "documentation-planner"},
		"review/perspective/testing":       {"review", spec, "--perspective", "testing", "--test-path", ".", "--role", "test-planner"},

		// tp audit — default only.
		"audit/default": {"audit", spec, "--affected-files", "subject.go", "--role", "spec-coverage"},
	}

	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			prompts := emitFor(t, dir, args...)
			require.Len(t, prompts, 1,
				"%s: --role reduces the emission to exactly one prompt", name)

			wanted := args[len(args)-1]
			assert.Equal(t, wanted, prompts[0].role,
				"%s: the surviving prompt is the role that was asked for", name)
		})
	}
}

// TestModeTableCoversEveryFlagEachCommandRegisters keeps the two tables above
// honest as the flag sets grow.
//
// Without it, a mode added to either command joins neither column and is tested
// by nothing — the failure §4.2.2 guards against, since a new non-emitting mode
// that accepts --role hands a caller a payload with no prompt in it.
func TestModeTableCoversEveryFlagEachCommandRegisters(t *testing.T) {
	// Flags that select a MODE, as opposed to flags that shape one. The split
	// is the same one §4.2.2 draws: a mode decides whether prompts are emitted
	// at all, so it is a mode flag exactly when it belongs in one of the two
	// columns.
	modeFlags := map[string]bool{
		"--merge": true, "--record": true, "--status": true, "--report": true,
		"--resolve": true, "--resolve-all": true, "--perspective": true,
		"--diff-from": true, "--verify": true,
	}

	for _, c := range []struct {
		command           string
		legal, refused    []string
		absent            []string
		absentExplainedBy string
	}{
		{
			command: "review",
			legal:   reviewLegalModes, refused: reviewRefusedModes,
		},
		{
			command: "audit",
			legal:   auditLegalModes, refused: auditRefusedModes,
			absent:            []string{"--perspective", "--diff-from", "--verify", "--report"},
			absentExplainedBy: "§4.2.2: tp audit registers none of these, which is why the table is per command",
		},
	} {
		t.Run(c.command, func(t *testing.T) {
			help := helpText(t, c.command)

			covered := map[string]bool{}
			for _, f := range append(append([]string{}, c.legal...), c.refused...) {
				if f != "" {
					covered[f] = true
				}
			}

			for flag := range modeFlags {
				registered := strings.Contains(help, flag)
				if !registered {
					assert.False(t, covered[flag],
						"tp %s does not register %s, so it must not be in the table", c.command, flag)
					continue
				}
				assert.True(t, covered[flag],
					"tp %s registers %s but the mode table names it in neither column", c.command, flag)
			}

			for _, flag := range c.absent {
				assert.NotContains(t, helpFlags(help), flag, c.absentExplainedBy)
			}
		})
	}
}

// helpText returns a command's --help output.
func helpText(t *testing.T, command string) string {
	t.Helper()
	stdout, stderr, code := runTPIn(t, t.TempDir(), command, "--help")
	require.Equal(t, 0, code, "tp %s --help must succeed: %s", command, stderr)
	return stdout + stderr
}

// helpFlags keeps the absence check from matching a flag named only in prose:
// it collects the tokens that begin a line's flag column.
func helpFlags(help string) []string {
	out := make([]string, 0, 32)
	for _, line := range strings.Split(help, "\n") {
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "--") {
				out = append(out, strings.TrimSuffix(field, ","))
			}
		}
	}
	return out
}
