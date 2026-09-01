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

// §4.2.2's table, per command. The two commands do not have the same flag sets,
// so the lists are separate rather than "and audit's equivalents" — tp audit
// registers no --perspective, --diff-from, --verify or --report at all, and a
// shared list would silently skip every mode only one command has.
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
//
// This test ALONE is not the property either, and audit round 1 proved it: in a
// mode that emits exactly one prompt anyway, passing that mode's own role name
// and asserting "one prompt, and it is that role" holds identically whether the
// filter ran or the flag was discarded. Five of the eight cases below were
// tautologies, and `--role` was in fact ignored in all five. The three
// name-class assertions in TestEveryLegalModeClassifiesTheNameItIsGiven are
// what could have failed; this one is kept for the count, not for the class.
func TestEveryLegalModeEmitsExactlyOnePrompt(t *testing.T) {
	dir, cases := legalModeCases(t)

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			prompts := emitFor(t, dir, append(c.args, "--role", c.ownRole)...)
			require.Len(t, prompts, 1,
				"%s: --role reduces the emission to exactly one prompt", name)
			assert.Equal(t, c.ownRole, prompts[0].role,
				"%s: the surviving prompt is the role that was asked for", name)
		})
	}
}

// TestEveryLegalModeClassifiesTheNameItIsGiven is the experiment that can fail.
//
// §4.2.1's three name classes must hold in EVERY mode §4.2.2 calls legal, not
// only in the two that happen to route through the default emission path. Round
// 1 measured all three collapsing to one outcome in five of tp review's seven
// emitting modes: `--verify` and the four `--perspective` values returned before
// the filter, so an unrecognised name exited 0 with a prompt, and asking for a
// role that mode does not emit handed back a DIFFERENT role's prompt.
//
// Each case uses a name the mode cannot emit, which is what makes the assertion
// discriminating: the mode's own name proves nothing, because it is what an
// ignored flag produces too.
func TestEveryLegalModeClassifiesTheNameItIsGiven(t *testing.T) {
	dir, cases := legalModeCases(t)

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// A name tp recognises nowhere: exit 2.
			_, stderr, code := runTPIn(t, dir, append(c.args, "--role", "zzz-not-a-role")...)
			assert.Equal(t, 2, code,
				"%s: an unrecognised name is a usage error, not a silent full emission; stderr: %s",
				name, stderr)

			// --role "" is a name, not an absent flag.
			_, _, emptyCode := runTPIn(t, dir, append(c.args, "--role", "")...)
			assert.Equal(t, 2, emptyCode,
				`%s: --role "" is refused as an unknown role`, name)

			// A name tp recognises that this mode does not emit: exit 0, empty.
			require.NotEqual(t, c.ownRole, c.foreignRole, "the foreign role must differ from the mode's own")
			stdout, foreignErr, foreignCode := runTPIn(t, dir,
				append(c.args, "--role", c.foreignRole, "--json")...)
			require.Equal(t, 0, foreignCode,
				"%s: a recognised name is not an error; stderr: %s", name, foreignErr)

			var payload struct {
				Prompts []struct {
					Role string `json:"role"`
				} `json:"prompts"`
			}
			require.NoError(t, json.Unmarshal([]byte(stdout), &payload))
			assert.Empty(t, payload.Prompts,
				"%s: --role %s emits nothing here; handing back %q instead is the defect round 1 found",
				name, c.foreignRole, c.ownRole)
		})
	}
}

// legalModeCase is one row of §4.2.2's acceptance column.
//
// ownRole is what the mode emits; foreignRole is a name tp recognises that this
// mode never emits. Both are needed: the first tests selection, the second
// tests that the filter ran at all.
type legalModeCase struct {
	args        []string
	ownRole     string
	foreignRole string
}

func legalModeCases(t *testing.T) (dir string, cases map[string]legalModeCase) {
	t.Helper()
	dir, spec, ndjson := roleModeFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "baseline.md"),
		[]byte("# tp v0.36.0 — The emitted round\n\n## 1. Overview\n\nold\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "subject.go"), []byte("package subject\n"), 0o600))

	// `architect` is an embedded reviewer role, so it is recognised everywhere
	// and emitted only by the default panel. `spec-coverage` plays the same
	// part for tp audit, where the default panel is the only emitting mode, so
	// its foreign name has to come from the reviewer corpus instead.
	return dir, map[string]legalModeCase{
		"review/default": {[]string{"review", spec}, "architect", "spec-coverage"},
		// No --round: the sandbox carries the spec's recorded state, so the
		// round is state-derived and a literal would conflict with it.
		"review/diff-from":                 {[]string{"review", spec, "--diff-from", "baseline.md"}, "architect", "spec-coverage"},
		"review/verify":                    {[]string{"review", "--verify", spec, "--findings", ndjson}, "verifier", "architect"},
		"review/perspective/regression":    {[]string{"review", spec, "--perspective", "regression"}, "regression", "architect"},
		"review/perspective/code-audit":    {[]string{"review", spec, "--perspective", "code-audit", "--affected-files", "subject.go"}, "code-auditor", "architect"},
		"review/perspective/documentation": {[]string{"review", spec, "--perspective", "documentation", "--docs-path", "."}, "documentation-planner", "architect"},
		"review/perspective/testing":       {[]string{"review", spec, "--perspective", "testing", "--test-path", "."}, "test-planner", "architect"},

		// tp audit — default only.
		"audit/default": {[]string{"audit", spec, "--affected-files", "subject.go"}, "spec-coverage", "architect"},
	}
}

// TestModeTableCoversEveryFlagEachCommandRegisters checks the two tables above
// against the hand-written modeFlags map below, and NOT against the flags each
// command actually registers.
//
// The distinction is the whole content of this comment, because an earlier
// version claimed the wider property — that a mode added to either command
// "joins neither column and is tested by nothing" would be caught here. It is
// not. Measured in v0.36.0's audit round 12: registering a genuinely new mode
// flag (`--zzprobe`) that is absent from modeFlags leaves this test green. What
// it catches is drift between the map and the two columns, which is narrower
// and still worth having.
//
// It is left narrow deliberately. No clause in v0.36.0 requires this guard to
// be exhaustive over registered flags — five were checked for such a
// requirement (§4.2.2's two tables, list-2-6, task-role-mode-legality,
// task-role-mode-test) and all five speak of a per-command enumerated list. The
// derived-from-the-code requirement lives elsewhere and is honoured elsewhere:
// clause_absence_test.go asks the binary for its own perspective list rather
// than restating it. Widening this one is spec/0.46.0.md's, not this release's.
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
	for line := range strings.SplitSeq(help, "\n") {
		for field := range strings.FieldsSeq(line) {
			if strings.HasPrefix(field, "--") {
				out = append(out, strings.TrimSuffix(field, ","))
			}
		}
	}
	return out
}
