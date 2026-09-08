package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewMerge_ALineOfJustAnEmptyArrayIsNotADroppedRole is the sink half of
// the `[]` repair.
//
// tp's own code-audit prompt told the role that finding nothing should be
// reported as an empty array on its own line. That line is not an NDJSON
// object, so --merge skipped it, and a file whose only content line was skipped
// is a dropped role: exit 1 for the whole merge, and since §5 row 10 no `-o`
// file at all. One role reporting a clean result therefore took the other four
// roles' findings down with it, and the record step behind it had nothing to
// read.
//
// The prompt is fixed too, but the guard belongs here as well as there: rounds
// are run by agents holding prompts emitted by older binaries, and the sink is
// the only place that reaches those.
//
// The control run is what keeps this from reading as "dropped-role detection
// was disabled": a role file whose content line is genuinely malformed still
// drops the merge at exit 1 with no `-o`.
func TestReviewMerge_ALineOfJustAnEmptyArrayIsNotADroppedRole(t *testing.T) {
	t.Parallel()

	const legalRow = `{"role":"implementer","severity":"high","class":"gap",` +
		`"location":"§1","finding":"the bound is unstated","evidence":"read §1"}`

	// The spelling varies because the emitters do. `[]` is what the old
	// prompt asked for; an agent that pretty-prints its empty result writes
	// one of the others. `strings.TrimSpace` reaches the outer padding and
	// cannot reach the interior space — measured, `strings.TrimSpace("[ ]")`
	// is `"[ ]"` — so what decides this is a JSON parse, not a string
	// compare.
	for _, spelling := range []string{"[]", "[]  ", "[ ]", "[\t]"} {
		t.Run("zero findings spelled "+strconv.Quote(spelling)+": the panel survives", func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			roleA := writeFindingsFile(t, dir, "role-a.ndjson", []string{legalRow})
			roleB := writeFindingsFile(t, dir, "role-b.ndjson", []string{spelling})
			merged := filepath.Join(dir, "merged.ndjson")

			stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", merged, roleA, roleB)
			require.Equal(t, 0, code, "one role's clean result must not fail the merge: %s", stderr)

			// Read-backs, not matched text: the file exists and holds the one
			// row the other role found.
			lines := nonBlankLines(t, merged)
			require.Len(t, lines, 1, "the surviving role's finding reaches -o")
			var row map[string]any
			require.NoError(t, json.Unmarshal([]byte(lines[0]), &row))
			assert.Equal(t, "the bound is unstated", row["finding"])

			var summary map[string]any
			require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "merge summary must be JSON: %s", stdout)
			assert.Equal(t, float64(1), summary["merged_count"])
		})
	}

	// The controls are what keep the above from reading as "dropped-role
	// detection was disabled". The second is the one that fences the repair:
	// a NON-empty array is a role whose findings would be silently lost, so
	// it must still drop the merge — an emptiness test, not an array test.
	for _, malformed := range []string{"not json at all", `[{"severity":"high","finding":"f","location":"§1","evidence":"read §1"}]`} {
		t.Run("still dropped: "+strconv.Quote(malformed), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			roleA := writeFindingsFile(t, dir, "role-a.ndjson", []string{legalRow})
			roleB := writeFindingsFile(t, dir, "role-b.ndjson", []string{malformed})
			merged := filepath.Join(dir, "merged.ndjson")

			_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", merged, roleA, roleB)
			require.Equal(t, 1, code, "dropped-role detection is untouched: %s", stderr)
			_, statErr := os.Stat(merged)
			assert.True(t, os.IsNotExist(statErr), "a refused merge leaves -o untouched (§5 row 10)")
		})
	}
}
