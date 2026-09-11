package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runAuditRecordBrief runs the audit-record unit's emitted brief_command
// through /bin/sh the way a unit does, against the real binary, over a round
// directory holding one role file with the given content. It returns the
// shell's exit code, its stderr, and how many audit rounds are recorded after.
func runAuditRecordBrief(t *testing.T, roleContent string) (exitCode int, stderr string, rounds int) {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\n## A\n\ntext\n"), 0o600))
	roundDir := filepath.Join(dir, "round")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(roundDir, "role-r.ndjson"), []byte(roleContent), 0o600))

	brief := engine.UnitAuditRecord.BriefCommand(engine.UnitTarget{Spec: "spec.md", RoundDir: roundDir})
	cmd := exec.Command("/bin/sh", "-c", brief)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Dir(binaryPath)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TP_ROUND_DIR="+roundDir, "NO_COLOR=1")
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	} else {
		require.NoError(t, err)
	}

	recorded, err := filepath.Glob(filepath.Join(dir, ".tp-review", "spec", "audit-round-*.ndjson"))
	require.NoError(t, err)
	return exitCode, errBuf.String(), len(recorded)
}

// TestAuditRecordBrief_ARefusedMergeEndsTheChain runs the emitted audit-record
// brief over a role file that parses nothing, so the merge refuses and writes
// no `-o`. The chain must end at the merge: nothing recorded, and the unit's
// exit and last word are the merge's own diagnosis, which names the field and
// the lines. Joined with `;`, the record step ran anyway and the unit ended at
// exit 3 on "cannot read results file", a missing path in place of the cause.
//
// The control is the same brief over a legal role file: it records one round,
// so the count below is a count and not a glob that matches nothing.
func TestAuditRecordBrief_ARefusedMergeEndsTheChain(t *testing.T) {
	t.Parallel()

	t.Run("control: a clean merge records the round", func(t *testing.T) {
		t.Parallel()
		code, stderr, rounds := runAuditRecordBrief(t, `{"role":"r","item_id":"a","status":"PASS"}`+"\n")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Equal(t, 1, rounds, "the brief's record step records one audit round")
	})

	t.Run("a refused merge records nothing and ends at its own exit", func(t *testing.T) {
		t.Parallel()
		code, stderr, rounds := runAuditRecordBrief(t, `{"role":"r","item_id":"a"}`+"\n")
		assert.Equal(t, 0, rounds, "a refused merge records no round")
		assert.Equal(t, 1, code, "the unit exits with the merge's own code, not --record's: %s", stderr)
		last := lastLine(t, stderr)
		assert.Contains(t, last, "no line parsed in", "the unit's last word is the merge's diagnosis")
		assert.NotContains(t, stderr, "cannot read results file", "the record step is never reached")
	})
}
