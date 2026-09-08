package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewRecord_RecordsAFileLegalOnAllFourKeys is §5 row 3: the positive
// half of §2's required set. Rows 1 and 2 pin what --record refuses; nothing
// pinned what it must still accept, so a check that over-refuses — testing a
// key a legal row is not obliged to carry — would leave both of them green.
//
// The mutant the row states is exactly that: --record testing
// `resolved.evidence`, the key the record path already reads for a
// pre-resolved wontfix row, in place of the top-level `evidence`. Every row of
// this legal file carries no `resolved` block at all, so under the mutant all
// three are refused, the command exits 1 and no round is written.
//
// The verdict does not rest on the exit code alone. `findings: 3` and a round
// file holding three rows pin that all three rows were recorded rather than
// some subset the count happened to agree with, and `consecutive_clean: 0`
// pins the round as dirty — which the `high` severity secures under the
// default blocking policy and the three survivors secure under `all`, so the
// assertion does not smuggle in a converge_on assumption.
//
// The three `evidence` texts are one character, a short phrase and a long
// sentence, because the row says no row's evidence length is load-bearing and
// any non-empty text serves: a check keyed on a length rather than on
// emptiness is red here.
func TestReviewRecord_RecordsAFileLegalOnAllFourKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	out, stderr, code := recordRound(t, dir,
		`{"severity":"high","finding":"f1","location":"§1","evidence":"x"}`+"\n"+
			`{"severity":"medium","finding":"f2","location":"§2","evidence":"ran tp lint"}`+"\n"+
			`{"severity":"low","finding":"f3","location":"§3","evidence":"ran tp review spec.md --status and read consecutive_clean off the payload"}`+"\n")

	require.Equal(t, 0, code, "a file legal on all four keys records: %s", stderr)
	assert.Equal(t, float64(1), out["round"], "recorded against a spec with no prior round")
	assert.Equal(t, float64(3), out["findings"])
	assert.Equal(t, float64(0), out["consecutive_clean"])

	// The round file is the read-back that keeps `findings: 3` from being a
	// count with nothing behind it.
	roundFile := filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson")
	data, err := os.ReadFile(roundFile) //nolint:gosec // path built from t.TempDir()
	require.NoError(t, err, "an accepted file writes its round")
	rows := make([]string, 0, 3)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) != "" {
			rows = append(rows, line)
		}
	}
	assert.Len(t, rows, 3, "every row of the legal file reaches the round file")
	assert.Equal(t, 1, reviewRoundsRecorded(t, dir), "review_rounds holds the one round")
}
