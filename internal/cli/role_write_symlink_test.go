package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoleWriteHookAllowsAcrossASymlinkedRoot pins the one direction the
// allowlist must never get wrong: refusing the single file the unit is allowed
// to write.
//
// Found by audit round 4, and reachable in this repo's own documented workflow.
// The driver hands TP_ROUND_DIR by whatever spelling the project was reached
// through, while the child it spawns reports a physical $PWD. On macOS /tmp is
// a symlink to /private/tmp and CLAUDE.md's Manual QA recipe puts projects under
// /tmp, so the two spellings differ for every QA run. A textual comparison then
// denied the role's own findings file; the role wrote nothing, the Stop hook
// blocked it for having written nothing, and every retry failed the same way.
//
// The deny side is asserted in the same test on purpose. Resolving symlinks can
// only make two paths compare equal when they name the same file, but that is
// an argument, and an argument is what a test is for.
func TestRoleWriteHookAllowsAcrossASymlinkedRoot(t *testing.T) {
	target := filepath.Join(t.TempDir(), "real")
	round := filepath.Join(target, "rounds", "review-r1")
	run := filepath.Join(target, "runs", "01JB0000000000000000000000")
	require.NoError(t, os.MkdirAll(round, 0o755))
	require.NoError(t, os.MkdirAll(run, 0o755))

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))

	// The driver's spelling goes through the symlink; the child's cwd is
	// physical. This is the pair that used to disagree.
	linkRound := filepath.Join(link, "rounds", "review-r1")
	linkRun := filepath.Join(link, "runs", "01JB0000000000000000000000")
	physicalRound, err := filepath.EvalSymlinks(round)
	require.NoError(t, err)
	require.NotEqual(t, linkRound, physicalRound,
		"the fixture must actually straddle a symlink, or this test proves nothing")

	env := []string{
		"TP_ROUND_DIR=" + linkRound,
		"TP_RUN_DIR=" + linkRun,
		"TP_UNIT_ID=implementer",
		"TP_UNIT_KIND=review-role",
		"TP_UNIT_SEQ=3",
	}

	allowed := []struct{ name, path string }{
		{"physical spelling of the findings file", filepath.Join(physicalRound, "role-implementer.ndjson.part")},
		{"symlinked spelling of the findings file", filepath.Join(linkRound, "role-implementer.ndjson.part")},
		{"symlinked spelling of the escalation record", filepath.Join(linkRun, "3-escalation.json")},
	}
	for _, c := range allowed {
		code, stderr := runRoleWriteHookAt(t, target, env, c.path)
		require.Equal(t, 0, code, "%s must be allowed: %s", c.name, stderr)
	}

	denied := []struct{ name, path string }{
		{"another role's findings file", filepath.Join(physicalRound, "role-tester.ndjson.part")},
		{"the final name rather than the .part", filepath.Join(physicalRound, "role-implementer.ndjson")},
		{"a sibling of the round directory", filepath.Join(target, "rounds", "role-implementer.ndjson.part")},
		{"a path outside the run entirely", filepath.Join(target, "notes.md")},
	}
	for _, c := range denied {
		code, _ := runRoleWriteHookAt(t, target, env, c.path)
		require.Equal(t, 2, code, "%s must still be denied — resolving must not widen the allowlist", c.name)
	}
}

// TestRoleWriteHookAllowsAcrossASymlinkedRootBeforeTheRoundDirExists is the
// case the test above does not reach: it creates the round directory, so the
// physical comparison always had an existing directory to resolve.
//
// Resolving only the immediate parent means the reconciliation is available
// exactly when that parent already exists. Under `tp run` it does — the driver
// creates the round directory in prepare(), before spawnAll() — so this is not
// a live denial. It is a gate defect: the repo's own suite passes or fails on
// whether the checkout path happens to traverse a symlink, which is how it was
// found (green at /Users/..., red at /tmp/...). A test that passes for a reason
// unrelated to the code is not evidence about the code.
func TestRoleWriteHookAllowsAcrossASymlinkedRootBeforeTheRoundDirExists(t *testing.T) {
	target := filepath.Join(t.TempDir(), "real")
	require.NoError(t, os.MkdirAll(target, 0o755))

	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))

	// Deliberately NOT created: this is the whole point of the case.
	linkRound := filepath.Join(link, "rounds", "review-r1")
	physicalRound := filepath.Join(target, "rounds", "review-r1")
	require.NoDirExists(t, physicalRound,
		"the round directory must be absent, or this test proves nothing new")

	env := []string{
		"TP_ROUND_DIR=" + linkRound,
		"TP_RUN_DIR=" + filepath.Join(link, "runs", "01JB0000000000000000000000"),
		"TP_UNIT_ID=implementer",
		"TP_UNIT_KIND=review-role",
		"TP_UNIT_SEQ=3",
	}

	code, stderr := runRoleWriteHookAt(t, target,
		env, filepath.Join(physicalRound, "role-implementer.ndjson.part"))
	require.Equal(t, 0, code,
		"the unit's own findings file is allowed by its physical spelling: %s", stderr)

	// The allowlist must not widen just because the directory is missing.
	code, _ = runRoleWriteHookAt(t, target,
		env, filepath.Join(physicalRound, "role-tester.ndjson.part"))
	require.Equal(t, 2, code, "another role's file is still denied")
}

// TestRoleWriteHookStaysCheapOnADeepMissingPath bounds the cost of the ancestor
// walk the test above needs.
//
// Each step of the walk costs a subshell, so an unbounded walk makes the hook's
// cost linear in path depth. That is not a mere slowdown: the plugin declares a
// 10-second timeout for this hook, and a PreToolUse hook the runtime kills does
// NOT exit 2 — so past the bound an allowlist fails OPEN, which is the one
// direction a fence must never fail. Found by audit round 7, which measured a
// 490-component path at 8.2s alone and 11.0s beside the four concurrent role
// siblings a `tp run` round actually spawns.
//
// The assertion is a RATIO against a shallow path, not a wall-clock deadline,
// and that is the whole design of this test. An absolute deadline measures the
// machine as much as the hook: the unbounded walk cost 0.22s on an idle machine
// and 5.2s with four agents running, so a deadline either passes when it should
// fail or fails when the machine is merely busy. Both halves here are measured
// back to back and share whatever load exists, so the ratio cancels it out.
// Unbounded, deep costs ~6x shallow; bounded, the two are within noise.
func TestRoleWriteHookStaysCheapOnADeepMissingPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "real")
	require.NoError(t, os.MkdirAll(root, 0o755))

	// Single-character components on purpose. The walk is bounded by path
	// DEPTH, but `dirname` refuses a path longer than PATH_MAX and returns
	// empty, which ends the walk after one step — so a fixture built from
	// wider components silently stops measuring the thing it is named for.
	// That is not hypothetical: the first version of this test used five-byte
	// components, came to 2494 bytes, and passed while the defect was live.
	deep := root
	for i := 0; i < 440; i++ {
		deep = filepath.Join(deep, string(rune('a'+i%26)))
	}
	require.Less(t, len(deep), 1000,
		"the fixture must stay under PATH_MAX or dirname ends the walk before it starts")

	env := []string{
		"TP_ROUND_DIR=" + filepath.Join(root, "rounds", "review-r1"),
		"TP_RUN_DIR=" + filepath.Join(root, "runs", "01JB0000000000000000000000"),
		"TP_UNIT_ID=implementer",
		"TP_UNIT_KIND=review-role",
		"TP_UNIT_SEQ=3",
	}

	// The script path is resolved ONCE, outside the timing loop. Going through
	// runRoleWriteHookAt would call repoRoot on every invocation, and that
	// fixed ~80ms swamps the ~100ms the walk costs — measured, and it made the
	// deep case look faster than the shallow one.
	script := filepath.Join(repoRoot(t), "hooks", "pre-tool-use-role-write-allow.sh")

	// Fastest of three: contention only ever adds time, so the minimum is the
	// honest estimate of what each case costs.
	best := func(path string) (time.Duration, int) {
		fastest, code := time.Hour, 0
		for i := 0; i < 3; i++ {
			cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
			cmd.Env = env
			cmd.Dir = root
			cmd.Stdin = strings.NewReader(
				fmt.Sprintf(`{"tool_name":"Write","tool_input":{"file_path":%q}}`, path))
			start := time.Now()
			err := cmd.Run()
			elapsed := time.Since(start)
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				code = exitErr.ExitCode()
			} else {
				require.NoError(t, err, "the hook must exit rather than fail to start")
				code = 0
			}
			if elapsed < fastest {
				fastest = elapsed
			}
		}
		return fastest, code
	}

	deepCost, deepCode := best(filepath.Join(deep, "role-implementer.ndjson.part"))
	// Same shape, same verdict, one level of missing directory instead of 490.
	shallowCost, _ := best(filepath.Join(root, "rounds", "review-r1", "role-tester.ndjson.part"))

	t.Logf("MEASURED deep=%v shallow=%v", deepCost, shallowCost)
	require.Equal(t, 2, deepCode, "a path outside the unit is denied however deep it is")
	require.Less(t, deepCost, 3*shallowCost,
		"the ancestor walk must be bounded (deep %v vs shallow %v): an unbounded one lets the runtime kill the hook past its declared timeout, and a killed PreToolUse hook denies nothing",
		deepCost, shallowCost)
}

// TestRoleWriteHookReadsTheMCPPathArgument pins the half of the v0.35.2 fence
// repair that a matcher change alone would not deliver.
//
// The hook extracts the target from `file_path` and `notebook_path`, the names
// the four native editors use. codedbpro — the toolset this repository routes
// every agent to, because the native editors are blocked at the user level —
// names it `file`. So before this repair the hook found no path in an MCP
// payload, and a payload with no path is allowed: adding the MCP tools to the
// matcher would have sent them to a hook that still waved them through.
//
// That is the shape this project keeps having to relearn — a change that looks
// like a fix and discriminates nothing — so the two halves get one test each
// rather than a single test that could pass on either.
func TestRoleWriteHookReadsTheMCPPathArgument(t *testing.T) {
	root := filepath.Join(t.TempDir(), "real")
	round := filepath.Join(root, "rounds", "review-r1")
	run := filepath.Join(root, "runs", "01JB0000000000000000000000")
	require.NoError(t, os.MkdirAll(round, 0o755))
	require.NoError(t, os.MkdirAll(run, 0o755))

	env := []string{
		"TP_ROUND_DIR=" + round,
		"TP_RUN_DIR=" + run,
		"TP_UNIT_ID=implementer",
		"TP_UNIT_KIND=review-role",
		"TP_UNIT_SEQ=3",
	}
	script := filepath.Join(repoRoot(t), "hooks", "pre-tool-use-role-write-allow.sh")

	// The payload shape is codedbpro's: the target is `file`, not `file_path`.
	probe := func(tool, path string) int {
		cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
		cmd.Env = env
		cmd.Dir = root
		cmd.Stdin = strings.NewReader(
			fmt.Sprintf(`{"tool_name":%q,"tool_input":{"file":%q}}`, tool, path))
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			require.ErrorAs(t, err, &exitErr, "the hook must exit rather than fail to start")
			return exitErr.ExitCode()
		}
		return 0
	}

	own := filepath.Join(round, "role-implementer.ndjson.part")
	assert.Equal(t, 0, probe("mcp__codedbpro__create", own),
		"the unit's own findings file is its one permitted write, whichever tool spells the path")

	for _, tool := range []string{
		"mcp__codedbpro__create", "mcp__codedbpro__edit",
		"mcp__codedbpro__patch", "mcp__codedbpro__replace",
	} {
		assert.Equal(t, 2, probe(tool, filepath.Join(repoRoot(t), "internal", "cli", "audit.go")),
			"%s must not reach a source file: the fence reads `file`, not only `file_path`", tool)
		assert.Equal(t, 2, probe(tool, filepath.Join(round, "role-tester.ndjson.part")),
			"%s must not reach another role's findings file", tool)
	}
}

// runRoleWriteHookAt runs the shipped hook with cwd at dir and returns its exit
// code and stderr. The package's other hook runner pins cwd to the repo root,
// which is exactly the variable under test here, so this one takes it.
func runRoleWriteHookAt(t *testing.T, dir string, env []string, path string) (exitCode int, stderr string) {
	t.Helper()

	script := filepath.Join(repoRoot(t), "hooks", "pre-tool-use-role-write-allow.sh")
	cmd := exec.Command(script) //nolint:gosec // a fixed path inside the repo under test
	cmd.Env = env
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(
		fmt.Sprintf(`{"tool_name":"Write","tool_input":{"file_path":%q}}`, path))

	var errBuf strings.Builder
	cmd.Stderr = &errBuf

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		require.ErrorAs(t, err, &exitErr, "the hook must exit rather than fail to start")
		return exitErr.ExitCode(), errBuf.String()
	}
	return 0, errBuf.String()
}
