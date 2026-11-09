package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitRefusingNthPath returns a PATH entry whose git refuses the nth and every
// later invocation of subcommand and hands everything else to the real git.
func gitRefusingNthPath(t *testing.T, subcommand string, n int) string {
	t.Helper()
	realGit, err := exec.LookPath("git")
	require.NoError(t, err)
	shimDir := t.TempDir()
	counter := filepath.Join(shimDir, "calls")
	shim := "#!/bin/sh\nif [ \"$1\" = " + subcommand + " ]; then\n" +
		"  c=$(cat '" + counter + "' 2>/dev/null || echo 0); c=$((c+1)); echo $c > '" + counter + "'\n" +
		"  if [ $c -ge " + strconv.Itoa(n) + " ]; then echo 'fatal: shim refuses " + subcommand + "' >&2; exit 128; fi\n" +
		"fi\nexec '" + realGit + "' \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o700)) //nolint:gosec // the shim must be executable
	return "PATH=" + shimDir + string(os.PathListSeparator) + os.Getenv("PATH")
}

// TestCommit_AFailedStagedCheckExitsThree: `git diff --cached --quiet` exits 1
// when changes are staged and 0 when none are; any other exit is git's own
// failure, which tp commit used to read as "changes staged" and commit past.
// It exits 3 with git's message, commits nothing, and names no committer
// identity, which reading the index does not involve.
func TestCommit_AFailedStagedCheckExitsThree(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o600))

	stdout, stderr, code := runTPEnv(t, dir, []string{gitRefusingNthPath(t, "diff", 1)}, "commit", "t1", "t1 acceptance met completely")
	require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Contains(t, stderr, "shim refuses diff")
	assert.NotContains(t, stdout+stderr, "user.name", "no committer identity is involved in reading the index")

	log := exec.Command("git", "log", "--format=%s")
	log.Dir = dir
	out, err := log.Output()
	require.NoError(t, err)
	assert.NotContains(t, string(out), "feat(t1)", "nothing is committed")
}

// TestCommit_AFailedFollowUpClosureCommitExitsThree: with every commit after
// the implementation commit refused, the amend fails and so does the follow-up
// chore(tp) commit, leaving the closure staged but in no commit. tp commit used
// to only warn and exit 0; it exits 3 and its envelope names the landed
// commit and the exact recovery, as a failed closure stage does.
func TestCommit_AFailedFollowUpClosureCommitExitsThree(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o600))

	stdout, stderr, code := runTPEnv(t, dir, []string{gitRefusingNthPath(t, "commit", 2)}, "commit", "t1", "t1 acceptance met completely")
	require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Contains(t, stderr, "shim refuses commit")

	var envelope map[string]any
	for line := range strings.SplitSeq(stderr, "\n") {
		if strings.HasPrefix(line, "{") {
			require.NoError(t, json.Unmarshal([]byte(line), &envelope))
		}
	}
	require.NotNil(t, envelope, "no JSON error envelope on stderr: %s", stderr)
	head := exec.Command("git", "rev-parse", "--short", "HEAD")
	head.Dir = dir
	out, err := head.Output()
	require.NoError(t, err)
	assert.Equal(t, strings.TrimSpace(string(out)), envelope["commit_sha"])
	hint, _ := envelope["hint"].(string)
	assert.Contains(t, hint, `git commit -m "chore(tp): record t1 closure"`)
	assert.Contains(t, hint, "do not re-run tp commit")
}
