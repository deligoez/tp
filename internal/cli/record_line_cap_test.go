package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overCapEvidence is one byte-count past the 1MB NDJSON read cap every other
// reader in the review family applies, padded well clear of it so the row is
// unambiguously over rather than at the boundary.
const overCapEvidenceBytes = 1100 * 1024

// TestReviewRecord_RowOverTheReadCapIsRefusedByEveryReader closes the round
// --record could write but not read back.
//
// Measured against the built binary before this test existed: a single row
// carrying 1.1 MB of evidence RECORDED at exit 0 and wrote a 1,126,500-byte
// review-round-1.ndjson, and both readers of that file then died on it —
// `tp review <round file> --resolve 0 fixed` and `tp review <round file>
// --report` each exited 3 with "bufio.Scanner: token too long". The round was
// therefore recorded and could not be dispositioned. --record was the one NDJSON
// reader in the family that never applied the cap: parseRecordRows split the
// whole file on newlines instead of scanning it.
//
// The verdict here does not rest on matched text. It rests on three read-backs:
// the exit code of each of the three readers, and os.IsNotExist on the round
// file --record must not have written. The control run is what makes the exit 3
// mean the cap rather than anything else about the row — the SAME row with a
// short evidence string records at exit 0, so all four required fields are legal
// and the length is the only difference between the two.
func TestReviewRecord_RowOverTheReadCapIsRefusedByEveryReader(t *testing.T) {
	t.Parallel()

	row := func(evidence string) string {
		b, err := json.Marshal(map[string]any{
			"role": "implementer", "severity": "high", "class": "gap",
			"location": "§1", "finding": "the bound is unstated", "evidence": evidence,
		})
		require.NoError(t, err)
		return string(b)
	}
	shortRow := row("read §1 and the bound is not there")
	longRow := row(strings.Repeat("x", overCapEvidenceBytes))
	require.Greater(t, len(longRow), 1024*1024,
		"the fixture must actually exceed the cap, or this test measures nothing")
	require.Less(t, len(shortRow), 1024*1024,
		"the control must be under the cap for the pair to isolate length")

	// The control: the same row, short evidence, records.
	control := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(control, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	out, stderr, code := recordRound(t, control, shortRow+"\n")
	require.Equal(t, 0, code, "the row is legal on all four required keys: %s", stderr)
	require.Equal(t, float64(1), out["findings"], "the control row is the round's one finding")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	findings := writeFindingsFile(t, dir, "findings.ndjson", []string{longRow})

	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", findings)

	// Read-back first, so it is reached whatever the exit code turns out to be.
	_, statErr := os.Stat(filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
	assert.True(t, os.IsNotExist(statErr),
		"a refused file records no round; before the fix this file existed and was 1.1 MB")

	assert.Equal(t, 3, code, "an unreadable row is a file error, like every sibling reader: %s", stderr)
	assert.Contains(t, stderr, "1MB NDJSON read cap",
		"the hint names the cap, not the path — the path was fine")

	// The same file through the two readers that could not read what --record
	// used to write. All three now answer alike.
	for _, mode := range [][]string{
		{"review", findings, "--resolve", "0", "fixed"},
		{"review", findings, "--report"},
	} {
		_, stderr, code = runTP(t, dir, mode...)
		assert.Equal(t, 3, code, "%v must refuse the over-cap row too: %s", mode, stderr)
	}
}
