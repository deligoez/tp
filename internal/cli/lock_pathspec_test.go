package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCommitLeavesForeignLockFilesAlone guards the most destructive defect this
// audit found. tp commit staged the tree and then ran
// git rm --cached --ignore-unmatch -- "*.lock" to drop any flock file it had
// picked up — but a git pathspec matches across directories, so that one
// invocation recorded the DELETION of yarn.lock, Gemfile.lock and every other
// lock file in the repository. The files stayed on disk, so nothing looked
// wrong until someone cloned. It survived this long because tp's own
// commit_strategy is hc, which refuses tp commit entirely.
func TestCommitLeavesForeignLockFilesAlone(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## 1. Thing\n\n1. Do it.\n"), 0o600))

	foreign := []string{"yarn.lock", "Gemfile.lock", filepath.Join("sub", "Cargo.lock")}
	for _, name := range foreign {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o600))
	}

	gitOut(t, dir, "init", "-q", ".")
	// Repo-local, not per-command: `tp commit` runs git in a subprocess of its
	// own, so a -c on the fixture's own commit does not reach it. Configured
	// only on the fixture, this passed on a developer machine — which has a
	// global identity — and failed on CI, which does not.
	gitOut(t, dir, "config", "user.email", "t@t")
	gitOut(t, dir, "config", "user.name", "t")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-qm", "init")

	_, stderr, code := runTP(t, dir, "init", specPath)
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["## 1. Thing"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim: %s", stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "out.txt"), []byte("work\n"), 0o600))

	_, stderr, code = runTP(t, dir, "commit", "t1", "implemented")
	require.Equal(t, 0, code, "commit: %s", stderr)

	deleted := gitOut(t, dir, "show", "--name-status", "--format=", "HEAD")
	for _, name := range foreign {
		assert.NotContains(t, deleted, name,
			"tp commit must not record a deletion of %s — the pathspec is for tp's own locks", name)
	}

	tracked := gitOut(t, dir, "ls-files")
	for _, name := range foreign {
		assert.Contains(t, tracked, filepath.ToSlash(name), "%s stays tracked", name)
	}
	assert.NotContains(t, tracked, ".tp/locks",
		"tp's own lock file is still kept out of the index")
}

// TestCommitDropsAlreadyTrackedLockFiles is the half the first version of this
// guard could not see. Asserting that .tp/locks is absent from the index proves
// nothing about the pathspec: .tp/.gitignore keeps it unstaged on its own, so
// the assertion passed even against a tpLockPathspecs() returning nonsense.
// Force-adding the locks first makes the pathspec the only thing that can drop
// them — and it catches the two spellings that looked right and matched nothing:
// a glob pathspec must match the whole path (so the directory form needs the
// trailing /**), and without :(top) the whole thing resolves against the
// current directory rather than the repo root.
func TestCommitDropsAlreadyTrackedLockFiles(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".tp", "locks"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(nested, ".tp", "locks"), 0o755))

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## 1. Thing\n\n1. Do it.\n"), 0o600))

	locks := []string{
		filepath.Join(".tp", "locks", "root.lock"),
		filepath.Join("sub", ".tp", "locks", "nested.lock"),
		"spec.tasks.json.lock",
		filepath.Join("sub", "other.tasks.json.lock"),
	}
	for _, name := range locks {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("stale\n"), 0o600))
	}

	gitOut(t, dir, "init", "-q", ".")
	gitOut(t, dir, "config", "user.email", "t@t")
	gitOut(t, dir, "config", "user.name", "t")
	// -f: the ignore file would otherwise keep them out, which is exactly the
	// masking that made the first guard vacuous.
	gitOut(t, dir, "add", "-A", "-f")
	gitOut(t, dir, "commit", "-qm", "init")
	require.Contains(t, gitOut(t, dir, "ls-files"), ".tp/locks/root.lock", "the fixture starts with locks tracked")

	_, stderr, code := runTP(t, dir, "init", specPath)
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["## 1. Thing"],"depends_on":[]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	_, stderr, code = runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim: %s", stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "out.txt"), []byte("work\n"), 0o600))

	// From the SUBDIRECTORY: pathspecs resolve against the current directory,
	// so this is where a missing :(top) shows up.
	_, stderr, code = runTPIn(t, nested, "--file", filepath.Join(dir, "spec.tasks.json"), "commit", "t1", "implemented")
	require.Equal(t, 0, code, "commit: %s", stderr)

	tracked := gitOut(t, dir, "ls-files")
	for _, name := range locks {
		assert.NotContains(t, tracked, filepath.ToSlash(name),
			"%s must be dropped from the index by the lock pathspec", name)
	}
}

// TestLockDirIsGitIgnoredWithoutInit: the lock file outlives the lock now, so
// the call that creates .tp/locks has to ignore it too. Only tp init and the
// local.json writers did that, which was survivable while the file was
// unlinked on release — after the lock fix, one locked write in a project that
// never ran tp init left an untracked file behind, and tp resume reported an
// unexplained change the agent could not clear by committing.
func TestLockDirIsGitIgnoredWithoutInit(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\n## 1. Thing\n\n1. Do it.\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\n"), 0o600))

	gitOut(t, dir, "init", "-q", ".")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init")

	results := filepath.Join(dir, "r.ndjson")
	require.NoError(t, os.WriteFile(results, []byte(
		`{"item_id":"list-0-1","status":"PASS","evidence_file":"a.go","evidence_lines":"1","category":null,"severity":null,"notes":"","role":"spec-coverage","location":"§1"}`+"\n"), 0o600))

	// No tp init: the lock directory is created by the locked write itself.
	_, stderr, code := runTP(t, dir, "audit", specPath, "--record", results)
	require.Equal(t, 0, code, "record: %s", stderr)

	require.FileExists(t, filepath.Join(dir, ".tp", ".gitignore"),
		"the write that creates .tp/locks also writes the ignore file")

	status := gitOut(t, dir, "status", "--porcelain", "-uall")
	for line := range strings.SplitSeq(status, "\n") {
		assert.NotContains(t, line, filepath.Join(".tp", "locks"),
			"no lock file may show up as an unexplained working-tree change")
	}
}

// runTPIn runs tp with its working directory set to workdir, so a path-sensitive behaviour like a git pathspec can be exercised from
// somewhere other than the repo root.
func runTPIn(t *testing.T, workdir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, append([]string{"--json"}, args...)...)
	cmd.Dir = workdir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TP_HC=0")

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if err != nil {
		t.Fatalf("unexpected error running tp: %v", err)
	}
	return outBuf.String(), errBuf.String(), exitCode
}
