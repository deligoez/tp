package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The one row §5 row 6 describes, and the control that proves what it is about.
// Both are built from preResolvedRowBase, so the ONLY difference between them
// is the `resolved` block — the fixture cannot drift into a second difference
// and take the control's meaning with it.
const (
	preResolvedTopEvidence      = `implementer: §1 line 4 states no upper bound`
	preResolvedResolvedEvidence = `verifier: §1 now states the bound, so this is fixed`

	preResolvedRowBase = `"role":"implementer","severity":"high","class":"gap",` +
		`"location":"§1","finding":"the bound is unstated","evidence":"` + preResolvedTopEvidence + `"`

	// Legal on all four required keys and carrying no resolved block.
	preResolvedControlRow = `{` + preResolvedRowBase + `}`

	// The same row, pre-resolved fixed with its own resolved.evidence.
	preResolvedFixedRow = `{` + preResolvedRowBase +
		`,"resolved":{"status":"fixed","evidence":"` + preResolvedResolvedEvidence + `"}}`
)

// TestReviewRecord_PreResolvedFixedRowIsStillRefused is §5 row 6: the boundary
// of §2's parity. A row pre-resolved `fixed` and legal on all four required
// keys is emitted by `--merge` at exit 0 and refused by `--record` at exit 1,
// because the parity the release adds is over the required-field SET and does
// not reach `--record`'s pre-resolved rules.
//
// The mutant the row states unifies the two gates by routing `--record` through
// `missingFindingFields` alone, which drops the pre-resolved `fixed` abort and
// takes `--record` to exit 0. Two assertions here go red under it and neither
// rests on matched text: the exit code, and the absence of `review-round-1`.
//
// Measured before this test existed: green at HEAD — parseRecordRows returns on
// the `fixed` case before §2's accumulator is ever reached. What the guard
// catches is a future unification of the two gates, which is exactly the shape
// of change §2's parity invites.
//
// The trap the row carries is that the same exit 1 is what a row MISSING a
// required field gets, so a fixture short of a key would pass for the wrong
// reason and the guard would pin nothing. The control run is what closes that,
// and it is a read-back rather than a match on the refusal's wording: the same
// row with the resolved block removed and nothing else changed RECORDS, at exit
// 0 with `findings: 1`. The four keys are therefore legal, so the refusal below
// cannot be §2's.
//
// Three further properties are pinned deliberately, each because a plausible
// weakening of the fixture would leave the verdict standing on nothing:
//
//   - The row sits alone in its own input file, so a `--merge` that skipped it
//     would leave that input parsing zero rows and refuse at exit 1 under the
//     existing dropped-input rule. `--merge`'s exit 0 is therefore evidence the
//     row was emitted, not merely tolerated; `merged_count: 1` reads it back.
//   - `--record` is run against the file `--merge` WROTE, not against the input,
//     so the two halves of the row are the same bytes. The merged row is read
//     back for `resolved.status: fixed` first — a merge that dropped the block
//     would make `--record` accept, and the test would then be measuring the
//     merge rather than the record gate.
//   - The top-level `evidence` and `resolved.evidence` are different non-empty
//     texts, so the assertion on the merged row's `evidence` distinguishes the
//     row's own text from the resolved block's.
func TestReviewRecord_PreResolvedFixedRowIsStillRefused(t *testing.T) {
	t.Parallel()

	require.NotEqual(t, preResolvedTopEvidence, preResolvedResolvedEvidence,
		"the two evidence texts must differ, or reading the wrong one is undetectable")

	// The control: the same row without the resolved block records. This is the
	// proof that the refusal below is not a required-field refusal.
	control := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(control, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	controlOut, controlErr, controlCode := recordRound(t, control, preResolvedControlRow+"\n")
	require.Equal(t, 0, controlCode,
		"the row is legal on all four required keys — only the resolved block separates it from this: %s", controlErr)
	require.Equal(t, float64(1), controlOut["findings"], "the control row is the round's one finding")

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{preResolvedFixedRow})
	merged := filepath.Join(dir, "merged.ndjson")

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", merged, f1)
	require.Equal(t, 0, code,
		"the row is alone in its input, so a skip would drop the input and refuse at exit 1: %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "merge summary must be JSON: %s", stdout)
	assert.Equal(t, float64(1), summary["merged_count"], "--merge emits the pre-resolved row")
	assert.Equal(t, float64(0), summary["duplicates_removed"])

	lines := nonBlankLines(t, merged)
	require.Len(t, lines, 1, "the merged file holds the one row")
	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &row), "the merged row must be JSON: %s", lines[0])
	assert.Equal(t, preResolvedTopEvidence, row["evidence"], "the emitted row carries its own evidence")
	resolved, ok := row["resolved"].(map[string]any)
	require.True(t, ok, "--merge passes the resolved block through, or --record below refuses a different row")
	assert.Equal(t, "fixed", resolved["status"])
	assert.Equal(t, preResolvedResolvedEvidence, resolved["evidence"])

	// The other half of the row, against the very bytes --merge wrote.
	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", merged)

	// The read-back comes first so that it is reached under the mutant rather
	// than shadowed by the exit-code require below: both go red there, and this
	// one rests on os.IsNotExist rather than on any text.
	_, statErr := os.Stat(filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
	assert.True(t, os.IsNotExist(statErr),
		"a refused file records no round; under the mutant this round file exists")

	require.Equal(t, 1, code,
		"--record's pre-resolved fixed rule is untouched by §2's parity: %s", stderr)
	assert.Equal(t, []int{1}, linesNamed(t, stderr), "--record names the offending line")
}
