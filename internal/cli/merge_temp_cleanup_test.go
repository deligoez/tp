package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chflagsOrSkip sets a BSD file flag on path and clears it when the test ends.
// It skips rather than fails where the tool or the flag is unavailable: the
// state under test is reachable, but not everywhere.
func chflagsOrSkip(t *testing.T, flag, path string) {
	t.Helper()
	if _, err := exec.LookPath("chflags"); err != nil {
		t.Skip("chflags is a BSD/macOS tool; this state has no portable construction")
	}
	if err := exec.Command("chflags", flag, path).Run(); err != nil {
		t.Skipf("chflags %s is not available here: %v", flag, err)
	}
	// Registered after t.TempDir's own cleanup, so it runs first and the
	// directory is removable again by the time the harness deletes it.
	t.Cleanup(func() { _ = exec.Command("chflags", "no"+flag, path).Run() })
}

// tempsLeftIn returns the names of the merge temporaries sitting in dir.
func tempsLeftIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	left := make([]string, 0, 1)
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tp-merge-") {
			left = append(left, e.Name())
		}
	}
	return left
}

// mergeRefusalPayload parses the JSON envelope a refused merge writes.
func mergeRefusalPayload(t *testing.T, stderr string) string {
	t.Helper()
	var payload struct {
		Error string `json:"error"`
		Hint  string `json:"hint"`
	}
	require.NoError(t, json.Unmarshal([]byte(lastLine(t, stderr)), &payload),
		"the refusal must be a JSON envelope: %s", stderr)
	return payload.Error
}

// TestReviewMerge_NamesTheTemporaryItCouldNotRemove drives writeMergeOutput's
// two failure endings through the binary.
//
// The field report this covers: with the temporary locked mid-write, `rename`
// failed EPERM and `os.Remove` failed EPERM as well. The removal error was
// discarded, so tp exited 3 reporting only the rename and left a 135 MB
// `out.ndjson.tp-merge-…` beside `-o` — a name the operator never chose and
// nothing would ever clean up.
//
// The predecessor of this test called mergeWriteFailure directly and was
// therefore blind to its call site: replacing that call with
// `_ = os.Remove(name)` / `mergeWriteFailure(err, nil, name)` left the whole
// suite green. Measured, that mutant turns the first subtest below red and
// leaves the second green.
//
// The pair was recorded as unreachable from a portable test. It is reachable
// on a BSD filesystem, without root and without a timing window: an
// append-only directory admits a new entry, so os.CreateTemp succeeds, and
// refuses to remove or rename an existing one, so both the rename and the
// cleanup fail. The second subtest gets the other ending from an immutable
// destination, where the rename is refused but the temporary is still
// removable.
func TestReviewMerge_NamesTheTemporaryItCouldNotRemove(t *testing.T) {
	t.Parallel()

	const legalRow = `{"role":"implementer","severity":"high","class":"gap",` +
		`"location":"§1","finding":"the bound is unstated","evidence":"read §1"}`

	t.Run("the removal fails too: the message names what is left behind", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		role := writeFindingsFile(t, dir, "role-a.ndjson", []string{legalRow})
		locked := filepath.Join(dir, "locked")
		require.NoError(t, os.Mkdir(locked, 0o755))
		chflagsOrSkip(t, "uappnd", locked)

		// The fixture's property, asserted rather than assumed: the flag must
		// really admit a new entry and really refuse to move one, or the pair
		// this test is named for was never built and a pass would mean nothing.
		probe := filepath.Join(locked, "probe")
		require.NoError(t, os.WriteFile(probe, []byte("x"), 0o600),
			"an append-only directory must still admit a new file")
		if os.Rename(probe, filepath.Join(locked, "probe-moved")) == nil {
			t.Skip("the append-only flag is not enforced here, so neither failure can be provoked")
		}

		_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", "locked/out.ndjson", role)
		require.Equal(t, 3, code, "an unwritable -o is a file error: %s", stderr)

		// Read-back first: the message is only worth checking if there really
		// is a file it has to name.
		left := tempsLeftIn(t, locked)
		require.Len(t, left, 1, "the temporary survives, because the removal failed for the same reason the rename did")

		msg := mergeRefusalPayload(t, stderr)
		assert.Contains(t, msg, left[0],
			"the operator cannot delete a file tp never names")
		assert.Contains(t, msg, "could not be removed either",
			"a message that reports only the rename leaves the survivor unexplained")
	})

	t.Run("the removal succeeds: the write error stands alone", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		role := writeFindingsFile(t, dir, "role-a.ndjson", []string{legalRow})
		out := filepath.Join(dir, "out.ndjson")
		require.NoError(t, os.WriteFile(out, []byte("previous\n"), 0o644))
		chflagsOrSkip(t, "uchg", out)

		_, stderr, code := runTPMerge(t, dir, "review", "--merge", "-o", "out.ndjson", role)
		require.Equal(t, 3, code, "an immutable -o cannot be renamed over: %s", stderr)

		assert.Empty(t, tempsLeftIn(t, dir),
			"nothing is left behind, so there is nothing to name")
		assert.NotContains(t, mergeRefusalPayload(t, stderr), "could not be removed either",
			"a clause about a survivor must not appear when there is none")

		// The refusal left the destination as it found it.
		body, err := os.ReadFile(out)
		require.NoError(t, err)
		assert.Equal(t, "previous\n", string(body), "a refused merge does not touch -o (§5 row 10)")
	})
}
