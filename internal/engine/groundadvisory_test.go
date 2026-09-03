package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundAdvisorySpec writes a spec in a fresh directory and returns its path.
// The text is never read by the advisory — §7.3 has it grade the EMITTED floor
// and never the spec as it now stands — so the file exists only to give
// ReviewStateDir a base to derive the state directory from.
func groundAdvisorySpec(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))
	return specPath
}

// emitGroundFloor writes what an emission writes for a round: the rendered
// index. It goes through FormatFloorIndex rather than a literal, so the fixture
// is the file tp itself writes — commit line, rows and summary line — and not a
// test author's idea of one.
func emitGroundFloor(t *testing.T, specPath string, round int, rows []FloorIndexRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll(ReviewStateDir(specPath), 0o755))
	require.NoError(t, os.WriteFile(GroundFloorPath(specPath, round),
		[]byte(FormatFloorIndex("c0ffee123456", rows)), 0o600))
}

// recordGroundDispositions writes a round file holding one row per index row
// handed in, copying that row's own anchor, hash and ordinal so the record is
// joined to the floor the way a real one is.
//
// Every row is NOT-A-CLAIM, on which §7.2 makes kind and tier optional. What a
// row SAYS is deliberately irrelevant here: §8 counts units DECIDED, and every
// one of §3's six verdicts is a disposition.
func recordGroundDispositions(t *testing.T, specPath string, round int, rows ...FloorIndexRow) {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, `{"unit_id":%q,"anchor":%q,"text_sha":%q,"ordinal":%d,"verdict":"NOT-A-CLAIM"}`+"\n",
			r.ID, r.Anchor, r.TextSHA, r.Ordinal)
	}
	require.NoError(t, os.MkdirAll(ReviewStateDir(specPath), 0o755))
	require.NoError(t, os.WriteFile(GroundRoundPath(specPath, round), []byte(b.String()), 0o600))

	// The fixture must be a record tp would accept, or a test built on it says
	// nothing about a real round.
	data, err := os.ReadFile(GroundRoundPath(specPath, round))
	require.NoError(t, err)
	_, err = ParseGroundRows(data)
	require.NoError(t, err, "the fixture must be a round §7.2 accepts")
}

// TestASpecWithNoGroundRoundHasNoAdvisory: §9 makes the key absent when no
// ground round exists, so there is no round number to name and no floor to
// count against. This is every spec before its first `tp ground`.
func TestASpecWithNoGroundRoundHasNoAdvisory(t *testing.T) {
	assert.Nil(t, LatestGroundAdvisory(groundAdvisorySpec(t)),
		"no emission, so nothing to say")
}

