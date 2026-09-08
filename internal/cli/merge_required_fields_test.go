package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewMerge_SkipsRowMissingEvidence is §5 row 5: `evidence` joins
// `location`, `severity` and `finding` in the merge gate's required set.
//
// The offending key is `evidence` rather than one of the other three on
// purpose. Row 5's mutant — `evidence` added to `--record`'s check only and not
// to `missingFindingFields` — is what the tree looked like before this test's
// task, and with `location` omitted instead the two gates would already agree,
// so the mutant would be indistinguishable from the shipped rule at both.
//
// The offending row sits alone in its own input file so the skip is observable:
// that input then parses no row and falls under `--merge`'s existing
// dropped-input rule, which the legal second file does not lift.
func TestReviewMerge_SkipsRowMissingEvidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	const noEvidence = `{"role":"implementer","severity":"high","class":"gap","location":"§1","finding":"missing bound","suggestion":"state it"}`
	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{noEvidence})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1, f2)

	require.Equal(t, 1, code,
		"the input holding only the row missing evidence parses no row, and the legal second file does not lift the dropped-input rule: %s", stderr)
	assert.Contains(t, stderr, "missing evidence",
		"the stderr warning names the missing key, as it already does for the other three")
	assert.Contains(t, stderr, f1, "the warning and the refusal name the offending input")

	inputs := mergeInputsOf(t, stdout)
	assert.Equal(t, [2]int{0, 1}, inputs[f1], "the row missing evidence is counted as skipped, not parsed")
	assert.Equal(t, [2]int{1, 0}, inputs[f2], "the legal file is untouched")

	// The other half of row 5: the same row at the record gate.
	_, stderr, code = recordRound(t, dir, noEvidence+"\n")
	require.Equal(t, 1, code, "--record refuses the same row: %s", stderr)
	assert.Equal(t, []int{1}, linesNamed(t, stderr), "--record names the offending line")
}
