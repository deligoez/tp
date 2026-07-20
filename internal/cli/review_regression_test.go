package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fixedRow = `{"severity":"high","category":"consistency","location":"L1","finding":"settled decision text","suggestion":"s","resolved":{"status":"fixed","evidence":"rewrote section 2"}}` + "\n"

func TestRegression_StandaloneStateMode(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\noriginal\n"), 0o600))

	// Round 1 prompts (writes snapshot), record an unresolved finding, then
	// resolve it as fixed in the recorded round file
	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = recordRound(t, dir,
		`{"severity":"high","category":"consistency","location":"L1","finding":"settled decision text","suggestion":"s"}`+"\n")
	require.Equal(t, 0, code)
	roundFile := filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson")
	_, _, code = runTP(t, dir, "review", "--resolve", roundFile, "0", "fixed", "rewrote section 2")
	require.Equal(t, 0, code)

	// Edit the spec so the diff is non-empty
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nrewritten\n"), 0o600))

	stateBefore, err := os.ReadDir(filepath.Join(dir, ".tp-review", "spec"))
	require.NoError(t, err)

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	prompts := out["prompts"].([]any)
	require.Len(t, prompts, 1)
	p := prompts[0].(map[string]any)
	assert.Equal(t, "regression", p["role"])
	text := p["prompt"].(string)

	// §11.3 body order elements
	assert.Contains(t, text, "You guard decisions this spec has already settled")
	assert.Contains(t, text, "Changed sections since round 1")
	assert.Contains(t, text, "settled decision text — rewrote section 2")
	assert.Contains(t, text, "1. Does any changed section revert or weaken a fixed finding above?")
	assert.Contains(t, text, "regression", "category enum includes regression")

	// Read-only: no new files, no state.json change
	stateAfter, err := os.ReadDir(filepath.Join(dir, ".tp-review", "spec"))
	require.NoError(t, err)
	assert.Equal(t, len(stateBefore), len(stateAfter), "standalone regression never writes state")
}

func TestRegression_StandaloneExplicitMode(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.md"), []byte("# Spec\n## 1. A\nold\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n## 1. A\nnew\n"), 0o600))
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(fixedRow), 0o600))

	stdout, _, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression", "--diff-from", "base.md", "--findings", findings)
	require.Equal(t, 0, code)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	text := out["prompts"].([]any)[0].(map[string]any)["prompt"].(string)
	assert.Contains(t, text, "Changed sections since baseline base.md")
	assert.Contains(t, text, "settled decision text")

	_, err := os.Stat(filepath.Join(dir, ".tp-review"))
	assert.True(t, os.IsNotExist(err), "mode (b) touches no state")
}

func TestRegression_UsageErrors(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// No state and no explicit inputs
	_, _, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression")
	assert.Equal(t, 2, code, "missing inputs are a usage error")

	// Partial explicit input
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.md"), []byte("# Spec\n"), 0o600))
	_, _, code = runTP(t, dir, "review", "spec.md", "--perspective", "regression", "--diff-from", "base.md")
	assert.Equal(t, 2, code, "diff-from without findings is a usage error")

	// Vacuous input: identical baseline and zero fixed findings
	findings := filepath.Join(dir, "f.ndjson")
	require.NoError(t, os.WriteFile(findings, []byte(`{"finding":"open issue"}`+"\n"), 0o600))
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--perspective", "regression", "--diff-from", "spec.md", "--findings", findings)
	assert.Equal(t, 2, code, "empty diff plus zero fixed findings is a usage error")
	assert.Contains(t, stderr, "nothing to check")
}
