package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// handFile writes a findings file holding one row of the first emitted role,
// as a merged file missing every other role would.
func handFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "merged.ndjson")
	row := `{"severity":"low","finding":"a gap","location":"§1","evidence":"line 3","role":"architect"}` + "\n"
	require.NoError(t, os.WriteFile(path, []byte(row), 0o600))
	return path
}

// TestReviewRecord_RefusesAHandFileOverAnEmittedRound: a round whose emission
// stored the roles it owes is recorded only from its inbox. A hand file cannot
// tell a role that found nothing from one that never ran, and a row deleted
// from it leaves without a disposition, so --record <file> over such a round
// exits 2 naming --record-inbox and records nothing.
func TestReviewRecord_RefusesAHandFileOverAnEmittedRound(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", handFile(t, dir))
	require.Equal(t, 3, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "--record-inbox")
	assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"), "nothing is recorded")
}

// TestReviewRecord_ATPRoundThatRewritesNothingOpensNothing: only a TP_ROUND
// naming a recorded round re-records. Any other value records the next round,
// the very round the rule protects, so it is refused like no value at all.
func TestReviewRecord_ATPRoundThatRewritesNothingOpensNothing(t *testing.T) {
	t.Parallel()
	for _, round := range []string{"99", "03", "0", "abc"} {
		dir, _ := inboxSpec(t)
		emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
		_, stderr, code := runTPEnv(t, dir, []string{"TP_ROUND=" + round}, "review", "spec.md", "--record", handFile(t, dir))
		assert.Equal(t, 3, code, "TP_ROUND=%s records the emitted round, so it is refused; stderr: %s", round, stderr)
		assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"), "TP_ROUND=%s", round)
	}
}

// TestReviewEmission_WithoutItsRolesListLeavesNoSnapshot: an emission that
// cannot store the roles it owes fails before its snapshot is written, so the
// round does not read as emitted before v1.20.0, which --record <file> records.
func TestReviewEmission_WithoutItsRolesListLeavesNoSnapshot(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	blocked := filepath.Join(dir, ".tp-review", "spec", "review-inbox", "round-1.roles.json")
	require.NoError(t, os.MkdirAll(blocked, 0o750), "a directory where the list goes makes its write fail")

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 3, code, "stderr: %s", stderr)
	assert.NoFileExists(t, filepath.Join(dir, ".tp-review", "spec", "snapshot-round-1.md"))
	_, _, code = runTP(t, dir, "review", "spec.md", "--record", handFile(t, dir))
	assert.Equal(t, 0, code, "with no snapshot the round reads as never emitted, which a hand file may record")
}

// TestReviewRecordInbox_ReadsTheRolesARunCollected: a tp run that stopped
// after its role units and before its record unit leaves the role files in
// the run's round directory, not in the inbox. --record-inbox finds them
// there, so the operator can record the round by hand.
func TestReviewRecordInbox_ReadsTheRolesARunCollected(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	root := engine.CanonicalProjectRoot(dir)
	roundDir := engine.RoundDir(root, filepath.Join(root, "spec.tasks.json"), engine.PhaseReview, 1)
	require.NoError(t, os.MkdirAll(roundDir, 0o750))
	for _, pm := range prompts {
		require.NoError(t, os.WriteFile(engine.RoleFindingsPath(roundDir, pm["role"].(string)), nil, 0o600))
	}

	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record-inbox")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.FileExists(t, filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
}

// TestReviewRecord_TheDriverRecordsOnlyWithEveryRolesFile: under tp run the
// record unit merges the role files in $TP_ROUND_DIR and records the merged
// file. The variable alone opens nothing: with one owed role's file missing
// the record is refused as by hand, and with every file present it records.
func TestReviewRecord_TheDriverRecordsOnlyWithEveryRolesFile(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	prompts := emittedPrompts(t, dir, noRoundDir(), "review", "spec.md")
	roundDir := filepath.Join(dir, "round-dir")
	require.NoError(t, os.MkdirAll(roundDir, 0o750))
	for _, pm := range prompts[1:] {
		require.NoError(t, os.WriteFile(filepath.Join(roundDir, "role-"+pm["role"].(string)+".ndjson"), nil, 0o600))
	}
	env := []string{"TP_ROUND=1", "TP_ROUND_DIR=" + roundDir}
	file := handFile(t, dir)

	_, stderr, code := runTPEnv(t, dir, env, "review", "spec.md", "--record", file)
	require.Equal(t, 3, code, "one owed role has no file in the round directory; stderr: %s", stderr)

	first := prompts[0]["role"].(string)
	require.NoError(t, os.WriteFile(filepath.Join(roundDir, "role-"+first+".ndjson"), nil, 0o600))
	_, stderr, code = runTPEnv(t, dir, env, "review", "spec.md", "--record", file)
	require.Equal(t, 0, code, "every owed role answered; stderr: %s", stderr)
}

// TestReviewRecord_AHandFileStillRecordsARoundNobodyEmitted: with no emission
// there is no list of owed roles to prove, so --record <file> records as before.
func TestReviewRecord_AHandFileStillRecordsARoundNobodyEmitted(t *testing.T) {
	t.Parallel()
	dir, _ := inboxSpec(t)
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", handFile(t, dir))
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(1), out["round"])
}
