package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keepListEntries(t *testing.T, dir string) []map[string]any {
	t.Helper()
	out, _, code := runTP(t, dir, "keep", "--list")
	require.Equal(t, 0, code)
	var entries []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	return entries
}

func TestKeep_AddOverwriteRemoveList(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	out, _, code := runTP(t, dir, "keep", "--list")
	require.Equal(t, 0, code)
	assert.Equal(t, "[]", strings.TrimSpace(out), "an empty keep-list prints [] not null")

	_, _, code = runTP(t, dir, "keep", "a.txt", "local config")
	require.Equal(t, 0, code)
	entries := keepListEntries(t, dir)
	require.Len(t, entries, 1)
	assert.Equal(t, "a.txt", entries[0]["path"])
	assert.Equal(t, "local config", entries[0]["reason"])

	_, _, code = runTP(t, dir, "keep", "a.txt", "new reason")
	require.Equal(t, 0, code)
	entries = keepListEntries(t, dir)
	require.Len(t, entries, 1, "a repeated path overwrites rather than appends")
	assert.Equal(t, "new reason", entries[0]["reason"])

	_, _, code = runTP(t, dir, "keep", "--remove", "a.txt")
	require.Equal(t, 0, code)
	entries = keepListEntries(t, dir)
	assert.Empty(t, entries)
}

func TestKeep_RemoveAbsentIsNoOp(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	_, _, code := runTP(t, dir, "keep", "--remove", "nope.txt")
	assert.Equal(t, 0, code, "removing an absent path exits 0")
}

func TestKeep_MissingReasonExit2(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	_, _, code := runTP(t, dir, "keep", "a.txt")
	assert.Equal(t, 2, code)
}

func TestKeep_MalformedGlobExit2(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	_, _, code := runTP(t, dir, "keep", "[bad", "reason")
	assert.Equal(t, 2, code)
}

func TestKeep_SubdirStoredRepoRootRelative(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	_, _, code := runTP(t, sub, "keep", "x.log", "log")
	require.Equal(t, 0, code)
	entries := keepListEntries(t, dir)
	require.Len(t, entries, 1)
	assert.Equal(t, "sub/x.log", entries[0]["path"], "a path given from a subdirectory is stored repo-root-relative")
}

func TestKeep_WritesOnlyLocalConfig(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	initGitRepo(t, dir)

	taskFile := filepath.Join(dir, "spec.tasks.json")
	before, err := os.ReadFile(taskFile)
	require.NoError(t, err)

	_, _, code = runTP(t, dir, "keep", "a.txt", "kept")
	require.Equal(t, 0, code)

	after, err := os.ReadFile(taskFile)
	require.NoError(t, err)
	assert.Equal(t, before, after, "tp keep writes no entry into the task file")

	_, statErr := os.Stat(filepath.Join(dir, ".tp-review"))
	assert.True(t, os.IsNotExist(statErr), "tp keep writes no .tp-review file")

	gitignore, err := os.ReadFile(filepath.Join(dir, ".tp", ".gitignore"))
	require.NoError(t, err)
	assert.Contains(t, string(gitignore), "local.json", ".tp/local.json stays git-ignored")
}
