package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// stopHookPath is the third hook the plugin ships, discovered by convention at
// the plugin root beside the session and write-deny hooks (v0.35.0 §6.1).
const stopHookPath = "hooks/stop-role-incomplete.sh"

// stopUnitEnv is the environment §3.1 gives a child, reduced to the five
// variables the Stop hook reads. It never reads the run state (§6.2): the whole
// point of the role predicate being two conditions over one file is that a hook
// can decide it from these names alone.
type stopUnitEnv struct {
	kind     string
	roundDir string
	unitID   string
	runDir   string
	unitSeq  string
}

// env renders the child environment, omitting a variable the case leaves empty
// so the hook sees it as genuinely unset.
func (e *stopUnitEnv) env() []string {
	out := []string{"PATH=/usr/bin:/bin"}
	for name, value := range map[string]string{
		"TP_UNIT_KIND": e.kind,
		"TP_ROUND_DIR": e.roundDir,
		"TP_UNIT_ID":   e.unitID,
		"TP_RUN_DIR":   e.runDir,
		"TP_UNIT_SEQ":  e.unitSeq,
	} {
		if value != "" {
			out = append(out, name+"="+value)
		}
	}
	return out
}

// findingsPath is the one file a role unit may write (§6.3): the .part name,
// because the hook runs inside the child and the driver's rename to the final
// name has not happened yet.
func (e *stopUnitEnv) findingsPath() string {
	return filepath.Join(e.roundDir, "role-"+e.unitID+".ndjson.part")
}

// escalationPath is the record `tp escalate` writes on the unit's behalf (§5.2).
func (e *stopUnitEnv) escalationPath() string {
	return filepath.Join(e.runDir, e.unitSeq+"-escalation.json")
}

// stopUnit builds a unit of one kind with a real round and run directory, so a
// test can put files exactly where the hook looks for them.
func stopUnit(t *testing.T, kind string) *stopUnitEnv {
	t.Helper()
	root := t.TempDir()
	unit := &stopUnitEnv{
		kind:     kind,
		roundDir: filepath.Join(root, "rounds", "0.35.0", "review-r3"),
		unitID:   "implementer",
		runDir:   filepath.Join(root, "runs", "01JB0000000000000000000000"),
		unitSeq:  "7",
	}
	require.NoError(t, os.MkdirAll(unit.roundDir, 0o755))
	require.NoError(t, os.MkdirAll(unit.runDir, 0o755))
	return unit
}

// runHookScript executes one shipped hook against a payload in a controlled
// environment and reports what it did with it.
func runHookScript(t *testing.T, rel string, env []string, payload []byte) preToolUseRun {
	t.Helper()

	cmd := exec.Command(filepath.Join(repoRoot(t), filepath.FromSlash(rel))) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = env
	cmd.Dir = repoRoot(t)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	run := preToolUseRun{}
	if runErr := cmd.Run(); runErr != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, runErr, &exitErr, "the hook must exit rather than fail to start")
		run.exitCode = exitErr.ExitCode()
	}
	run.stdout, run.stderr = stdout.String(), stderr.String()
	return run
}

// runStopHook feeds the Stop hook the payload shape Claude Code sends on stdin.
// stop_hook_active is the field that says this stop was already blocked once.
func runStopHook(t *testing.T, unit *stopUnitEnv, active bool) preToolUseRun {
	t.Helper()

	payload, err := json.Marshal(map[string]any{
		"session_id":       "3f4a5b6c",
		"transcript_path":  filepath.Join(t.TempDir(), "transcript.jsonl"),
		"cwd":              repoRoot(t),
		"hook_event_name":  "Stop",
		"stop_hook_active": active,
	})
	require.NoError(t, err)

	return runHookScript(t, stopHookPath, unit.env(), payload)
}

// roleKinds are the two kinds §6.2 scopes the hook to; nonRoleKinds are the six
// whose predicates need tp's own state readers and are therefore the driver's
// job, not a hook's.
func roleKinds(t *testing.T) (role, nonRole []string) {
	t.Helper()
	for _, kind := range engine.UnitKinds() {
		if strings.HasSuffix(string(kind), "-role") {
			role = append(role, string(kind))
			continue
		}
		nonRole = append(nonRole, string(kind))
	}
	require.Len(t, role, 2, "§6.2 scopes the Stop hook to the two role kinds")
	require.Len(t, nonRole, 6)
	return role, nonRole
}

// TestStopHookIsRegisteredForEveryStop guards §6.2's third row in the manifest:
// the hook is wired to the Stop event with §6.4's timeout, and the script it
// names exists and can be executed.
func TestStopHookIsRegisteredForEveryStop(t *testing.T) {
	var manifest pluginHooksManifest
	raw := readRepoDoc(t, pluginHooksManifestPath)
	require.NoError(t, json.Unmarshal([]byte(raw), &manifest), "%s must be valid JSON", pluginHooksManifestPath)

	groups := manifest.Hooks["Stop"]
	require.Len(t, groups, 1, "one Stop group: the hook decides on TP_UNIT_KIND, not on a matcher")

	require.Len(t, groups[0].Hooks, 1)
	entry := groups[0].Hooks[0]
	assert.Equal(t, "command", entry.Type)
	assert.Equal(t, "${CLAUDE_PLUGIN_ROOT}/"+stopHookPath, entry.Command,
		"the command resolves through the plugin root rather than the session's cwd")
	assert.Equal(t, hookTimeoutSeconds, entry.Timeout, "§6.4 bounds every shipped hook")

	info, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(stopHookPath)))
	require.NoError(t, err, "%s must exist", stopHookPath)
	assert.NotZero(t, info.Mode().Perm()&0o111, "%s must be executable", stopHookPath)
}

// TestStopHookBlocksARoleUnitThatWroteNeither is test 26's middle case: a role
// unit that is about to stop with neither its findings file nor an escalation
// record has not finished, and the block is the only thing that tells it so
// before the driver charges the attempt as a failure.
func TestStopHookBlocksARoleUnitThatWroteNeither(t *testing.T) {
	role, _ := roleKinds(t)

	for _, kind := range role {
		t.Run(kind, func(t *testing.T) {
			unit := stopUnit(t, kind)
			run := runStopHook(t, unit, false)

			require.Equal(t, 2, run.exitCode, "the stop must be blocked; stderr=%q", run.stderr)
			assert.Contains(t, run.stderr, unit.findingsPath(), "the reason names the file the unit owes")
			assert.Contains(t, run.stderr, "tp escalate", "the reason names the other legitimate ending")
			assert.Empty(t, run.stdout, "the reason reaches the agent on stderr")
		})
	}
}

// TestStopHookAllowsARoleUnitThatEscalated is test 26's first case. An
// escalation is a normal, expected outcome (§5.2), so a unit that wrote one has
// ended legitimately even with no findings file at all.
func TestStopHookAllowsARoleUnitThatEscalated(t *testing.T) {
	role, _ := roleKinds(t)

	for _, kind := range role {
		t.Run(kind, func(t *testing.T) {
			unit := stopUnit(t, kind)
			record := `{"decision":"raise-review-cap","unit_kind":"` + kind + `","unit_id":"implementer"}`
			require.NoError(t, os.WriteFile(unit.escalationPath(), []byte(record), 0o600))

			run := runStopHook(t, unit, false)
			assert.Zero(t, run.exitCode, "an escalation ends the unit; stderr=%q", run.stderr)
			assert.Empty(t, run.stderr, "an allowed stop is silent")
		})
	}
}

// TestStopHookAllowsAnEscalationOverAMalformedFindingsFile pins the "either" in
// §6.2: the two conditions are alternatives, so a unit that escalated is
// allowed to stop however its half-written findings file looks.
func TestStopHookAllowsAnEscalationOverAMalformedFindingsFile(t *testing.T) {
	unit := stopUnit(t, string(engine.UnitAuditRole))
	require.NoError(t, os.WriteFile(unit.findingsPath(), []byte("{\"item_id\":\"x\"\n"), 0o600))
	require.NoError(t, os.WriteFile(unit.escalationPath(), []byte(`{"decision":"other"}`), 0o600))

	run := runStopHook(t, unit, false)
	assert.Zero(t, run.exitCode, "stderr=%q", run.stderr)
	assert.Empty(t, run.stderr)
}

// TestStopHookAllowsACompleteRoleUnit is test 67: the role's .part satisfies
// §3.3's predicate, so the stop goes through. A hook that only ever blocked
// would pass every test above this one and make an agent that can never finish.
func TestStopHookAllowsACompleteRoleUnit(t *testing.T) {
	role, _ := roleKinds(t)

	for _, kind := range role {
		t.Run(kind, func(t *testing.T) {
			unit := stopUnit(t, kind)
			body := `{"class":"missing-guard","location":"internal/engine/driver.go:88","severity":"major"}` + "\n"
			require.NoError(t, os.WriteFile(unit.findingsPath(), []byte(body), 0o600))

			run := runStopHook(t, unit, false)
			assert.Zero(t, run.exitCode, "the durable write is present; stderr=%q", run.stderr)
			assert.Empty(t, run.stderr, "an allowed stop is silent")
		})
	}
}

