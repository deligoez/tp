package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStateRounds_Lifecycle(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	stateDir := filepath.Join(dir, ".tp-review", "spec")

	// Round 1: state dir + snapshot created, round auto-numbered
	stdout, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	loop := out["review_loop"].(map[string]any)
	assert.Equal(t, float64(1), loop["round"], "R = recorded rounds + 1")
	snap1, err := os.ReadFile(filepath.Join(stateDir, "snapshot-round-1.md"))
	require.NoError(t, err, "snapshot written at prompt generation")
	assert.Equal(t, "# Spec\ncontent\n", string(snap1), "byte copy of the spec")

	// Regenerating before recording overwrites the same round's snapshot
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\nedited\n"), 0o600))
	_, _, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	snap1b, err := os.ReadFile(filepath.Join(stateDir, "snapshot-round-1.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Spec\nedited\n", string(snap1b), "same-round snapshot overwritten")

	// Record round 1 with a finding; round 2 injects it into role prompts
	_, _, code = recordRound(t, dir,
		`{"severity":"high","category":"consistency","location":"L1","finding":"distinctive-previous-finding","suggestion":"fix"}`+"\n")
	require.Equal(t, 0, code)

	stdout, _, code = runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	loop = out["review_loop"].(map[string]any)
	assert.Equal(t, float64(2), loop["round"])
	assert.Equal(t, float64(1), loop["previous_findings"], "previous findings auto-injected")
	prompts := out["prompts"].([]any)
	require.Len(t, prompts, 3)
	for _, p := range prompts {
		assert.Contains(t, p.(map[string]any)["prompt"], "distinctive-previous-finding", "each role prompt carries the findings summary")
	}
}

func TestReviewStateRounds_RoundConflictAndNoState(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// --round conflicting with the state-derived R is a usage error
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--round", "3")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--no-state")

	// Matching --round is accepted
	_, _, code = runTP(t, dir, "review", "spec.md", "--round", "1")
	assert.Equal(t, 0, code)

	// --no-state: no reads or writes, round echoes the flag
	dir2 := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "spec.md"), []byte("# Spec\n"), 0o600))
	stdout, _, code := runTP(t, dir2, "review", "spec.md", "--no-state", "--round", "2")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	loop := out["review_loop"].(map[string]any)
	assert.Equal(t, float64(2), loop["round"], "round echoes the flag under --no-state")
	assert.Contains(t, loop["convergence"], "not being recorded")
	_, err := os.Stat(filepath.Join(dir2, ".tp-review"))
	assert.True(t, os.IsNotExist(err), "--no-state writes no state")

	// --no-state with state modes is a usage error
	for _, args := range [][]string{
		{"review", "spec.md", "--no-state", "--record", "x.ndjson"},
		{"review", "spec.md", "--no-state", "--status"},
	} {
		_, _, code := runTP(t, dir2, args...)
		assert.Equal(t, 2, code, "%v", args)
	}
}

func TestReviewStateRounds_PerspectivesDoNotTouchState(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package x\n"), 0o600))

	_, _, code := runTP(t, dir, "review", "spec.md", "--perspective", "code-audit", "--affected-files", "code.go")
	require.Equal(t, 0, code)
	_, err := os.Stat(filepath.Join(dir, ".tp-review"))
	assert.True(t, os.IsNotExist(err), "code-audit neither reads nor writes state")
}
