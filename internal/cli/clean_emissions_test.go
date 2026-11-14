package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCleanEmissions runs scripts/clean-emissions.sh in dir as a dry run and
// returns its output.
func runCleanEmissions(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command(filepath.Join(repoRootDir(t), "scripts", "clean-emissions.sh"), "--root", filepath.Join("spec", ".tp-review"))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)
	return string(out)
}

// writeOld writes a file and backdates it past the script's age guard.
func writeOld(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))
	old := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
}

// TestCleanEmissionsKnowsTheInboxes is scripts/clean-emissions.sh's half of
// spec/backlog/ground-scratch-in-state-dir.md and role-files-in-state-dir.md:
// an inbox file (ground's scratch file, a review or audit role file) whose
// round was never recorded is a removable emission artifact, and a leftover
// whose round IS recorded is removable too — --record removes it, so one
// still there is debris, not the round's evidence.
func TestCleanEmissionsKnowsTheInboxes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	initGitRepo(t, dir)
	state := filepath.Join(dir, "spec", ".tp-review", "a")
	for _, name := range []string{
		filepath.Join("ground-inbox", "round-1.ndjson"), filepath.Join("ground-inbox", "round-2.ndjson"),
		filepath.Join("review-inbox", "round-1-tester.ndjson"), filepath.Join("review-inbox", "round-2-tester.ndjson"),
		filepath.Join("audit-inbox", "round-1-spec-coverage.ndjson"),
	} {
		writeOld(t, filepath.Join(state, name))
	}
	writeOld(t, filepath.Join(state, "ground-round-1.ndjson"))
	writeOld(t, filepath.Join(state, "review-round-1.ndjson"))

	out := runCleanEmissions(t, dir)
	for _, name := range []string{
		filepath.Join("ground-inbox", "round-1.ndjson"), filepath.Join("ground-inbox", "round-2.ndjson"),
		filepath.Join("review-inbox", "round-1-tester.ndjson"), filepath.Join("review-inbox", "round-2-tester.ndjson"),
		filepath.Join("audit-inbox", "round-1-spec-coverage.ndjson"),
	} {
		assert.Contains(t, out, name, "an old inbox file is removable, its round recorded or not")
	}
	assert.Contains(t, out, "recorded inbox file", "each path carries its reason")
	assert.Contains(t, out, "unrecorded inbox file")
}

// TestCleanEmissionsKnowsTheAuditPools: the pools an audit emission keeps
// beside its role files (engine.AuditPoolsPath) are judged like them — --record
// removes them, so an old one is removable whether its round is recorded or
// not.
func TestCleanEmissionsKnowsTheAuditPools(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	initGitRepo(t, dir)
	state := filepath.Join(dir, "spec", ".tp-review", "a")
	writeOld(t, filepath.Join(state, "audit-inbox", "round-1.pools.json"))
	writeOld(t, filepath.Join(state, "audit-inbox", "round-2.pools.json"))
	writeOld(t, filepath.Join(state, "audit-round-1.ndjson"))

	out := runCleanEmissions(t, dir)
	assert.Contains(t, out, filepath.Join("audit-inbox", "round-1.pools.json")+"  recorded inbox file")
	assert.Contains(t, out, filepath.Join("audit-inbox", "round-2.pools.json")+"  unrecorded inbox file")
}

// TestCleanEmissionsKnowsTheReviewOwedRoles: the roles a review emission owes
// (engine.ReviewOwedRolesPath) and the rows --record-inbox merged from them are
// inbox files too, judged like the role files beside them.
func TestCleanEmissionsKnowsTheReviewOwedRoles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	initGitRepo(t, dir)
	state := filepath.Join(dir, "spec", ".tp-review", "a")
	writeOld(t, filepath.Join(state, "review-inbox", "round-1.roles.json"))
	writeOld(t, filepath.Join(state, "review-inbox", "round-1.merged.ndjson"))
	writeOld(t, filepath.Join(state, "review-round-1.ndjson"))

	out := runCleanEmissions(t, dir)
	assert.Contains(t, out, filepath.Join("review-inbox", "round-1.roles.json")+"  recorded inbox file")
	assert.Contains(t, out, filepath.Join("review-inbox", "round-1.merged.ndjson")+"  recorded inbox file")
}

// TestCleanEmissionsReportsOldLayoutRoundDirectories: a round directory under
// the pre-1.12.0 layout, .tp/rounds/<base>/<phase>-r<N>, for a task file that
// is not at the root is no longer read by the driver
// (TestARoundInFlightAcrossTheUpgradeIsRespawned), so the script reports it.
// A root task file's directory did not move and is kept.
func TestCleanEmissionsReportsOldLayoutRoundDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	initGitRepo(t, dir)
	writeOld(t, filepath.Join(dir, ".tp", "rounds", "a", "review-r1", "role-tester.ndjson"))
	writeOld(t, filepath.Join(dir, ".tp", "rounds", "b", "review-r1", "role-tester.ndjson"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.tasks.json"), []byte("{}"), 0o600))
	writeOld(t, filepath.Join(dir, ".tp", "rounds", "x", "a", "review-r1", "role-tester.ndjson"))

	out := runCleanEmissions(t, dir)
	assert.Contains(t, out, filepath.Join(".tp", "rounds", "a", "review-r1")+"  old-layout round dir", "an orphaned old-layout round directory is reported, with its reason")
	assert.NotContains(t, out, filepath.Join(".tp", "rounds", "b", "review-r1"), "a root task file's round directory is kept")
	assert.NotContains(t, out, filepath.Join(".tp", "rounds", "x", "a"), "and so is a new-layout one")
}
