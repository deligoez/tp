package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func readStateJSON(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st map[string]any
	require.NoError(t, json.Unmarshal(data, &st))
	return st
}

func TestReviewState_RoundLifecycle(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// clean, dirty, clean, clean -> consecutive_clean 2
	sequence := []string{"", dirtyRow, "", ""}
	for i, content := range sequence {
		out, stderr, code := recordRound(t, dir, content)
		require.Equal(t, 0, code, "round %d: %s", i+1, stderr)
		assert.Equal(t, float64(i+1), out["round"], "rounds numbered by tp")
	}

	st := readStateJSON(t, dir)
	rounds := st["review_rounds"].([]any)
	require.Len(t, rounds, 4, "record appends entries")

	// Entries are immutable: recorded flags stay as computed at record time
	first := rounds[0].(map[string]any)
	assert.Equal(t, true, first["clean"])
	second := rounds[1].(map[string]any)
	assert.Equal(t, false, second["clean"])
	assert.Equal(t, "review-round-2.ndjson", second["file"])

	// consecutive-clean over the mixed sequence
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(2), out["consecutive_clean"], "clean, dirty, clean, clean -> 2")

	// Later --resolve edits to a round file never change the recorded entry
	roundFile := filepath.Join(dir, ".tp-review", "spec", "review-round-2.ndjson")
	_, _, code = runTP(t, dir, "review", "--resolve", roundFile, "0", "wontfix", "verifier: not a real issue")
	require.Equal(t, 0, code)
	st = readStateJSON(t, dir)
	second = st["review_rounds"].([]any)[1].(map[string]any)
	assert.Equal(t, false, second["clean"], "recorded clean flag is frozen at record time")
	assert.Equal(t, float64(1), second["findings"], "recorded count is frozen")
}

func TestReviewState_Staleness(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\noriginal\n"), 0o600))

	for i := 0; i < 2; i++ {
		_, _, code := recordRound(t, dir, "")
		require.Equal(t, 0, code)
	}

	// Matching hash keeps converged
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["stale"])
	assert.Equal(t, true, out["converged"])

	// Editing the spec after the last recorded round flips stale and unconverges
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\nedited after round\n"), 0o600))
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["stale"])
	assert.Equal(t, false, out["converged"])
}

func TestReviewState_CorruptIndexAborts(t *testing.T) {
	corruptCases := []struct {
		name  string
		setup func(t *testing.T, stateDir string)
	}{
		{
			name: "unparseable state.json",
			setup: func(t *testing.T, stateDir string) {
				require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte("{broken"), 0o600))
			},
		},
		{
			name: "round files without an index",
			setup: func(t *testing.T, stateDir string) {
				require.NoError(t, os.WriteFile(filepath.Join(stateDir, "review-round-1.ndjson"), []byte("{}\n"), 0o600))
			},
		},
	}

	for _, tc := range corruptCases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
			stateDir := filepath.Join(dir, ".tp-review", "spec")
			require.NoError(t, os.MkdirAll(stateDir, 0o755))
			tc.setup(t, stateDir)

			ndjson := filepath.Join(dir, "f.ndjson")
			require.NoError(t, os.WriteFile(ndjson, []byte(""), 0o600))
			importFile := filepath.Join(dir, "import.json")
			require.NoError(t, os.WriteFile(importFile, []byte(`[`+enforceTask+`]`), 0o600))

			commands := [][]string{
				{"review", "spec.md"},
				{"review", "spec.md", "--record", ndjson},
				{"review", "spec.md", "--status"},
				{"import", importFile, "--spec", "spec.md"},
			}
			stateBefore, _ := os.ReadFile(filepath.Join(stateDir, "state.json"))
			for _, args := range commands {
				_, stderr, code := runTP(t, dir, args...)
				assert.Equal(t, 3, code, "%v must abort with exit 3", args)
				assert.Contains(t, stderr, "repair or delete", "%v carries the repair hint", args)
			}
			stateAfter, _ := os.ReadFile(filepath.Join(stateDir, "state.json"))
			assert.Equal(t, stateBefore, stateAfter, "no silent rebuild of the index")
		})
	}
}
