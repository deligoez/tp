package cli_test

import (
	"os"
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
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "init")

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
	for _, line := range strings.Split(status, "\n") {
		assert.NotContains(t, line, filepath.Join(".tp", "locks"),
			"no lock file may show up as an unexplained working-tree change")
	}
}
