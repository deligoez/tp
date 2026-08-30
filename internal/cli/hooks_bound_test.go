package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pluginHooksDir is where the plugin's hook scripts live, discovered by
// convention at the plugin root beside skills/, agents/ and .claude-plugin/
// (v0.35.0 §6.1).
const pluginHooksDir = "hooks"

// hookBoundMargin is what these tests give a hook to finish. It is half of
// §6.4's declared timeout, deliberately: the timeout is the runtime's backstop
// for a hook that has already gone wrong, so a hook that needs most of it on a
// quiet machine has nothing left on a loaded one and is not bounded in any
// useful sense.
const hookBoundMargin = hookTimeoutSeconds * time.Second / 2

// shippedHookDecl is one registration of one hook script: the script, the event
// it fires on, the bound declared for it, and the file that declares it.
type shippedHookDecl struct {
	source  string
	event   string
	typ     string
	command string
	timeout int
}

// where names the declaration in a failure message, since the same script may
// be registered from more than one place.
func (d shippedHookDecl) where() string {
	return d.source + " " + d.event + " -> " + d.command
}

// shippedHookDeclarations enumerates every hook declaration the plugin ships,
// from both places a plugin can carry one: the manifest at hooks/hooks.json,
// and the frontmatter of each agent definition under agents/ — §6.3's role
// write allowlist is declared there and appears in the manifest nowhere.
//
// Nothing here is enumerated by hand. §6.4 is a release-wide invariant over
// *every* shipped hook, so a fifth hook registered in either place is held to
// the bound the moment it ships, rather than the next time someone remembers to
// extend a per-hook test.
func shippedHookDeclarations(t *testing.T) []shippedHookDecl {
	t.Helper()

	out := make([]shippedHookDecl, 0, 8)

	var manifest pluginHooksManifest
	raw := readRepoDoc(t, pluginHooksManifestPath)
	require.NoError(t, json.Unmarshal([]byte(raw), &manifest), "%s must be valid JSON", pluginHooksManifestPath)
	for event, groups := range manifest.Hooks {
		for _, group := range groups {
			for _, entry := range group.Hooks {
				out = append(out, shippedHookDecl{
					source:  pluginHooksManifestPath,
					event:   event,
					typ:     entry.Type,
					command: entry.Command,
					timeout: entry.Timeout,
				})
			}
		}
	}

	entries, err := os.ReadDir(filepath.Join(repoRoot(t), pluginAgentsDir))
	require.NoError(t, err, "%s/ must exist at the plugin root", pluginAgentsDir)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		rel := pluginAgentsDir + "/" + entry.Name()
		for event, groups := range readAgentDefinition(t, rel).Hooks {
			for _, group := range groups {
				for _, hook := range group.Hooks {
					out = append(out, shippedHookDecl{
						source:  rel,
						event:   event,
						typ:     hook.Type,
						command: hook.Command,
						timeout: hook.Timeout,
					})
				}
			}
		}
	}

	require.NotEmpty(t, out, "the plugin ships hooks; discovering none would make every assertion below vacuous")
	sort.Slice(out, func(i, j int) bool {
		if out[i].source != out[j].source {
			return out[i].source < out[j].source
		}
		if out[i].event != out[j].event {
			return out[i].event < out[j].event
		}
		return out[i].command < out[j].command
	})
	return out
}

// shippedHookScripts lists the hook scripts the plugin ships on disk. It is the
// other half of the invariant: the declarations say what is bounded, and this
// says what is shipped, so a script that ships without a declaration cannot
// hide behind the hooks that do have one.
func shippedHookScripts(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(repoRoot(t), pluginHooksDir))
	require.NoError(t, err, "%s/ must exist at the plugin root", pluginHooksDir)

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sh") {
			out = append(out, pluginHooksDir+"/"+entry.Name())
		}
	}
	sort.Strings(out)
	require.NotEmpty(t, out, "the plugin ships hook scripts; discovering none would make every assertion below vacuous")
	return out
}

// TestEveryShippedHookDeclaresTheBound is test 43's declarative half, taken over
// every hook the plugin ships rather than one per hook: each declaration names a
// command inside hooks/, resolves it through the plugin root rather than the
// session's cwd, carries §6.4's timeout, and points at a script that exists and
// can be executed. The closing loop is the invariant the per-hook tests cannot
// state — a script that ships under hooks/ and is registered nowhere is bounded
// by nothing.
func TestEveryShippedHookDeclaresTheBound(t *testing.T) {
	const prefix = "${CLAUDE_PLUGIN_ROOT}/"

	declared := make(map[string]bool)
	for _, decl := range shippedHookDeclarations(t) {
		assert.Equal(t, "command", decl.typ, "%s: §6.4 bounds command hooks", decl.where())
		require.True(t, strings.HasPrefix(decl.command, prefix),
			"%s: the command resolves through the plugin root rather than the session's cwd", decl.where())

		rel := strings.TrimPrefix(decl.command, prefix)
		assert.True(t, strings.HasPrefix(rel, pluginHooksDir+"/"),
			"%s: a shipped hook lives under %s/, which is what this test enumerates", decl.where(), pluginHooksDir)
		assert.Equal(t, hookTimeoutSeconds, decl.timeout,
			"%s: §6.4 bounds every shipped hook at %d seconds", decl.where(), hookTimeoutSeconds)

		info, err := os.Stat(filepath.Join(repoRoot(t), filepath.FromSlash(rel)))
		require.NoError(t, err, "%s: the declared script must exist", decl.where())
		assert.NotZero(t, info.Mode().Perm()&0o111, "%s: %s must be executable", decl.where(), rel)

		declared[rel] = true
	}

	for _, rel := range shippedHookScripts(t) {
		assert.True(t, declared[rel],
			"%s ships in the plugin but no manifest or agent definition registers it, so nothing declares its bound (§6.4)", rel)
	}
}

// hookStdin is one thing a hook can be handed on stdin. A hook reads its payload
// with `cat`, so stdin is where it blocks if it blocks at all — which is why the
// shapes below are the ones worth running rather than a well-formed payload.
type hookStdin struct {
	name    string
	payload []byte
	// devNull hands the hook nothing at all, the way a harness that forgot to
	// wire stdin does.
	devNull bool
	// closedPipe hands it a pipe whose writer is already gone.
	closedPipe bool
}

// hookBoundRun is one bounded execution of a hook.
type hookBoundRun struct {
	exitCode int
	elapsed  time.Duration
	timedOut bool
	stderr   string
}

// runBoundedHook executes one shipped hook under a deadline. The deadline is the
// experiment: a hook that has to be killed reports exit -1 and told nobody
// anything, which is precisely the failure §6.4 names — the driver cannot
// observe it from outside, so the test observes it from here.
func runBoundedHook(t *testing.T, rel string, env []string, in hookStdin) hookBoundRun {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), hookBoundMargin)
	defer cancel()

	cmd := exec.CommandContext(ctx, filepath.Join(repoRoot(t), filepath.FromSlash(rel))) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = env
	cmd.Dir = repoRoot(t)

	switch {
	case in.devNull:
		cmd.Stdin = nil
	case in.closedPipe:
		reader, writer, pipeErr := os.Pipe()
		require.NoError(t, pipeErr)
		require.NoError(t, writer.Close())
		defer func() { _ = reader.Close() }()
		cmd.Stdin = reader
	default:
		cmd.Stdin = bytes.NewReader(in.payload)
	}

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	started := time.Now()
	runErr := cmd.Run()

	run := hookBoundRun{elapsed: time.Since(started), stderr: stderr.String(), timedOut: ctx.Err() != nil}
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			run.exitCode = exitErr.ExitCode()
		} else {
			require.True(t, run.timedOut, "the hook must exit rather than fail to start: %v", runErr)
		}
	}
	return run
}

// assertHookBounded is §6.4's second clause for one run: the hook ended on its
// own, inside the margin, with a status the harness can act on. wantExit of -1
// means the case does not pin one, only that it is a status Claude Code defines:
// 0 allow, 1 a non-blocking error, 2 the refusal.
func assertHookBounded(t *testing.T, rel, name string, run hookBoundRun, wantExit int) {
	t.Helper()

	require.False(t, run.timedOut,
		"%s (%s): still running after %s and had to be killed; §6.4 says a hook exits rather than hangs", rel, name, run.elapsed)
	assert.Less(t, run.elapsed, hookBoundMargin,
		"%s (%s): took %s, which leaves no margin inside §6.4's %ds bound", rel, name, run.elapsed, hookTimeoutSeconds)

	if wantExit >= 0 {
		assert.Equal(t, wantExit, run.exitCode, "%s (%s): stderr=%s", rel, name, run.stderr)
		return
	}
	assert.Contains(t, []int{0, 1, 2}, run.exitCode,
		"%s (%s): a hook reports 0, 1 or 2; anything else is a crash or a kill, stderr=%s", rel, name, run.stderr)
}

// hookAdverseStdin are the inputs a well-formed payload never exercises. Each is
// a shape that has stranded a hook somewhere: a harness that wired no stdin at
// all, one that closed the pipe, bytes that are not JSON, a payload cut off
// mid-key, and one large enough that buffering it is itself the work.
func hookAdverseStdin(t *testing.T) []hookStdin {
	t.Helper()

	huge, err := json.Marshal(map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"tool_input": map[string]any{
			"content":   strings.Repeat("payload ", 262144),
			"file_path": "docs/notes.md",
		},
	})
	require.NoError(t, err)
	require.Greater(t, len(huge), 2<<20, "the large-payload case must actually be large")

	return []hookStdin{
		{name: "empty stdin", payload: []byte{}},
		{name: "no stdin at all", devNull: true},
		{name: "a closed pipe", closedPipe: true},
		{name: "bytes that are not JSON", payload: []byte("\x01not json at all\x02 {\"file_path\"")},
		{name: "JSON truncated mid-key", payload: []byte(`{"tool_name":"Write","tool_input":{"file_path":"`)},
		{name: "a payload over 2 MiB", payload: huge},
	}
}

// TestEveryShippedHookTerminatesOnAdverseInput is test 43's behavioural half,
// and the reason it is not a manifest assertion: reading `timeout: 10` out of a
// declaration proves the bound was written down, not that the hook stays inside
// it. Every shipped script is run against every adverse stdin, discovered rather
// than listed, so a hook added later is exercised on the same inputs.
func TestEveryShippedHookTerminatesOnAdverseInput(t *testing.T) {
	env := []string{"PATH=/usr/bin:/bin"}

	for _, rel := range shippedHookScripts(t) {
		for _, in := range hookAdverseStdin(t) {
			t.Run(rel+" "+in.name, func(t *testing.T) {
				assertHookBounded(t, rel, in.name, runBoundedHook(t, rel, env, in), -1)
			})
		}
	}
}

// hookFailClosedCase is the experiment the second clause needs. An adverse
// payload a hook simply ignores cannot discriminate: the hook exits 0 whether it
// is bounded or merely lucky. Each case below hands one hook the input on which
// it must do its job — refuse, or decide a predicate over a file it did not
// write — and asserts it does so inside the margin rather than grinding.
type hookFailClosedCase struct {
	name     string
	env      func(t *testing.T) []string
	payload  func(t *testing.T) []byte
	wantExit int
}

// stopHookUnitEnv builds the child environment §3.1 gives a role unit, with the
// findings file written by the caller's own function.
func stopHookUnitEnv(t *testing.T, findings []byte) []string {
	t.Helper()

	root := t.TempDir()
	roundDir := filepath.Join(root, "rounds", "0.35.0", "review-r1")
	runDir := filepath.Join(root, "runs", "01JB0000000000000000000000")
	require.NoError(t, os.MkdirAll(roundDir, 0o750))
	require.NoError(t, os.MkdirAll(runDir, 0o750))

	if findings != nil {
		require.NoError(t, os.WriteFile(filepath.Join(roundDir, "role-implementer.ndjson.part"), findings, 0o600))
	}

	return []string{
		"PATH=/usr/bin:/bin",
		"TP_UNIT_KIND=review-role",
		"TP_UNIT_ID=implementer",
		"TP_ROUND_DIR=" + roundDir,
		"TP_RUN_DIR=" + runDir,
		"TP_UNIT_SEQ=1",
	}
}

// hookFailClosedCases keys one or more experiments to each shipped hook. The
// test below requires an entry per shipped script, so a hook added without an
// experiment fails rather than passing unverified.
func hookFailClosedCases() map[string][]hookFailClosedCase {
	minimal := func(*testing.T) []string { return []string{"PATH=/usr/bin:/bin"} }
	writePayload := func(path string, content string) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			t.Helper()
			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Write",
				"tool_input":      map[string]any{"content": content, "file_path": path},
			})
			require.NoError(t, err)
			return payload
		}
	}
	stopPayload := func(*testing.T) []byte {
		return []byte(`{"hook_event_name":"Stop","stop_hook_active":false}`)
	}

	return map[string][]hookFailClosedCase{
		sessionStartHookPath: {{
			name: "tp is not on PATH",
			env: func(t *testing.T) []string {
				t.Helper()
				return []string{"PATH=" + t.TempDir(), "CLAUDE_PLUGIN_ROOT=" + repoRoot(t)}
			},
			payload:  func(*testing.T) []byte { return []byte(`{"hook_event_name":"SessionStart","source":"startup"}`) },
			wantExit: 2,
		}},
		preToolUseHookPath: {{
			// The fenced path sits after two megabytes of content, so a hook that
			// gave up on the payload rather than scanning it would fall open here.
			name:     "a fenced path behind 2 MiB of content",
			env:      minimal,
			payload:  writePayload(".tp/config.json", strings.Repeat("payload ", 262144)),
			wantExit: 2,
		}},
		roleWriteHookPath: {{
			name:     "no round environment to build the allowlist from",
			env:      minimal,
			payload:  writePayload("internal/cli/root.go", "package cli"),
			wantExit: 2,
		}},
		stopHookPath: {
			{
				name:     "a role unit that wrote no findings file",
				env:      func(t *testing.T) []string { t.Helper(); return stopHookUnitEnv(t, nil) },
				payload:  stopPayload,
				wantExit: 2,
			},
			{
				// The hook decides §3.3's predicate over a file the unit wrote and
				// nothing bounds the size of. A role that dumps a large blob on one
				// line is exactly when the block matters, so it is exactly when the
				// predicate must still be decidable inside the bound.
				name: "a findings file that is one megabyte on a single line",
				env: func(t *testing.T) []string {
					t.Helper()
					line := `{"role":"implementer","evidence":"` + strings.Repeat("x", 1<<20) + `"}` + "\n"
					return stopHookUnitEnv(t, []byte(line))
				},
				payload:  stopPayload,
				wantExit: 0,
			},
		},
	}
}

// TestEveryShippedHookFailsClosedInsideTheBound is the discriminating half of
// test 43. The adverse-stdin sweep shows a hook terminates; these show it
// terminates having done its job — the refusal is still a refusal, and the one
// hook that reads a file it did not write still decides its predicate inside the
// bound.
func TestEveryShippedHookFailsClosedInsideTheBound(t *testing.T) {
	cases := hookFailClosedCases()

	for _, rel := range shippedHookScripts(t) {
		entries := cases[rel]
		require.NotEmpty(t, entries,
			"%s ships without an experiment showing what it does when it cannot do its job; §6.4's second clause is a behaviour, and a hook added without one is unverified", rel)

		for _, tc := range entries {
			t.Run(rel+" "+tc.name, func(t *testing.T) {
				run := runBoundedHook(t, rel, tc.env(t), hookStdin{name: tc.name, payload: tc.payload(t)})
				assertHookBounded(t, rel, tc.name, run, tc.wantExit)
			})
		}
	}
}
