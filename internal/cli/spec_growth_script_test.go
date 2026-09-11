package cli

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSpecGrowthShellTestPasses puts scripts/check-spec-growth-test.sh in the
// gate by running it.
//
// scripts/check-spec-growth.py is registered in .tp/config.json's
// workflow.checks, so tp review runs it every round and reads its exit code
// under the checks contract (engine.CheckRan). A wrong baseline is invisible
// from that exit code alone — a sidecar read at the wrong commit turns growth
// into a pass — and a usage error that exited 1 would read as "found growth".
// The shell test builds its own repository under mktemp and clears the git
// variables that would point it at this one, so it is run from the repo root
// as is.
func TestSpecGrowthShellTestPasses(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"python3", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the shell test cannot run: %v", tool, err)
		}
	}

	root := repoRoot(t)
	cmd := exec.Command(filepath.Join(root, "scripts", "check-spec-growth-test.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "scripts/check-spec-growth-test.sh must exit 0:\n%s", out)
}
