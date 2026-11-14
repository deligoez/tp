package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// tpSpecCheckFixture is a git repository holding two specs, a.md and b.md,
// each with its task file, and one registered check that prints the
// TP_SPEC and TP_ROUND it was handed and fails, so its output tail carries
// them. A check used to learn the spec by asking `tp resume`, which names the
// active task file's spec rather than the one under review, so every check
// in a repository with two specs graded the wrong one.
func tpSpecCheckFixture(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	git := exec.Command("git", "init", "-q")
	git.Dir = dir
	require.NoError(t, git.Run())
	require.NoError(t, os.Mkdir(filepath.Join(dir, "spec"), 0o755))
	for _, name := range []string{"a", "b"} {
		body := "# Spec " + name + "\n\n## One\ncontent\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec", name+".md"), []byte(body), 0o600))
	}
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
	cmd := `echo \"spec=${TP_SPEC-unset} round=${TP_ROUND-unset}\"; exit 1`
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
		[]byte(`{"workflow":{"checks":[{"class":"spec-probe","cmd":"`+cmd+`"}]}}`), 0o600))
	for _, name := range []string{"a", "b"} {
		_, stderr, code := runTP(t, dir, "init", "spec/"+name+".md")
		require.Equal(t, 0, code, "init failed: %s", stderr)
	}
	// The active pointer names a, the spec that is NOT under review below.
	_, stderr, code := runTP(t, dir, "use", "spec/a.tasks.json")
	require.Equal(t, 0, code, "use failed: %s", stderr)
	return dir
}

// checkSaw returns the line the fixture's check printed.
func checkSaw(t *testing.T, stdout string) string {
	t.Helper()
	var payload struct {
		MechanicalChecks []struct {
			OutputTail []string `json:"output_tail"`
		} `json:"mechanical_checks"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload), "stdout: %s", stdout)
	require.Len(t, payload.MechanicalChecks, 1, "stdout: %s", stdout)
	for _, line := range payload.MechanicalChecks[0].OutputTail {
		if strings.HasPrefix(line, "spec=") {
			return line
		}
	}
	t.Fatalf("the check printed no spec line: %v", payload.MechanicalChecks[0].OutputTail)
	return ""
}

// TestRegisteredCheck_SeesTheSpecUnderReview: a registered check gets the
// reviewed spec in TP_SPEC, relative to the repository root, whichever spec
// the active pointer names, and never a TP_SPEC or TP_ROUND inherited from
// the shell that ran tp.
func TestRegisteredCheck_SeesTheSpecUnderReview(t *testing.T) {
	t.Parallel()
	inherited := []string{"TP_SPEC=spec/stale.md", "TP_ROUND=9"}
	want := "spec=spec/b.md round=unset"

	t.Run("emission", func(t *testing.T) {
		t.Parallel()
		dir := tpSpecCheckFixture(t)
		stdout, stderr, code := runTPEnv(t, dir, inherited, "review", "spec/b.md")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		require.Equal(t, want, checkSaw(t, stdout))
	})

	t.Run("status check", func(t *testing.T) {
		t.Parallel()
		dir := tpSpecCheckFixture(t)
		stdout, stderr, code := runTPEnv(t, dir, inherited, "review", "spec/b.md", "--status", "--check")
		require.Equal(t, 1, code, "a failing check fails --check; stderr: %s", stderr)
		require.Equal(t, want, checkSaw(t, stdout))
	})

	t.Run("status check by --file", func(t *testing.T) {
		t.Parallel()
		dir := tpSpecCheckFixture(t)
		stdout, stderr, code := runTPEnv(t, dir, inherited, "--file", "spec/b.tasks.json", "review", "spec/b.md", "--status", "--check")
		require.Equal(t, 1, code, "stderr: %s", stderr)
		require.Equal(t, want, checkSaw(t, stdout))
	})
}
