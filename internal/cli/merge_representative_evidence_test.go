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

// Two rows sharing (location, class) and severity, differing in finding text
// and in `evidence`. `implementer` sorts before `tester`, so §8.4's total order
// — severity, then role, then finding text — settles the representative on the
// role key and the fixture pins which row's `evidence` must survive.
const (
	repEvidenceImplementer = `implementer: §1 line 4 states no upper bound`
	repEvidenceTester      = `tester: ran the 0 and the 61 case, both accepted`

	repRowImplementer = `{"role":"implementer","severity":"high","class":"gap","location":"§1","finding":"zzz the bound is unstated","evidence":"` + repEvidenceImplementer + `"}`
	repRowTester      = `{"role":"tester","severity":"high","class":"gap","location":"§1","finding":"aaa no boundary test","evidence":"` + repEvidenceTester + `"}`
)

// TestReviewMerge_RepresentativeCarriesItsOwnEvidence is §5 row 4: when
// `--merge` collapses a cluster, the representative row it emits carries its
// own `evidence`, and the merged file is therefore itself recordable.
//
// The verdict rests on an end-to-end merge-then-record run, not on a field
// read: the last assertion runs `tp review <spec> --record` against the file
// `--merge` wrote. Row 4's mutant — the merge emitting the representative with
// `evidence` dropped — makes both halves red, the field read on the missing
// key and the record run at exit 1.
//
// Measured before this test existed: both halves are already green at HEAD,
// because clusterMergeFindings emits `allFindings[Representative(...)]`
// verbatim and never rebuilds the row. The test is a regression guard for that
// property — any future merge path that projects the representative onto a
// field list instead of passing the row through would drop `evidence` and
// silently make `--merge`'s own output unrecordable.
//
// The fixture's two rows must actually cluster for the guard to mean anything:
// two unclustered rows would each carry their own `evidence` under the mutant
// too. `merged_count` and `found_by` pin the cluster, and the finding texts are
// ordered so that role, not finding text, is what chooses the representative.
func TestReviewMerge_RepresentativeCarriesItsOwnEvidence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	require.NotEqual(t, repEvidenceImplementer, repEvidenceTester,
		"the fixture is only a test of whose evidence survives while the two texts differ")

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{repRowImplementer})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{repRowTester})
	merged := filepath.Join(dir, "merged.ndjson")

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", merged, f1, f2)
	require.Equal(t, 0, code, "two legal rows merge cleanly: %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "merge summary must be JSON: %s", stdout)
	require.Equal(t, float64(1), summary["merged_count"],
		"the two rows share (location, class) and must collapse to one, or the guard tests nothing")
	assert.Equal(t, float64(1), summary["duplicates_removed"])

	lines := nonBlankLines(t, merged)
	require.Len(t, lines, 1, "the merged file holds the one representative row")

	var row map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &row), "the merged row must be JSON: %s", lines[0])

	assert.Equal(t, "implementer", row["role"],
		"severity ties, so §8.4 settles the representative on role")
	assert.Equal(t, float64(2), row["found_by"],
		"both roles reached this cluster, so it really is a merge of the two rows")
	assert.Equal(t, repEvidenceImplementer, row["evidence"],
		"the representative carries its OWN evidence, not the other member's and not a synthesized one")

	// The point of the row: the merged file is recordable as it stands.
	recordOut, stderr, code := runTP(t, dir, "review", "spec.md", "--record", merged)
	require.Equal(t, 0, code, "--record accepts the merged file unchanged: %s", stderr)

	var recorded map[string]any
	require.NoError(t, json.Unmarshal([]byte(recordOut), &recorded), "record output must be JSON: %s", recordOut)
	assert.Equal(t, float64(1), recorded["round"])
	assert.Equal(t, float64(1), recorded["findings"], "the one merged row is the round's one finding")
}

// nonBlankLines returns the content lines of an NDJSON file.
func nonBlankLines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test-owned path under t.TempDir()
	require.NoError(t, err)
	out := make([]string, 0, 2)
	for l := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
