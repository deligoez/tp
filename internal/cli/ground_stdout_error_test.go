package cli_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runTPUnwritableStdout runs tp with fd 1 opened READ-ONLY, so every write to
// stdout returns EBADF and no other part of the invocation is disturbed.
//
// The fd is the whole apparatus: nothing is stubbed, no code path is special-
// cased, and the failure arrives exactly where a full disk, a closed pipe or a
// misdirected redirect would put it. os.DevNull opened for reading is the
// cheapest such fd, and exec hands the *os.File straight to the child.
func runTPUnwritableStdout(t *testing.T, dir string, args ...string) (stderr string, exitCode int) {
	t.Helper()
	readOnly, err := os.Open(os.DevNull)
	require.NoError(t, err)
	defer func() { require.NoError(t, readOnly.Close()) }()

	cmd := exec.Command(binaryPath, append([]string{"--json"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TP_HC=0")
	cmd.Stdout = readOnly

	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	runErr := cmd.Run()

	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		exitCode = exitErr.ExitCode()
	} else if runErr != nil {
		t.Fatalf("unexpected error running tp: %v", runErr)
	}
	return errBuf.String(), exitCode
}

// TestOneStdoutWriteErrorIsOneClassification is the property every mode of this
// command shares and only one of them held: a write to stdout that fails is a
// failure, and it is the SAME failure whichever mode was writing.
//
// Measured before this held, with fd 1 read-only: `--units` exited **0** having
// printed nothing and said nothing, because fmt.Print's return was dropped;
// `--record` exited **1** with task-file advice, having already written the
// round file, which invites a re-run that then exits 3 for want of a round-2
// floor; and only `--status` exited 3. One error, three answers.
//
// The verdict rests on the exit code and on the envelope's own `code` field
// rather than on any sentence, and every mode is asserted in one table — a
// per-mode test is what let two of the three drift apart in the first place.
func TestOneStdoutWriteErrorIsOneClassification(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args func(dir string) []string
	}{
		{"--units", func(string) []string { return []string{"ground", "spec.md", "--units"} }},
		{"--status", func(string) []string { return []string{"ground", "spec.md", "--status"} }},
		{"--record", func(dir string) []string {
			return []string{"ground", "spec.md", "--record", writeGroundRows(t, dir, groundRecordRow(1))}
		}},
		{"the emission", func(string) []string { return []string{"ground", "spec.md"} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := writeGroundFixture(t)
			groundEmit(t, dir)

			stderr, code := runTPUnwritableStdout(t, dir, tc.args(dir)...)
			require.Equal(t, 3, code,
				"a stdout tp cannot write to is a file error, in every mode: stderr %q", stderr)
			assert.Equal(t, float64(3), groundErrorEnvelope(t, stderr)["code"])
		})
	}
}

// TestARecordThatCannotReportSaysTheRoundIsOnDisk pins the half the exit code
// cannot carry.
//
// --record writes the round file and only then prints, so a failed print leaves
// a round that IS recorded. The old exit 1 said the opposite — that the input
// needed fixing and the command re-running — and re-running it consumes the next
// round number, which has no emitted floor. The refusal now names the file that
// exists, so the operator's next step is to read it rather than to record again.
func TestARecordThatCannotReportSaysTheRoundIsOnDisk(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	rows := writeGroundRows(t, dir, groundRecordRow(1))

	stderr, code := runTPUnwritableStdout(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 3, code, "stderr: %q", stderr)

	require.FileExists(t, groundStatePath(dir, "ground-round-1.ndjson"),
		"the round was recorded before the report failed, which is what makes the message's claim true")
	assert.Contains(t, groundErrorEnvelope(t, stderr)["hint"], "ground-round-1.ndjson",
		"the hint names the round already on disk, so the operator does not re-record it")
}
