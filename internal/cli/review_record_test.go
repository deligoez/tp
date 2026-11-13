package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordRound records ndjsonContent as the next review round of dir/spec.md,
// as a round that was emitted first (markRoundEmitted).
func recordRound(t *testing.T, dir, ndjsonContent string) (out map[string]any, stderr string, code int) {
	t.Helper()
	markRoundEmitted(t, dir, "review")
	f := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(ndjsonContent), 0o600))
	var stdout string
	stdout, stderr, code = runTP(t, dir, "review", "spec.md", "--record", f)
	if code == 0 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	}
	return out, stderr, code
}

// markRoundEmitted stands in for the emission a fixture skips when it records
// rounds only to reach a state: it writes the snapshot `tp <phase> spec.md`
// writes for the round about to be recorded, from dir/spec.md as it stands,
// unless that snapshot is already on disk or there is no spec to snapshot.
//
// A record clears staleness only through the snapshot of its own emission
// (spec/backlog/hand-recorded-staleness.md), so a fixture that reaches a
// converged state by recording needs one per round. The snapshot is written
// rather than the emission run, because an emission also runs the registered
// checks, and several fixtures count their runs.
func markRoundEmitted(t *testing.T, dir, phase string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	if err != nil {
		return
	}
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	recorded, err := filepath.Glob(filepath.Join(stateDir, phase+"-round-*.ndjson"))
	require.NoError(t, err)
	name := fmt.Sprintf("snapshot-round-%d.md", len(recorded)+1)
	if phase != "review" {
		name = fmt.Sprintf("snapshot-%s-round-%d.md", phase, len(recorded)+1)
	}
	snapshot := filepath.Join(stateDir, name)
	if _, statErr := os.Stat(snapshot); statErr == nil {
		return
	}
	require.NoError(t, os.MkdirAll(stateDir, 0o750))
	require.NoError(t, os.WriteFile(snapshot, body, 0o600))
}

func TestReviewRecord_Lifecycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	// Round 1: dirty (2 findings — one surviving high, one exempt duplicate).
	// Under review_converge_on=blocking the surviving high finding blocks a
	// clean round; the duplicate-with-evidence row is out of the surviving set.
	out, stderr, code := recordRound(t, dir,
		`{"evidence":"read the cited section","severity":"high","category":"consistency","location":"L1","finding":"f1","suggestion":"s"}`+"\n"+
			`{"evidence":"read the cited section","severity":"low","category":"consistency","location":"L2","finding":"f2","suggestion":"s","resolved":{"status":"duplicate","evidence":"dup of f1"}}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(2), out["findings"])
	assert.Equal(t, false, out["clean"], "a surviving high finding blocks a clean round")
	assert.Equal(t, false, out["converged"])
	// The high is fixed: left open, it would stay carried under "DO NOT
	// re-report" and hold convergence whatever rounds 2 and 3 say
	// (spec/backlog/carried-rows-block.md).
	_, stderr, code = runTP(t, dir, "review", filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"), "--resolve", "0", "fixed", "reworded L1")
	require.Equal(t, 0, code, "resolve failed: %s", stderr)

	// Round 2: clean (zero rows)
	out, _, code = recordRound(t, dir, "\n  \n")
	require.Equal(t, 0, code)
	assert.Equal(t, float64(2), out["round"])
	assert.Equal(t, true, out["clean"], "zero rows records a clean round")
	assert.Equal(t, float64(1), out["consecutive_clean"])

	// Round 3: clean via all-wontfix rows -> converged (default 2 clean rounds)
	out, _, code = recordRound(t, dir,
		`{"evidence":"read the cited section","severity":"low","category":"x","location":"L","finding":"rejected","suggestion":"s","resolved":{"status":"wontfix","evidence":"verifier: false positive"}}`+"\n")
	require.Equal(t, 0, code)
	assert.Equal(t, float64(3), out["round"])
	assert.Equal(t, true, out["clean"], "all-wontfix rows record a clean round")
	assert.Equal(t, float64(2), out["consecutive_clean"])
	assert.Equal(t, true, out["converged"])
	assert.Equal(t, false, out["stale"])

	// Round files copied into the state dir
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	for _, f := range []string{"state.json", "review-round-1.ndjson", "review-round-2.ndjson", "review-round-3.ndjson"} {
		_, err := os.Stat(filepath.Join(stateDir, f))
		assert.NoError(t, err, "%s must exist", f)
	}
}

func TestReviewRecord_RowRules(t *testing.T) {
	t.Parallel()
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		return dir
	}

	t.Run("invalid JSON aborts with line number", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"evidence":"read the cited section","severity":"low","location":"L1","finding":"ok row"}`+"\n"+`not json`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 2")
	})

	t.Run("pre-resolved fixed aborts", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"evidence":"read the cited section","severity":"high","location":"L1","finding":"f","resolved":{"status":"fixed","evidence":"e"}}`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 1")
		assert.Contains(t, stderr, "fixed")
	})

	t.Run("wontfix with empty evidence aborts", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"evidence":"read the cited section","severity":"high","location":"L1","finding":"f","resolved":{"status":"wontfix","evidence":"  "}}`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 1")
		assert.Contains(t, stderr, "evidence")
	})

	t.Run("stale flips when spec edited after round", func(t *testing.T) {
		dir := setup(t)
		_, _, code := recordRound(t, dir, "")
		require.Equal(t, 0, code)
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec edited\n"), 0o600))
		out, _, code := recordRound(t, dir, "")
		require.Equal(t, 0, code)
		// round 2 recorded against the edited spec; not stale for itself
		assert.Equal(t, float64(2), out["round"])
	})
}

func TestReviewRecord_CorruptStateAborts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{broken"), 0o600))

	_, stderr, code := recordRound(t, dir, "")
	assert.Equal(t, 3, code, "corrupt state aborts with exit 3")
	assert.Contains(t, stderr, "repair or delete")
}

func TestReviewRecord_MechanizeCandidatesInOutput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	_, _, code := recordRound(t, dir, `{"evidence":"read the cited section","severity":"low","category":"c","location":"L1","finding":"f1","suggestion":"s","class":"repeat-class"}`+"\n")
	require.Equal(t, 0, code)
	out, _, code := recordRound(t, dir, `{"evidence":"read the cited section","severity":"low","category":"c","location":"L2","finding":"f2","suggestion":"s","class":"repeat-class"}`+"\n")
	require.Equal(t, 0, code)

	candidates := out["mechanize_candidates"].([]any)
	require.Len(t, candidates, 1, "class in 2 distinct rounds is a candidate")
	c := candidates[0].(map[string]any)
	assert.Equal(t, "repeat-class", c["class"])
	assert.Equal(t, float64(2), c["rounds_seen"])
	assert.Contains(t, out["hint"], "tp set --workflow checks")
}

func TestReviewRecord_RoleMissingWarns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// Line 1 carries `role`; line 2 omits it. The round still records (exit 0)
	// and the warning names the offending line and the findings file.
	ndjson := `{"evidence":"read the cited section","severity":"low","category":"c","location":"L1","finding":"f1","suggestion":"s","role":"implementer"}` + "\n" +
		`{"evidence":"read the cited section","severity":"low","category":"c","location":"L2","finding":"f2","suggestion":"s"}` + "\n"
	out, stderr, code := recordRound(t, dir, ndjson)
	require.Equal(t, 0, code, "round still records despite the role-less row")
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(2), out["findings"])
	assert.Contains(t, stderr, "missing the role field")
	assert.Contains(t, stderr, "line 2")
	assert.Contains(t, stderr, "findings.ndjson", "warning must name the findings file")
	assert.NotContains(t, stderr, "line 1", "a row carrying role must not warn")
}
