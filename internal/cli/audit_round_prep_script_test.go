package cli

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAuditRoundPrepShellTestPasses puts scripts/audit-round-prep-test.sh in
// the gate by running it.
//
// The script decides which audit rows a role re-records without re-measuring,
// and a wrong decision is invisible from its exit code: a carried row reads as
// a PASS whether or not its evidence moved. Its shell test was in no gate, so a
// change to the script — or to the id scheme it reads — could leave the fixture
// red with nothing noticing. The shell test builds its own repository under
// mktemp and clears the git variables that would point it at this one, so it
// is run from the repo root as is.
func TestAuditRoundPrepShellTestPasses(t *testing.T) {
	t.Parallel()
	for _, tool := range []string{"python3", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH, so the shell test cannot run: %v", tool, err)
		}
	}

	root := repoRoot(t)
	cmd := exec.Command(filepath.Join(root, "scripts", "audit-round-prep-test.sh"))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "scripts/audit-round-prep-test.sh must exit 0:\n%s", out)
}
