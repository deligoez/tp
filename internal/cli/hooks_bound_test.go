package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

// hookBoundMargin is the CPU time these tests give a hook for one case. It is
// half of §6.4's declared timeout, deliberately: the timeout is the runtime's
// backstop for a hook that has already gone wrong, and it runs on the wall
// clock, so a hook whose work alone needs most of it has nothing left once it
// shares a CPU, and is not bounded in any useful sense.
//
// The measure is CPU, not wall clock, because only CPU is a fact about the
// hook. This suite runs with -race beside every other package, often beside
// other agents' gates, and wall clock reports that load as much as the hook: on
// a 10-core machine with 120 busy processes beside it, the 1 MB single-line case
// took 10.4s of wall clock and failed the old 5s wall deadline in every retry,
// while its CPU read 0.57s unloaded and 0.80s loaded, far inside the margin
// both times. What is counted is the hook's own CPU plus that of
// every child it waited for, which is where a shell hook's work happens: on
// Linux and darwin the usage wait4 and waitid return for a process includes its
// reaped descendants, and TestHookBoundJudgesCPUNotWallClock checks it on
// whichever of them it runs on. A child a hook starts and never waits for would
// escape the count; no shipped hook starts one.
const hookBoundMargin = hookTimeoutSeconds * time.Second / 2

// hookHangDeadline is the wall clock after which a hook is killed as hung. It is
// not the bound, only the detector for a hook that never ends, which no CPU
// measure can see: a hook blocked on a read or a sleep spends none. It is six
// times §6.4's timeout so that load cannot reach it — the loaded run above used
// a sixth of it — and the price is paid only by a hook that really hangs.
const hookHangDeadline = 6 * hookTimeoutSeconds * time.Second

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
	t.Parallel()
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
	// cpu is user plus system time of the hook and every child it waited for,
	// as the kernel reported it with the exit status.
	cpu time.Duration
	// elapsed is wall clock, which the verdict reports and never judges.
	elapsed time.Duration
	// timedOut is a hook that did not end on its own: it was killed at the
	// deadline, or it exited leaving a child that held its output open.
	timedOut bool
	stderr   string
}

// runBoundedHook executes one hook script, a shipped one or a stub, and reports
// what it cost. The deadline only stops a hook that never ends: a killed hook
// reports exit -1 and has told nobody anything, which is precisely the failure
// §6.4 names — the driver cannot observe it from outside, so the test observes
// it from here.
//
// The hook runs as the leader of its own process group, and the deadline kills
// the whole group. A hook's children inherit its output, so killing only the
// hook leaves the wait open for as long as a child keeps running — under the
// old 5s kill the 1 MB case was reported at 10.3s, a child still holding the
// pipe — and leaves that child running after the test. The test's cleanup kills
// the group again, for a child the hook left behind when it exited on its own.
// WaitDelay remains for a child that left the group: after the same span again
// the output is closed rather than waited for.
func runBoundedHook(t *testing.T, script string, env []string, in hookStdin, deadline time.Duration) hookBoundRun {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, script) //nolint:gosec // a fixed path inside the repo under test, or a stub the test wrote
	cmd.Env = env
	cmd.Dir = repoRoot(t)
	ownProcessGroup(cmd)
	cmd.Cancel = func() error { return killHookGroup(cmd.Process.Pid) }
	cmd.WaitDelay = deadline

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
	require.NoError(t, cmd.Start(), "the hook must start")
	t.Cleanup(func() { _ = killHookGroup(cmd.Process.Pid) })
	runErr := cmd.Wait()

	state := cmd.ProcessState
	require.NotNil(t, state, "the hook must be waited for: %v", runErr)
	return hookBoundRun{
		exitCode: state.ExitCode(),
		cpu:      state.UserTime() + state.SystemTime(),
		elapsed:  time.Since(started),
		timedOut: ctx.Err() != nil || errors.Is(runErr, exec.ErrWaitDelay),
		stderr:   stderr.String(),
	}
}

// hookBoundVerdict is §6.4's second clause for one run, returned rather than
// asserted so the stubs below can show that it discriminates: "" when the hook
// ended on its own having spent less CPU than margin, otherwise why not.
func hookBoundVerdict(run hookBoundRun, margin time.Duration) string {
	if run.timedOut {
		return fmt.Sprintf("had not ended on its own after %s of wall clock; §6.4 says a hook exits rather than hangs", run.elapsed)
	}
	if run.cpu >= margin {
		return fmt.Sprintf("spent %s of CPU (in %s of wall clock), which leaves no margin inside §6.4's %ds bound", run.cpu, run.elapsed, hookTimeoutSeconds)
	}
	return ""
}

// assertHookBounded holds one run of a shipped hook to the verdict at
// hookBoundMargin, and to a status the harness can act on. wantExit of -1 means
// the case does not pin one, only that it is a status Claude Code defines: 0
// allow, 1 a non-blocking error, 2 the refusal.
func assertHookBounded(t *testing.T, rel, name string, run hookBoundRun, wantExit int) {
	t.Helper()

	require.Empty(t, hookBoundVerdict(run, hookBoundMargin), "%s (%s)", rel, name)

	if wantExit >= 0 {
		assert.Equal(t, wantExit, run.exitCode, "%s (%s): stderr=%s", rel, name, run.stderr)
		return
	}
	assert.Contains(t, []int{0, 1, 2}, run.exitCode,
		"%s (%s): a hook reports 0, 1 or 2; anything else is a crash or a kill, stderr=%s", rel, name, run.stderr)
}

// TestHookBoundJudgesCPUNotWallClock shows the verdict discriminates, on stub
// hooks written here rather than shipped ones, at a margin scaled down so the
// experiment costs a fraction of a second of CPU rather than five seconds.
func TestHookBoundJudgesCPUNotWallClock(t *testing.T) {
	t.Parallel()
	const margin = 100 * time.Millisecond
	env := []string{"PATH=/usr/bin:/bin"}
	stub := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "hook.sh")
		require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700)) //nolint:gosec // a stub hook the test executes
		return path
	}

	t.Run("time off the CPU is not the hook's cost", func(t *testing.T) {
		t.Parallel()
		// A sleeping hook is off the CPU just as a runnable one is while a
		// loaded machine gives its core to someone else, and it needs no load to
		// show it. Its wall clock is ten times the margin; a verdict that read
		// wall clock would fail it, as the old one failed shipped cases under
		// load.
		run := runBoundedHook(t, stub(t, "sleep 1\nexit 2\n"), env, hookStdin{}, hookHangDeadline)
		require.Greater(t, run.elapsed, margin, "the experiment needs a run whose wall clock alone exceeds the margin")
		assert.Empty(t, hookBoundVerdict(run, margin))
		assert.Equal(t, 2, run.exitCode, "stderr=%s", run.stderr)
	})

	t.Run("CPU spent in a child the hook waited for counts against it", func(t *testing.T) {
		t.Parallel()
		// The grind runs in awk, as the shipped hooks' parsing does, so a hook
		// that grinds past the margin fails here — and on whatever OS this runs
		// on, it is the check that the kernel's usage for the hook includes the
		// CPU of a child it waited for. Were it only the shell's own, this would
		// read a few milliseconds and pass the verdict.
		run := runBoundedHook(t, stub(t, "awk 'BEGIN { for (i = 0; i < 20000000; i++) s += i }'\nexit 0\n"), env, hookStdin{}, hookHangDeadline)
		require.False(t, run.timedOut, "the grind ends on its own; only its cost is under test")
		assert.GreaterOrEqual(t, run.cpu, margin, "awk's CPU must reach the hook's measure")
		assert.NotEmpty(t, hookBoundVerdict(run, margin))
	})

	t.Run("a hook that never ends is killed at the deadline, children and all", func(t *testing.T) {
		t.Parallel()
		// sleep runs as a child holding the hook's output. The kill has to reach
		// it too: a hung hook's children are as hung as it is, and one left
		// behind outlives the test on every run. The deadline leaves the stub
		// seconds to record the pid, which takes it milliseconds, before the
		// kill.
		pidfile := filepath.Join(t.TempDir(), "child.pid")
		run := runBoundedHook(t, stub(t, "sleep 60 &\necho $! > '"+pidfile+"'\nwait\nexit 0\n"), env, hookStdin{}, 5*time.Second)
		assert.True(t, run.timedOut)
		assert.NotEmpty(t, hookBoundVerdict(run, margin))
		assert.Less(t, run.elapsed, 30*time.Second, "the wait must end at the kill, not when the child would")
		assertChildGone(t, pidfile)
	})

	t.Run("a child a hook leaves running does not outlive the test", func(t *testing.T) {
		t.Parallel()
		// The hook exits on its own and its child keeps no output open, so
		// nothing is killed at a deadline; the test's cleanup is what ends it.
		// Registered first, this check runs after runBoundedHook's cleanup.
		pidfile := filepath.Join(t.TempDir(), "child.pid")
		t.Cleanup(func() { assertChildGone(t, pidfile) })
		run := runBoundedHook(t, stub(t, "sleep 60 >/dev/null 2>&1 &\necho $! > '"+pidfile+"'\nexit 0\n"), env, hookStdin{}, hookHangDeadline)
		require.False(t, run.timedOut, "the hook exits on its own; only what it leaves behind is under test")
	})
}

// assertChildGone checks that the process whose pid a stub hook wrote to pidfile
// has ended, allowing a moment for an orphan's reaper to collect it.
func assertChildGone(t *testing.T, pidfile string) {
	t.Helper()

	raw, err := os.ReadFile(pidfile) //nolint:gosec // a file inside the test's own temporary directory
	if !assert.NoError(t, err, "the stub records its child's pid before anything can kill it") {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if !assert.NoError(t, err) {
		return
	}
	for deadline := time.Now().Add(5 * time.Second); processAlive(pid) && time.Now().Before(deadline); {
		time.Sleep(20 * time.Millisecond)
	}
	assert.False(t, processAlive(pid), "process %d, started by the hook, is still running after the hook was done with", pid)
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
	t.Parallel()
	env := []string{"PATH=/usr/bin:/bin"}

	for _, rel := range shippedHookScripts(t) {
		for _, in := range hookAdverseStdin(t) {
			t.Run(rel+" "+in.name, func(t *testing.T) {
				run := runBoundedHook(t, filepath.Join(repoRoot(t), filepath.FromSlash(rel)), env, in, hookHangDeadline)
				assertHookBounded(t, rel, in.name, run, -1)
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
	batchPayload := func(created, lastEdit string) func(*testing.T) []byte {
		return func(t *testing.T) []byte {
			t.Helper()
			op := func(tool string, args map[string]any) map[string]any {
				return map[string]any{"tool": tool, "args": args}
			}
			payload, err := json.Marshal(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "mcp__codedbpro__batch",
				"tool_input": map[string]any{"ops": []any{
					op("read", map[string]any{"file": "spec/.tp-review/1.1.0/snapshot-round-1.md"}),
					op("create", map[string]any{"file": created, "content": strings.Repeat(`"{payload}" \`, 174763)}),
					op("faster_search", map[string]any{"path": ".tp/config.json", "pattern": "gate"}),
					op("edit", map[string]any{"file": lastEdit, "content": "{}"}),
				}},
			})
			require.NoError(t, err)
			require.Greater(t, len(payload), 2<<20, "the large batch case must actually be large")
			return payload
		}
	}
	// roleUnit is a role unit's environment with its directories named
	// relative to the hook's cwd, so the payload can name its files without
	// knowing a temporary directory.
	roleUnit := func(*testing.T) []string {
		return []string{
			"PATH=/usr/bin:/bin",
			"TP_ROUND_DIR=.tp/rounds/0.35.0/review-r1",
			"TP_UNIT_ID=implementer",
			"TP_RUN_DIR=.tp/runs/01JB0000000000000000000000",
			"TP_UNIT_SEQ=1",
		}
	}
	const roleFindings = ".tp/rounds/0.35.0/review-r1/role-implementer.ndjson.part"
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
		preToolUseHookPath: {
			{
				// The fenced path sits after two megabytes of content, so a hook that
				// gave up on the payload rather than scanning it would fall open here.
				name:     "a fenced path behind 2 MiB of content",
				env:      minimal,
				payload:  writePayload(".tp/config.json", strings.Repeat("payload ", 262144)),
				wantExit: 2,
			},
			{
				// A batch is read operation by operation, so its whole payload is
				// walked rather than grepped: two megabytes of escaped quotes and
				// braces precede the one write into the fence, and the reads beside
				// it must be classified without the walk grinding past the margin.
				name:     "a batched fenced write behind 2 MiB of escaped content",
				env:      minimal,
				payload:  batchPayload("docs/notes.md", "spec/.tp-review/1.1.0/state.json"),
				wantExit: 2,
			},
			{
				// The same batch with its last write moved out of the fence. The
				// refusal above cannot tell a finished walk from one that gave up
				// and judged the whole payload; only a pass can, because the reads
				// still name fenced paths and pass only once the walk has
				// classified them.
				name:     "a batch whose fenced paths are all reads, behind 2 MiB of escaped content",
				env:      minimal,
				payload:  batchPayload("docs/notes.md", "internal/cli/run.go"),
				wantExit: 0,
			},
		},
		roleWriteHookPath: {
			{
				name:     "no round environment to build the allowlist from",
				env:      minimal,
				payload:  writePayload("internal/cli/root.go", "package cli"),
				wantExit: 2,
			},
			{
				// The allowlist walks a batch through the same classifier, so the
				// same two experiments hold here: a write outside the unit's two
				// files behind two megabytes of escaped content is still refused
				// inside the margin...
				name:     "a batched write outside the unit's files behind 2 MiB of escaped content",
				env:      roleUnit,
				payload:  batchPayload(roleFindings, "spec/0.35.0.md"),
				wantExit: 2,
			},
			{
				// ...and a batch whose only writes are the unit's own passes, which
				// only a finished walk can decide, since its reads name files the
				// unit may not write.
				name:     "a batch writing only the unit's own files, behind 2 MiB of escaped content",
				env:      roleUnit,
				payload:  batchPayload(roleFindings, ".tp/runs/01JB0000000000000000000000/1-escalation.json"),
				wantExit: 0,
			},
		},
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
	t.Parallel()
	cases := hookFailClosedCases()

	for _, rel := range shippedHookScripts(t) {
		entries := cases[rel]
		require.NotEmpty(t, entries,
			"%s ships without an experiment showing what it does when it cannot do its job; §6.4's second clause is a behaviour, and a hook added without one is unverified", rel)

		for _, tc := range entries {
			t.Run(rel+" "+tc.name, func(t *testing.T) {
				script := filepath.Join(repoRoot(t), filepath.FromSlash(rel))
				run := runBoundedHook(t, script, tc.env(t), hookStdin{name: tc.name, payload: tc.payload(t)}, hookHangDeadline)
				assertHookBounded(t, rel, tc.name, run, tc.wantExit)
			})
		}
	}
}
