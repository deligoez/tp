package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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
