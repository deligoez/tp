package cli

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearNoticedKeys drops the once-per-condition suppression set. A CLI process
// advises once and exits, but these tests drive several runs in one process,
// where a leftover key would silence the very advisory under test.
func clearNoticedKeys() {
	noticedMu.Lock()
	noticedKeys = map[string]bool{}
	noticedMu.Unlock()
}

// rejectedGitError returns a real *exec.ExitError from a git invocation git
// refuses, which is how the reported failure arrives: git answers a rejected
// invocation with its whole usage text on stderr.
func rejectedGitError(t *testing.T) *exec.ExitError {
	t.Helper()
	cmd := exec.Command("git", "diff", "--tp-not-a-real-option")
	cmd.Dir = t.TempDir()
	_, err := cmd.Output()
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "the fixture needs git to reject the invocation")
	return exitErr
}

// TestWarnGitFailure_BoundedAndOncePerProbe pins both halves of the advisory's
// cost. warnGitFailure travels output.Notice, which JSON mode does NOT suppress
// — that visibility is the point, and it is also why the advisory has to be
// cheap. Uncapped, git's usage dump put ~30KB on the stderr of a run that
// exits 0; unkeyed, one broken condition repeated that dump once per selection
// range, since auditDiffStats and auditDeletedFiles each run their probe over
// every range auditDiffRanges yields.
func TestWarnGitFailure_BoundedAndOncePerProbe(t *testing.T) {
	exitErr := rejectedGitError(t)
	require.Greater(t, len(exitErr.Stderr), 4000,
		"the fixture must reproduce git's multi-line usage dump, or the cap is untested")

	clearNoticedKeys()
	stderr := captureCLIStderr(t, func() {
		// The same probe over three ranges, then a second probe: exactly what
		// one audit does when git cannot answer at all.
		warnGitFailure("diff --numstat", exitErr, "diff", "--numstat")
		warnGitFailure("diff --numstat", exitErr, "diff", "--numstat", "--cached")
		warnGitFailure("diff --numstat", exitErr, "diff", "--numstat", "v1...HEAD")
		warnGitFailure("diff --name-only --diff-filter=D", exitErr, "diff", "--name-only", "--diff-filter=D")
	})

	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	assert.Len(t, lines, 2, "one advisory per probe, not one per range: %q", stderr)
	assert.Contains(t, lines[0], "git diff --numstat failed")
	assert.Contains(t, lines[1], "git diff --name-only --diff-filter=D failed")
	for _, line := range lines {
		assert.LessOrEqual(t, len(line), noticeDetailCap+120,
			"an advisory must stay one bounded line, got %d bytes: %q", len(line), line)
	}
	assert.Less(t, len(stderr), 600, "the whole advisory budget for one failed condition")
	// The dump's later lines are what made this 30KB; the first line is the
	// diagnosis and is all the advisory keeps.
	assert.NotContains(t, stderr, "--diff-algorithm",
		"git's usage dump must not reach stderr past its first line")
}

// TestFirstLineCapped keeps the two reductions the advisory relies on honest:
// a multi-line detail loses everything after its first line, and a single
// over-long line is truncated rather than passed through whole.
func TestFirstLineCapped(t *testing.T) {
	assert.Equal(t, "fatal: not a git repository",
		firstLineCapped("fatal: not a git repository\nusage: git diff\n  more\n"))

	long := strings.Repeat("x", noticeDetailCap+50)
	got := firstLineCapped(long)
	assert.Equal(t, strings.Repeat("x", noticeDetailCap)+"...", got)
	assert.Len(t, got, noticeDetailCap+3)
}

// TestNoticeOnce_SuppressesRepeatsPerKey: the key is the condition, and two
// conditions must never collapse into one line just because the first one
// already spoke.
func TestNoticeOnce_SuppressesRepeatsPerKey(t *testing.T) {
	clearNoticedKeys()
	stderr := captureCLIStderr(t, func() {
		noticeOnce("k1", "first condition")
		noticeOnce("k1", "first condition again")
		noticeOnce("k2", "second condition")
	})
	assert.Contains(t, stderr, "first condition\n")
	assert.NotContains(t, stderr, "first condition again")
	assert.Contains(t, stderr, "second condition")
}
