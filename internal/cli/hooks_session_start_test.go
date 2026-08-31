package cli

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The plugin's hooks are discovered by convention at the plugin root, the same
// root that already carries skills/tp and .claude-plugin/ (v0.35.0 §6.1).
const (
	pluginHooksManifestPath = "hooks/hooks.json"
	sessionStartHookPath    = "hooks/session-start.sh"
)

// hookTimeoutSeconds is §6.4's bound: every shipped hook declares it, so an
// unbounded hook can never strand an unattended run past its wall-clock budget.
const hookTimeoutSeconds = 10

// installCommand is what the preflight must print when tp is missing or too
// old. §6.1 keeps the binary out of the plugin, so this string is the whole
// remedy the operator gets.
const installCommand = "go install github.com/deligoez/tp/cmd/tp@latest"

type pluginHooksManifest struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Type    string `json:"type"`
			Command string `json:"command"`
			Timeout int    `json:"timeout"`
		} `json:"hooks"`
	} `json:"hooks"`
}

// TestSessionStartHookIsRegisteredForEveryMatcher guards §6.2's first row: the
// session hook fires on every matcher and deliberately does not branch on the
// source, because `clear` is not reliably reported on every client. It also
// pins §6.4's timeout on the hook this task ships, so the bound arrives with
// the hook rather than after it.
func TestSessionStartHookIsRegisteredForEveryMatcher(t *testing.T) {
	var manifest pluginHooksManifest
	raw := readRepoDoc(t, pluginHooksManifestPath)
	require.NoError(t, json.Unmarshal([]byte(raw), &manifest), "%s must be valid JSON", pluginHooksManifestPath)

	groups := manifest.Hooks["SessionStart"]
	require.Len(t, groups, 1, "one SessionStart group: the hook does not branch on the matcher")
	assert.Equal(t, "*", groups[0].Matcher, "the hook runs on every SessionStart source")

	require.Len(t, groups[0].Hooks, 1)
	entry := groups[0].Hooks[0]
	assert.Equal(t, "command", entry.Type)
	assert.Equal(t, "${CLAUDE_PLUGIN_ROOT}/"+sessionStartHookPath, entry.Command,
		"the command resolves through the plugin root rather than the session's cwd")
	assert.Equal(t, hookTimeoutSeconds, entry.Timeout, "§6.4 bounds every shipped hook")

	info, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(sessionStartHookPath)))
	require.NoError(t, err, "%s must exist", sessionStartHookPath)
	assert.NotZero(t, info.Mode().Perm()&0o111, "%s must be executable", sessionStartHookPath)
}

// hookRun is one execution of the session hook against a controlled PATH.
type hookRun struct {
	stdout   string
	stderr   string
	exitCode int
	// tpArgs holds one line per invocation of the fake tp, as it was called.
	tpArgs []string
}

// fakeTpScript stands in for the real binary. It is env-driven so a single
// script covers the absent, stale, current and no-cycle cases, and it logs
// every invocation so a test can assert the hook's payload is `tp resume
// --compact` and nothing else (§6.2).
const fakeTpScript = `#!/bin/sh
printf '%s\n' "$*" >> "$TP_FAKE_LOG"
if [ "$1" = "--version" ]; then
	printf 'tp version %s\n' "$TP_FAKE_VERSION"
	exit 0
fi
if [ "$1" = "resume" ]; then
	printf '%s' "$TP_FAKE_RESUME"
	exit "${TP_FAKE_RESUME_EXIT:-0}"
fi
exit 1
`

// runSessionStartHook executes the hook with a PATH holding only the temporary
// bin directory plus the system directories that carry sh's own utilities. The
// real tp lives in the Go bin directory or Homebrew's, neither of which is on
// that list, so "tp absent" is genuinely absent rather than merely shadowed.
func runSessionStartHook(t *testing.T, version string, extraEnv map[string]string) hookRun {
	t.Helper()

	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "tp-args.log")

	if version != "" {
		fake := filepath.Join(binDir, "tp")
		require.NoError(t, os.WriteFile(fake, []byte(fakeTpScript), 0o700)) //nolint:gosec // test fixture in a temp dir
	}

	env := []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"TP_FAKE_LOG=" + logPath,
		"TP_FAKE_VERSION=" + version,
		"CLAUDE_PLUGIN_ROOT=" + repoRoot(t),
	}
	for key, value := range extraEnv {
		env = append(env, key+"="+value)
	}

	script := filepath.Join(repoRoot(t), filepath.FromSlash(sessionStartHookPath))
	cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = env
	cmd.Dir = repoRoot(t)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	run := hookRun{}
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "the hook must exit rather than fail to start")
		run.exitCode = exitErr.ExitCode()
	}
	run.stdout, run.stderr = stdout.String(), stderr.String()

	if logged, readErr := os.ReadFile(logPath); readErr == nil {
		for _, line := range strings.Split(strings.TrimRight(string(logged), "\n"), "\n") {
			if line != "" {
				run.tpArgs = append(run.tpArgs, line)
			}
		}
	} else {
		require.True(t, errors.Is(readErr, os.ErrNotExist), "unexpected error reading the fake tp log: %v", readErr)
	}
	return run
}

// TestSessionStartHookFailsWhenTpAbsent is test 50's first half: nothing on
// PATH, so the hook must name the install command instead of letting the
// session start with no tp.
func TestSessionStartHookFailsWhenTpAbsent(t *testing.T) {
	run := runSessionStartHook(t, "", nil)

	assert.NotZero(t, run.exitCode, "an absent tp fails the preflight")
	output := run.stdout + run.stderr
	assert.Contains(t, output, installCommand, "the failure carries the install command (§6.1)")
	assert.Empty(t, run.tpArgs, "there was no tp to invoke")
}

// TestSessionStartHookFailsWhenTpTooOld is test 50's second half, and the case
// a happy-path-only test would pass whether or not the comparison works: tp is
// present, answers --version, and is below plugin.json's minimum.
func TestSessionStartHookFailsWhenTpTooOld(t *testing.T) {
	for _, version := range []string{"v0.34.9", "v0.9.0", "0.34.0", "v0.34.3-0.20260820093420-104822c4904b+dirty"} {
		t.Run(version, func(t *testing.T) {
			run := runSessionStartHook(t, version, nil)

			assert.NotZero(t, run.exitCode, "%s is below the plugin minimum", version)
			output := run.stdout + run.stderr
			assert.Contains(t, output, installCommand)
			assert.Contains(t, output, pluginMinVersion, "the failure names the minimum it compared against")
			assert.NotContains(t, strings.Join(run.tpArgs, "\n"), "resume",
				"a failed preflight injects no orientation")
		})
	}
}

// TestSessionStartHookAcceptsTheMinimumVersion pins the boundary rather than a
// value comfortably past it: a check written with 999 passes whether the bound
// is inclusive or exclusive, and §6.1 makes plugin.json's own version the
// minimum, so exactly that version must be accepted.
func TestSessionStartHookAcceptsTheMinimumVersion(t *testing.T) {
	// The last entry is the build tp's own self-development runs against: a
	// binary built from a working tree at the minimum reports a pseudo-version
	// with a suffix, and the preflight compares the numbers only, so dogfooding
	// the release under development is not blocked by its own hook.
	// The literals are all above the minimum by construction rather than by a
	// number that happened to be above it when they were written: a fixed
	// "v0.35.1" was accepted until the minimum reached 0.35.2 and then failed
	// this test, which is the version-pinning trap one level down.
	for _, version := range []string{
		pluginMinVersion, "v" + pluginMinVersion, "v1.0.0", "v99.0.0",
		"v" + pluginMinVersion + "-0.20260820093420-104822c4904b+dirty",
	} {
		t.Run(version, func(t *testing.T) {
			run := runSessionStartHook(t, version, map[string]string{"TP_FAKE_RESUME": "{}"})

			assert.Zero(t, run.exitCode, "%s satisfies the minimum", version)
			assert.NotContains(t, run.stdout+run.stderr, installCommand)
		})
	}
}

// TestSessionStartHookInjectsResumeCompact is the acceptance's second half: on
// a current tp the payload is `tp resume --compact` and nothing else, injected
// as additional context. Claude Code adds a SessionStart hook's plain stdout to
// the session's context on exit 0, so the orientation is that output verbatim
// rather than a re-encoding of it — the fixture carries a quote, a backslash
// and a newline precisely to catch a wrapper that mangles them.
func TestSessionStartHookInjectsResumeCompact(t *testing.T) {
	payload := "{\"phase\":\"implement\",\"note\":\"a \\\"quoted\\\" path C:\\\\tmp\"}\ntrailing"

	run := runSessionStartHook(t, "v"+pluginMinVersion, map[string]string{"TP_FAKE_RESUME": payload})

	require.Zero(t, run.exitCode, "stderr: %s", run.stderr)
	assert.Equal(t, []string{"--version", "resume --compact"}, run.tpArgs,
		"the payload is `tp resume --compact` and nothing else")
	assert.Equal(t, payload, strings.TrimRight(run.stdout, "\n"),
		"the orientation reaches the session byte for byte")
	assert.Empty(t, run.stderr, "a successful preflight is silent")
}

// TestSessionStartHookStaysSilentWithoutACycle covers the session that is not a
// tp cycle at all: `tp resume` fails, and the hook has no orientation to inject.
// Failing the session there would make the plugin unusable in any other
// repository, and §6.1 scopes the preflight's failure to tp's absence or age.
func TestSessionStartHookStaysSilentWithoutACycle(t *testing.T) {
	run := runSessionStartHook(t, "v"+pluginMinVersion, map[string]string{
		"TP_FAKE_RESUME":      "",
		"TP_FAKE_RESUME_EXIT": "3",
	})

	assert.Zero(t, run.exitCode, "no cycle is not a preflight failure; stderr: %s", run.stderr)
	assert.Empty(t, strings.TrimSpace(run.stdout), "nothing to orient with, so nothing is injected")
}

// TestSessionStartHookResolvesItsOwnPluginRoot proves the fallback: a client
// that does not export CLAUDE_PLUGIN_ROOT must still find plugin.json, since
// that file is where the minimum version comes from. Without it the preflight
// would silently compare against nothing.
func TestSessionStartHookResolvesItsOwnPluginRoot(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "tp-args.log")
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "tp"), []byte(fakeTpScript), 0o700)) //nolint:gosec // test fixture

	script := filepath.Join(repoRoot(t), filepath.FromSlash(sessionStartHookPath))
	cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = []string{
		"PATH=" + binDir + ":/usr/bin:/bin",
		"TP_FAKE_LOG=" + logPath,
		"TP_FAKE_VERSION=v0.1.0",
	}
	cmd.Dir = t.TempDir() // not the repo, so only the script's own location can locate the manifest

	out, err := cmd.CombinedOutput()
	require.Error(t, err, "v0.1.0 is below the minimum, so the preflight must fail")
	assert.Contains(t, string(out), installCommand)
	assert.Contains(t, string(out), pluginMinVersion,
		"the minimum was read from plugin.json without CLAUDE_PLUGIN_ROOT")
}
