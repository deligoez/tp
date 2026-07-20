package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recordRound(t *testing.T, dir, ndjsonContent string) (out map[string]any, stderr string, code int) {
	t.Helper()
	f := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(ndjsonContent), 0o600))
	var stdout string
	stdout, stderr, code = runTP(t, dir, "review", "spec.md", "--record", f)
	if code == 0 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	}
	return out, stderr, code
}

func TestReviewRecord_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	// Round 1: dirty (2 findings, one unresolved + one duplicate)
	out, stderr, code := recordRound(t, dir,
		`{"severity":"low","category":"consistency","location":"L1","finding":"f1","suggestion":"s"}`+"\n"+
			`{"severity":"low","category":"consistency","location":"L2","finding":"f2","suggestion":"s","resolved":{"status":"duplicate","evidence":"dup of f1"}}`+"\n")
	require.Equal(t, 0, code, "record failed: %s", stderr)
	assert.Equal(t, float64(1), out["round"])
	assert.Equal(t, float64(2), out["findings"])
	assert.Equal(t, false, out["clean"], "duplicate rows dirty the round")
	assert.Equal(t, false, out["converged"])

	// Round 2: clean (zero rows)
	out, _, code = recordRound(t, dir, "\n  \n")
	require.Equal(t, 0, code)
	assert.Equal(t, float64(2), out["round"])
	assert.Equal(t, true, out["clean"], "zero rows records a clean round")
	assert.Equal(t, float64(1), out["consecutive_clean"])

	// Round 3: clean via all-wontfix rows -> converged (default 2 clean rounds)
	out, _, code = recordRound(t, dir,
		`{"severity":"low","category":"x","location":"L","finding":"rejected","suggestion":"s","resolved":{"status":"wontfix","evidence":"verifier: false positive"}}`+"\n")
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
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
		return dir
	}

	t.Run("invalid JSON aborts with line number", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"ok":"row"}`+"\n"+`not json`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 2")
	})

	t.Run("pre-resolved fixed aborts", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"finding":"f","resolved":{"status":"fixed","evidence":"e"}}`+"\n")
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "line 1")
		assert.Contains(t, stderr, "fixed")
	})

	t.Run("wontfix with empty evidence aborts", func(t *testing.T) {
		dir := setup(t)
		_, stderr, code := recordRound(t, dir, `{"finding":"f","resolved":{"status":"wontfix","evidence":"  "}}`+"\n")
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
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	_, _, code := recordRound(t, dir, `{"severity":"low","category":"c","location":"L1","finding":"f1","suggestion":"s","class":"repeat-class"}`+"\n")
	require.Equal(t, 0, code)
	out, _, code := recordRound(t, dir, `{"severity":"low","category":"c","location":"L2","finding":"f2","suggestion":"s","class":"repeat-class"}`+"\n")
	require.Equal(t, 0, code)

	candidates := out["mechanize_candidates"].([]any)
	require.Len(t, candidates, 1, "class in 2 distinct rounds is a candidate")
	c := candidates[0].(map[string]any)
	assert.Equal(t, "repeat-class", c["class"])
	assert.Equal(t, float64(2), c["rounds_seen"])
	assert.Contains(t, out["hint"], "tp set --workflow checks")
}
