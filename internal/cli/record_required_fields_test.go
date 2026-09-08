package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recordLinePattern extracts the "line N" references a --record refusal names.
// The verdict of the row-1 test rests on the SET of numbers it yields, not on
// any wording around them: the mutant §5 row 1 states — a record-side check
// testing only `evidence` — also exits 1 and also names line 4, so a test
// asserting an exit code or a single line number is green under it.
var recordLinePattern = regexp.MustCompile(`line (\d+)`)

// linesNamed returns every distinct line number the error message references,
// in the order they appear.
func linesNamed(t *testing.T, stderr string) []int {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload))
	seen := map[int]bool{}
	out := make([]int, 0, 4)
	for _, m := range recordLinePattern.FindAllStringSubmatch(payload.Error, -1) {
		n, err := strconv.Atoi(m[1])
		require.NoError(t, err)
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out
}

// reviewRoundsRecorded reads the state index and returns how many rounds it
// holds. It is the "review_rounds unchanged" half of §5 rows 1 and 2.
func reviewRoundsRecorded(t *testing.T, dir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st struct {
		ReviewRounds []json.RawMessage `json:"review_rounds"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	return len(st.ReviewRounds)
}

// specWithOneRecordedRound writes a spec and records one legal round against
// it, so `review_rounds` holds a non-zero value for the refusal under test to
// leave unchanged (§5 rows 1 and 2 both pin this).
func specWithOneRecordedRound(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	_, stderr, code := recordRound(t, dir,
		`{"severity":"low","finding":"f0","location":"L0","evidence":"ran tp lint spec.md"}`+"\n")
	require.Equal(t, 0, code, "the seeding round must record: %s", stderr)
	require.Equal(t, 1, reviewRoundsRecorded(t, dir))
	return dir
}

// TestReviewRecord_RefusesEveryRowMissingARequiredField is §5 row 1. The
// fixture is one row per required key, each legal on the other three, so the
// set of lines named is what separates the shipped rule from a check that
// tests `evidence` alone.
func TestReviewRecord_RefusesEveryRowMissingARequiredField(t *testing.T) {
	t.Parallel()
	dir := specWithOneRecordedRound(t)

	_, stderr, code := recordRound(t, dir,
		`{"finding":"f1","location":"L1","evidence":"read §2"}`+"\n"+
			`{"severity":"high","location":"L2","evidence":"read §2"}`+"\n"+
			`{"severity":"medium","finding":"f3","evidence":"read §2"}`+"\n"+
			`{"severity":"low","finding":"f4","location":"L4"}`+"\n")

	require.Equal(t, 1, code, "a row missing a required field is refused: %s", stderr)
	assert.ElementsMatch(t, []int{1, 2, 3, 4}, linesNamed(t, stderr),
		"every offending line is named in one exit, not just the row missing evidence")

	var payload struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload))
	for _, key := range []string{"severity", "finding", "location", "evidence"} {
		assert.Contains(t, payload.Error, key, "the message names the key each row lacks")
	}
	assert.NotEmpty(t, payload.Hint, "an exit-1 record site never inherits the task-file default")

	_, err := os.Stat(filepath.Join(dir, ".tp-review", "spec", "review-round-2.ndjson"))
	assert.True(t, os.IsNotExist(err), "a refused file writes no round file")
	assert.Equal(t, 1, reviewRoundsRecorded(t, dir), "review_rounds is unchanged")
}

// TestReviewRecord_RefusesAnEmptyObjectRow is §5 row 2: at HEAD `{}` records
// at exit 0 as one finding and advances the round.
func TestReviewRecord_RefusesAnEmptyObjectRow(t *testing.T) {
	t.Parallel()
	dir := specWithOneRecordedRound(t)

	_, stderr, code := recordRound(t, dir, "{}\n")

	require.Equal(t, 1, code, "a row with no keys at all is refused: %s", stderr)
	assert.Equal(t, []int{1}, linesNamed(t, stderr))

	_, err := os.Stat(filepath.Join(dir, ".tp-review", "spec", "review-round-2.ndjson"))
	assert.True(t, os.IsNotExist(err), "a refused file writes no round file")
	assert.Equal(t, 1, reviewRoundsRecorded(t, dir), "review_rounds is unchanged")
}

// TestReviewRecord_RequiredFieldRefusalIsRaisedLast holds the ordering the
// required-field check must not disturb: it pre-empts neither the pre-resolved
// `fixed` abort's message nor the unreadable-spec exit 3.
//
// This assertion exists because nothing else discriminates it any more.
// spec_hash_hint_test.go carried the exit-3 half incidentally until its fixture
// was migrated to four legal keys; a legal row reaches engine.SpecHash whatever
// the ordering, so that test is now green under both.
func TestReviewRecord_RequiredFieldRefusalIsRaisedLast(t *testing.T) {
	t.Parallel()

	t.Run("the pre-resolved fixed abort keeps its message", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

		// Line 1 is missing `location`; line 2 is legal on all four keys and
		// arrives pre-resolved fixed. The fixed abort returns from the row
		// loop, so the accumulated line-1 refusal never reaches the exit.
		_, stderr, code := recordRound(t, dir,
			`{"severity":"high","finding":"f1","evidence":"read §2"}`+"\n"+
				`{"severity":"high","finding":"f2","location":"L2","evidence":"read §2","resolved":{"status":"fixed","evidence":"e"}}`+"\n")

		require.Equal(t, 1, code)
		var payload struct {
			Error string `json:"error"`
		}
		require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload))
		assert.Contains(t, payload.Error, "pre-resolved fixed")
		assert.Equal(t, []int{2}, linesNamed(t, stderr),
			"the fixed row's line, not line 1's missing location")
	})

	t.Run("an unreadable spec is still a file error", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root reads regardless of mode bits")
		}
		dir := t.TempDir()
		specPath := filepath.Join(dir, "spec.md")
		require.NoError(t, os.WriteFile(specPath, []byte("# S\n## A\n| c |\n|---|\n| x |\n"), 0o600))
		_, _, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code)

		// The row is refused by the required-field rule, so this run exits 1
		// unless the spec's own file error is raised first.
		findings := filepath.Join(dir, "findings.ndjson")
		require.NoError(t, os.WriteFile(findings, []byte("{}\n"), 0o600))

		require.NoError(t, os.Chmod(specPath, 0o000))
		t.Cleanup(func() { _ = os.Chmod(specPath, 0o600) })

		_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", findings)
		require.Equal(t, 3, code, "an unreadable spec is a file error, not a refused row")

		var payload struct {
			Error string `json:"error"`
			Hint  string `json:"hint"`
		}
		require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload))
		assert.Contains(t, payload.Hint, "permission denied", "the hint names the real cause")
	})
}
