package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/fakerunner"
)

// lockedCorpusProject is a project with one recorded review round and one
// recorded audit round, each over a user role file, after which both role
// files are made unreadable. Every path that hashes a phase's corpus then
// meets a read error instead of a hash.
func lockedCorpusProject(t *testing.T) string {
	t.Helper()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the read error cannot be produced")
	}
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))

	roles := make([]string, 0, 2)
	for _, phase := range []string{"reviewers", "auditors"} {
		phaseDir := filepath.Join(dir, ".tp", phase)
		require.NoError(t, os.MkdirAll(phaseDir, 0o755))
		role := filepath.Join(phaseDir, "r.json")
		require.NoError(t, os.WriteFile(role, []byte(`{"id":"r","title":"T","instructions":"I"}`), 0o600))
		roles = append(roles, role)
	}
	for _, cmd := range []string{"review", "audit"} {
		_, stderr, code := runTP(t, dir, cmd, "spec.md", "--record", rec)
		require.Equal(t, 0, code, "%s round 1: %s", cmd, stderr)
	}
	for _, role := range roles {
		require.NoError(t, os.Chmod(role, 0o000))
	}
	return dir
}

// A role file that cannot be read is a file error on every path that stamps or
// compares a corpus hash: --record must not store a round with an empty hash,
// and --status must not report roles_stale against one.
func TestRolesHashReadErrorExitsInsteadOfRecording(t *testing.T) {
	t.Parallel()
	dir := lockedCorpusProject(t)

	for _, args := range [][]string{
		{"review", "spec.md", "--record", "empty.ndjson"},
		{"audit", "spec.md", "--record", "empty.ndjson"},
		{"review", "spec.md", "--status"},
		{"audit", "spec.md", "--status"},
	} {
		stdout, stderr, code := runTP(t, dir, args...)
		assert.Equal(t, 3, code, "%v exits with the corpus read error: %s%s", args, stdout, stderr)
		assert.Contains(t, stderr, "cannot read role file", "%v names the read error", args)
	}

	review, audit := readRoundHashes(t, dir)
	assert.Len(t, review, 1, "the refused review --record stored no round")
	assert.Len(t, audit, 1, "the refused audit --record stored no round")
}

// `tp run --status` reports the run even when the audit signals cannot be
// computed (addAuditSignals omits them on any failure). An unreadable auditor
// corpus is such a failure: the signals are omitted, not computed against an
// empty hash.
func TestRunStatusOmitsAuditSignalsWhenTheCorpusCannotBeRead(t *testing.T) {
	t.Parallel()
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the read error cannot be produced")
	}
	dir := runProject(t)
	env := append(seamEnv(t), fakerunner.EnvDurable+"=1")
	_, stderr, code := runTPEnv(t, dir, env, "run")
	require.Equal(t, 4, code, "the run stops in the audit phase: %s", stderr)

	stdout, stderr, code := runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status: %s", stderr)
	require.Contains(t, decodeStatus(t, stdout), "role_streaks",
		"with a readable corpus the audit-phase stop carries the signals")

	auditors := filepath.Join(dir, ".tp", "auditors")
	require.NoError(t, os.MkdirAll(auditors, 0o755))
	role := filepath.Join(auditors, "r.json")
	require.NoError(t, os.WriteFile(role, []byte(`{"id":"r","title":"T","instructions":"I"}`), 0o600))
	require.NoError(t, os.Chmod(role, 0o000))

	stdout, stderr, code = runTP(t, dir, "run", "--status")
	require.Equal(t, 0, code, "--status still reports the run: %s", stderr)
	st := decodeStatus(t, stdout)
	assert.NotContains(t, st, "role_streaks", "no signal is computed against an unread corpus")
	assert.NotContains(t, st, "spec_coverage_clean_rounds")
	assert.NotContains(t, st, "divergence")
}
