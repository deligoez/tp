package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// emissionFixture seeds a spec and one affected file both phases can be emitted
// against, and returns the directory holding them.
func emissionFixture(t *testing.T) (dir, specPath, codePath string) {
	t.Helper()
	dir = t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath,
		[]byte("# Spec\n## 1. Models\n### 1.1 Task\nCreate a Task model.\n"), 0o600))
	codePath = filepath.Join(dir, "code.go")
	require.NoError(t, os.WriteFile(codePath, []byte("package main\nfunc Foo() int { return 42 }\n"), 0o600))
	return dir, specPath, codePath
}

// emittedPrompts runs tp and returns its prompts array.
func emittedPrompts(t *testing.T, dir string, env []string, args ...string) []map[string]any {
	t.Helper()
	stdout, stderr, exit := runTPEnv(t, dir, env, args...)
	require.Equal(t, 0, exit, "stderr: %s", stderr)
	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	raw, ok := result["prompts"].([]any)
	require.True(t, ok, "the payload carries prompts: %s", stdout)
	require.NotEmpty(t, raw)
	prompts := make([]map[string]any, 0, len(raw))
	for _, p := range raw {
		prompts = append(prompts, p.(map[string]any))
	}
	return prompts
}

// TestRoleOutputPathIsRoundScopedUnderARun is test 27's emission half (§6.3).
// Under TP_ROUND_DIR, tp review and tp audit emit
// $TP_ROUND_DIR/role-<that role's id>.ndjson.part per role — the single path
// §6.2's write allowlist grants the unit and the driver renames on exit 0 — so
// the prompt and the allowlist name one filename rather than two. The legacy
// <phase>-r<N>-<role>.ndjson name must be gone from the prompt entirely: a role
// told two filenames writes the one the allowlist denies.
func TestRoleOutputPathIsRoundScopedUnderARun(t *testing.T) {
	t.Parallel()
	dir, specPath, codePath := emissionFixture(t)

	for _, tc := range []struct {
		phase string
		args  []string
	}{
		{"review", []string{"review", specPath, "--no-state"}},
		{"audit", []string{"audit", specPath, "--affected-files", codePath}},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			roundDir := filepath.Join(dir, ".tp", "rounds", "spec", tc.phase+"-r1")
			prompts := emittedPrompts(t, dir, []string{engine.EnvRoundDir + "=" + roundDir}, tc.args...)

			for _, pm := range prompts {
				role := pm["role"].(string)
				got := pm["output_path"].(string)
				want := filepath.Join(roundDir, "role-"+role+".ndjson.part")
				assert.Equal(t, want, got,
					"under TP_ROUND_DIR the role's output file is $TP_ROUND_DIR/role-<role>.ndjson.part")

				body := pm["prompt"].(string)
				assert.Contains(t, body, "Write this round's findings to: "+want,
					"§10.4's line names the round-scoped path, not a second one")
				assert.NotContains(t, body, tc.phase+"-r1-"+role+".ndjson",
					"the cwd-relative name must not survive anywhere in the prompt")
			}
		})
	}
}

// TestRoleOutputPathIsUnchangedOutsideARun is §6.3's last sentence: outside a
// run the emitted filename is unchanged, so the interactive loop that has always
// collected <phase>-r<N>-<role>.ndjson in the working directory keeps working.
// It is the arm that fails if the round-scoped path is emitted unconditionally.
func TestRoleOutputPathIsUnchangedOutsideARun(t *testing.T) {
	t.Parallel()
	dir, specPath, codePath := emissionFixture(t)

	for _, tc := range []struct {
		phase string
		args  []string
	}{
		{"review", []string{"review", specPath, "--no-state"}},
		{"audit", []string{"audit", specPath, "--affected-files", codePath}},
	} {
		t.Run(tc.phase, func(t *testing.T) {
			// TP_ROUND_DIR is cleared explicitly rather than merely left
			// unset: the child inherits the test process's environment, so
			// an ambient value — a `go test` run from inside a unit — would
			// otherwise turn this arm red for a reason that is not the code.
			prompts := emittedPrompts(t, dir, []string{engine.EnvRoundDir + "="}, tc.args...)
			for _, pm := range prompts {
				role := pm["role"].(string)
				assert.Equal(t, tc.phase+"-r1-"+role+".ndjson", pm["output_path"].(string),
					"with no TP_ROUND_DIR in the environment the name is the one tp always emitted")
			}
		})
	}
}

// TestEmittedRoleFileFeedsTheRecordUnitsGlob ties §6.3's two halves together.
// The record unit merges $TP_ROUND_DIR/role-*.ndjson; the role is told to write
// $TP_ROUND_DIR/role-<role>.ndjson.part, which the driver renames. Writing what
// the prompt names and promoting it exactly as the driver does must therefore
// produce a file that glob collects — an emitted name one character off would
// merge as a dropped role rather than as an error.
func TestEmittedRoleFileFeedsTheRecordUnitsGlob(t *testing.T) {
	t.Parallel()
	dir, specPath, _ := emissionFixture(t)
	roundDir := filepath.Join(dir, ".tp", "rounds", "spec", "review-r1")
	require.NoError(t, os.MkdirAll(roundDir, 0o750))

	prompts := emittedPrompts(t, dir, []string{engine.EnvRoundDir + "=" + roundDir},
		"review", specPath, "--no-state")

	want := make([]string, 0, len(prompts))
	for _, pm := range prompts {
		part := pm["output_path"].(string)
		require.True(t, strings.HasSuffix(part, ".ndjson.part"),
			"the role writes the .part; the driver owns the final name")
		require.NoError(t, os.WriteFile(part, []byte("{\"id\":\"f1\"}\n"), 0o600))
		// Exactly the driver's promotion (engine.promoteRoleFindings).
		final := strings.TrimSuffix(part, ".part")
		require.NoError(t, os.Rename(part, final))
		require.Equal(t, engine.RoleFindingsPath(roundDir, pm["role"].(string)), final,
			"the promoted name is the one §3.3's predicate and the merge glob read")
		want = append(want, final)
	}

	got, err := filepath.Glob(filepath.Join(roundDir, "role-*.ndjson"))
	require.NoError(t, err)
	assert.ElementsMatch(t, want, got,
		"the record unit's role-*.ndjson glob collects exactly the files the prompts named")
}
