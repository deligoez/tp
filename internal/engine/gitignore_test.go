package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runArtifactIgnoreEntries are the four .tp/.gitignore entries that cover the
// run artifacts §3.5 names: the run state file, the per-run directory, the
// per-cycle round directory and the last-failure record. They are written
// relative to .tp/, which is where the file lives.
var runArtifactIgnoreEntries = []string{"run-*.json", "runs/", "rounds/", "last_failure-*.json"}

// TestGitignore_NoTPActive verifies the repo-root .gitignore no longer lists the
// removed .tp-active marker (§11.2).
func TestGitignore_NoTPActive(t *testing.T) {
	root := locateRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	require.NoError(t, err, "repo-root .gitignore is readable")
	for line := range strings.SplitSeq(string(data), "\n") {
		assert.NotEqual(t, ".tp-active", strings.TrimSpace(line), ".tp-active must not be gitignored after v0.25.0")
	}
}

// TestEnsureTPGitignore_IgnoresTheRunArtifacts is §10.2 test 15: the four run
// artifacts §3.5 names — .tp/run-<base>.json, .tp/runs/, .tp/rounds/ and
// .tp/last_failure-<base>.json — must be git-ignored by the .tp/.gitignore tp
// writes.
//
// It asks git rather than asserting the entry strings, because a pattern that
// is present and matches nothing reads exactly like one that works. The
// negative half is the half that discriminates: an entry broad enough to
// swallow .tp/config.json or the role corpus would satisfy every positive
// assertion while quietly untracking the project's own committed state.
func TestEnsureTPGitignore_IgnoresTheRunArtifacts(t *testing.T) {
	root := t.TempDir()
	gitInTempRepo(t, root, "init")
	tpDir := filepath.Join(root, ".tp")
	require.NoError(t, os.MkdirAll(tpDir, 0o755))
	require.NoError(t, EnsureTPGitignore(tpDir))

	ignored := []string{
		".tp/run-spec.json",
		".tp/runs/01J0000000000000000000000A/3-implement-task-x.jsonl",
		".tp/rounds/spec/review-r1/role-tester.ndjson",
		".tp/last_failure-spec.json",
	}
	for _, path := range ignored {
		assert.True(t, gitIgnores(t, root, path), "%s must be git-ignored", path)
	}

	tracked := []string{".tp/config.json", ".tp/reviewers/implementer.json", ".tp/.gitignore"}
	for _, path := range tracked {
		assert.False(t, gitIgnores(t, root, path), "%s must stay committable", path)
	}
}

// TestShippedTPGitignore_CoversTheRunArtifacts is the other half of test 15:
// tp's own committed .tp/.gitignore carries the same four entries, so the
// repository that develops tp does not accumulate untracked run artifacts.
func TestShippedTPGitignore_CoversTheRunArtifacts(t *testing.T) {
	root := locateRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, ".tp", ".gitignore"))
	require.NoError(t, err, "the repo ships a .tp/.gitignore")

	present := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		present[strings.TrimSpace(line)] = true
	}
	for _, entry := range runArtifactIgnoreEntries {
		assert.True(t, present[entry], "the shipped .tp/.gitignore must carry %q", entry)
	}
}

// gitInTempRepo runs a git command in a throwaway repository and fails the test
// on any error.
func gitInTempRepo(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// gitIgnores reports whether git ignores path within dir. git check-ignore -q
// exits 0 when the path is ignored and 1 when it is not; any other exit status
// is a real failure and fails the test rather than reading as "not ignored".
func gitIgnores(t *testing.T, dir, path string) bool {
	t.Helper()
	cmd := exec.Command("git", "check-ignore", "-q", path)
	cmd.Dir = dir
	err := cmd.Run()
	if err == nil {
		return true
	}
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr, "git check-ignore %s", path)
	require.Equal(t, 1, exitErr.ExitCode(), "git check-ignore %s", path)
	return false
}

// locateRepoRoot walks up from the test working directory until it finds go.mod.
func locateRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found walking up from the test directory")
		}
		dir = parent
	}
}
