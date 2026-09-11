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

// preReleaseRoundFixture is the §5 row 7 fixture: one row copied byte for byte
// out of a round this repository recorded before v1.1.0
// (spec/.tp-review/1.0.0/review-round-4.ndjson, line 4). It is a file under
// testdata rather than a read of the live corpus because the corpus stops being
// uniformly pre-v1.1.0 at this release's own first recorded round — measured,
// every one of the 90 recorded review rows carrying a non-empty `evidence` is in
// spec/.tp-review/1.1.0/, and a test reading the corpus would silently change
// subject as this cycle records more.
const preReleaseRoundFixture = "pre-v1.1.0-review-round.ndjson"

// readPreReleaseRow returns the fixture's bytes after asserting it is the input
// row 7 describes. Every clause here is a property the verdict rests on, so a
// later edit of the fixture fails loudly instead of making the test vacuous:
// one row (so `findings` arithmetic and the round's cleanliness have one
// source), severity `high` (so the round is unclean under the default blocking
// policy AND under `all`, and the assertion smuggles in no review_converge_on
// assumption), no `resolved` block (so it is undispositioned and cannot leave
// the surviving set), and no `evidence` key while the three keys that predate
// this release are all present and non-empty (so `evidence` is the ONLY
// required key it lacks — the exact input §2's record-time predicate refuses).
func readPreReleaseRow(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", preReleaseRoundFixture))
	require.NoError(t, err)

	lines := make([]string, 0, 1)
	for l := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	require.Len(t, lines, 1, "the fixture is one round row")

	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &row))
	require.Equal(t, "high", row["severity"])
	require.NotContains(t, row, "resolved", "the row is undispositioned")
	require.NotContains(t, row, "evidence", "the row predates the required key")
	for _, k := range []string{"finding", "location", "severity"} {
		require.NotEmpty(t, row[k], "the row is legal on %s, so evidence is the only key it lacks", k)
	}
	return data
}

// TestReviewStatus_ReadsARoundRecordedBeforeThisRelease is §5 row 7, which is
// §1 decision 3: the required-field set is enforced when a round is RECORDED
// and never when one is LOADED. The two paths are different packages —
// parseRecordRows in internal/cli has exactly one caller, the --record sink,
// while --status reads through engine.LoadRoundRows, which validates nothing —
// so nothing but a test holds them apart.
//
// The mutant row 7 names is a status path that applies the record-time check on
// load: it exits 1, and the plain --status run below requires 0. Under --check
// the exit code does NOT separate them (the shipped tree exits 1 there too,
// because one undispositioned high row leaves the round unclean), which is why
// the plain run and the payload carry the verdict and the --check run only pins
// the "as before" half of the SHALL.
//
// Two read-backs keep the exit code from standing alone, and both would be
// satisfiable by an empty state — which is exactly the vacuous pass this design
// is built to exclude, since consecutive_clean is 0 for a spec with no rounds
// at all:
//
//  1. state.json stores clean:true and findings:0 for the round. That is NOT
//     what a real record would have written for this row; it is deliberate, and
//     it is the discriminator. tp recomputes a round's cleanliness live from its
//     recorded rows and falls back to the stored flag only when the round file
//     cannot be read, so review_rounds[0].clean coming back FALSE proves the
//     file was opened, parsed, and its evidence-less row counted as a surviving
//     blocking finding rather than skipped.
//  2. overlap_report credits the row's own role, which is derived from the round
//     file's contents by a second, independent path (latestRoundSignals) that
//     re-parses the same bytes.
//
// stale is pinned false so the --check exit is attributable to the round rather
// than to a spec hash that never matched, and the same bytes are handed to
// --record in a separate temp dir and required to be refused — without that,
// the whole test would pass against a fixture that carries evidence and would
// pin nothing about the boundary.
func TestReviewStatus_ReadsARoundRecordedBeforeThisRelease(t *testing.T) {
	t.Parallel()
	row := readPreReleaseRow(t)

	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\ncontent\n"), 0o600))

	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "review-round-1.ndjson"), row, 0o600))

	// A round file alone is not a readable round: --status reads the round
	// index, and a round file beside a missing state.json aborts at exit 3.
	specHash, err := engine.SpecHash(specPath)
	require.NoError(t, err)
	state := `{"spec":"spec.md","review_rounds":[{"round":1,"findings":0,"clean":true,` +
		`"recorded_at":"2026-01-01T00:00:00Z","file":"review-round-1.ndjson",` +
		`"spec_hash":"` + specHash + `"}],"audit_rounds":[]}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(state), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code, "a round recorded before this release still loads: %s", stderr)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, float64(0), out["consecutive_clean"])
	assert.Equal(t, false, out["converged"])
	assert.Equal(t, false, out["stale"], "the spec is unchanged since the round")

	rounds, ok := out["review_rounds"].([]any)
	require.True(t, ok)
	require.Len(t, rounds, 1, "the hand-written index is what --status read")
	assert.Equal(t, false, rounds[0].(map[string]any)["clean"],
		"stored clean:true flipped to false, which only reading the round file can do")

	report, ok := out["overlap_report"].([]any)
	require.True(t, ok)
	require.Len(t, report, 1, "the round file's one row reaches the overlap report")
	entry := report[0].(map[string]any)
	assert.Equal(t, "tester", entry["role"], "the row's own role, read back out of the round file")
	assert.Equal(t, float64(1), entry["unique"])

	// The "(exit 1 under --check, as before)" half: unchanged behaviour, and
	// unchanged for the pre-release reason — an unconverged spec, not a refusal.
	_, _, code = runTP(t, dir, "review", "spec.md", "--status", "--check")
	assert.Equal(t, 1, code, "--check still gates on convergence")

	// The boundary the row is about: these exact bytes are on the wrong side of
	// the record-time gate. Run in its own temp dir so the status fixture above
	// is untouched.
	recDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(recDir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	findings := filepath.Join(recDir, "findings.ndjson")
	require.NoError(t, os.WriteFile(findings, row, 0o600))
	_, recStderr, recCode := runTP(t, recDir, "review", "spec.md", "--record", findings)
	require.Equal(t, 1, recCode, "the fixture must be a row --record refuses: %s", recStderr)
	assert.Contains(t, recStderr, "evidence", "refused for the key it lacks")
}
