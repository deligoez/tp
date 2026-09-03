package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundEmittedDir builds the tree an emission leaves behind (§7.3: emit writes
// the snapshot and the floor, --record adds the round file beside them) and
// returns the spec path.
//
// The directory is deliberately not empty. §11 row 10's claim is about a
// directory "byte-identical to before the attempt", and a digest over an empty
// directory is the same digest whatever a failing implementation deletes from
// it; the two artifacts here are the ones a real record runs beside.
func groundEmittedDir(t *testing.T, round int) string {
	t.Helper()
	root := t.TempDir()
	specPath := filepath.Join(root, "spec", "1.0.0.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(specPath), 0o755))
	require.NoError(t, os.WriteFile(specPath, []byte("# spec\n"), 0o600))

	dir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, fmt.Sprintf("snapshot-ground-round-%d.md", round)),
		[]byte("# spec\n"), 0o600))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, fmt.Sprintf("floor-ground-round-%d.txt", round)),
		[]byte("u1\t§0\t0123456789ab\t1\t24B\n"), 0o600))
	return specPath
}

// groundDirDigest hashes a directory tree — every relative path, its permission
// bits and, for a file, its whole content — so a single string stands for
// "byte-identical". Any addition, deletion, truncation or in-place edit changes
// it.
func groundDirDigest(t *testing.T, dir string) string {
	t.Helper()
	h := sha256.New()
	files := 0
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		info, infoErr := d.Info()
		if infoErr != nil {
			return infoErr
		}
		fmt.Fprintf(h, "%s\x00%v\x00%o\x00", rel, d.IsDir(), info.Mode().Perm())
		if d.IsDir() {
			return nil
		}
		files++
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		h.Write(data)
		return nil
	}))
	require.GreaterOrEqual(t, files, 2,
		"the digest must cover the artifacts emit wrote, or it cannot witness their survival")
	return hex.EncodeToString(h.Sum(nil))
}

// groundRecordPayload joins NDJSON lines into the bytes of a --record file,
// newline-terminated.
func groundRecordPayload(lines ...[]byte) []byte {
	out := make([]byte, 0, 256)
	for _, l := range lines {
		out = append(out, l...)
		out = append(out, '\n')
	}
	return out
}

// groundNumberedRow is a legal row for unit u<n>, distinct from its neighbours
// so a test can tell which rows survived.
func groundNumberedRow(t *testing.T, n int) []byte {
	t.Helper()
	return groundWireRow(t, groundClaimRow(), map[string]any{
		"unit_id": fmt.Sprintf("u%d", n),
	}, nil)
}

// TestARecordedRoundIsWrittenWhenEveryRowValidates is the half that stops the
// atomicity test below being a tautology.
//
// An implementation that never writes anything satisfies "one invalid row
// writes no round file" perfectly. This pins the other side: on a payload whose
// every row validates, the round file appears at the name §7.3 gives it,
// carrying the bytes the unit wrote, and every row comes back in file order.
func TestARecordedRoundIsWrittenWhenEveryRowValidates(t *testing.T) {
	specPath := groundEmittedDir(t, 2)
	payload := groundRecordPayload(
		groundNumberedRow(t, 1),
		groundNumberedRow(t, 2),
		groundNumberedRow(t, 3),
	)

	rows, err := RecordGroundRound(specPath, 2, payload)
	require.NoError(t, err)

	require.Len(t, rows, 3, "every row is returned, not just the first")
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		require.NotNil(t, r.UnitID)
		ids = append(ids, *r.UnitID)
	}
	assert.Equal(t, []string{"u1", "u2", "u3"}, ids, "rows come back in file order")

	written, err := os.ReadFile(filepath.Join(ReviewStateDir(specPath), "ground-round-2.ndjson"))
	require.NoError(t, err, "a round whose rows all validate is written as ground-round-<N>.ndjson")
	assert.Equal(t, string(payload), string(written),
		"the recorded round is the rows as the unit wrote them")
}

// TestOneInvalidRowWritesNoRoundFileAndLeavesTheDirectoryByteIdentical is §11
// row 10.
//
// Each case puts the one invalid row somewhere different, and the position is
// the point rather than the decoration. A validate-as-you-write implementation
// passes the "invalid first" case — there was nothing to write before it — and
// fails the other three, where two legal rows precede the bad one. The
// unknown-key case binds this task's second acceptance to the first: a
// misspelled `carried_from` is a rejection, and a rejection writes nothing.
func TestOneInvalidRowWritesNoRoundFileAndLeavesTheDirectoryByteIdentical(t *testing.T) {
	good1, good2 := groundNumberedRow(t, 1), groundNumberedRow(t, 2)
	unknownKey := groundWireRow(t, groundClaimRow(), map[string]any{"carried_form": 1}, nil)
	badCell := groundWireRow(t, groundClaimRow(), map[string]any{"ordinal": 0}, nil)

	cases := []struct {
		name    string
		payload []byte
	}{
		{"invalid row first", groundRecordPayload(badCell, good1, good2)},
		{"invalid row last", groundRecordPayload(good1, good2, badCell)},
		{"unknown top-level key on the last row", groundRecordPayload(good1, good2, unknownKey)},
		{"a trailing partial line", append(groundRecordPayload(good1, good2), '{')},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := groundEmittedDir(t, 3)
			dir := ReviewStateDir(specPath)
			before := groundDirDigest(t, dir)

			rows, err := RecordGroundRound(specPath, 3, tc.payload)
			require.Error(t, err, "a payload holding one invalid row is rejected")
			assert.Nil(t, rows, "a rejected round yields no rows")

			_, statErr := os.Stat(filepath.Join(dir, "ground-round-3.ndjson"))
			assert.True(t, os.IsNotExist(statErr),
				"no round file may exist: a partially valid round would make coverage a lie")
			assert.Equal(t, before, groundDirDigest(t, dir),
				"the state directory must be byte-identical to before the attempt")
		})
	}
}

// TestEveryLineIsValidatedAndTheRejectionNamesItsLine holds two properties one
// test can decide together, because the same counter carries both.
//
// The invalid row is the third line and the second row: the blank line between
// them means an implementation numbering rows rather than lines reports 2, and
// an implementation that stops after the first row reports nothing at all.
func TestEveryLineIsValidatedAndTheRejectionNamesItsLine(t *testing.T) {
	payload := groundRecordPayload(
		groundNumberedRow(t, 1),
		[]byte("   "),
		groundWireRow(t, groundClaimRow(), map[string]any{"tier": "guess"}, nil),
	)

	_, err := ParseGroundRows(payload)
	require.Error(t, err)

	var lineErr *GroundLineError
	require.ErrorAs(t, err, &lineErr, "a rejection names the line of the file it is about")
	assert.Equal(t, 3, lineErr.Line, "line numbers count the file's lines, blanks included")

	var rowErr *GroundRowError
	require.ErrorAs(t, err, &rowErr, "the cell failure survives the line wrapper")
	assert.Equal(t, "tier", rowErr.Field)
}

// TestAnUnknownTopLevelKeyIsRejectedNamingIt is §7.2's unknown-key rule, on
// allowedRoleKeys' precedent.
//
// The same base row is parsed with and without the offending key, so the
// rejection is attributable to the key rather than to anything else the fixture
// happens to carry — without the second half a row that was invalid for some
// other reason would pass this test.
func TestAnUnknownTopLevelKeyIsRejectedNamingIt(t *testing.T) {
	for _, key := range []string{"carried_form", "unitid", "severity", "role"} {
		t.Run(key, func(t *testing.T) {
			base := groundClaimRow()
			_, err := ParseGroundRow(groundWireRow(t, base, nil, nil))
			require.NoError(t, err, "the base row must be legal, or the rejection proves nothing")

			_, err = ParseGroundRow(groundWireRow(t, base, map[string]any{key: "x"}, nil))
			require.Error(t, err, "a key that is no cell of §7.2's table is rejected, not ignored")

			var rowErr *GroundRowError
			require.ErrorAs(t, err, &rowErr)
			assert.Equal(t, key, rowErr.Field, "the rejection names the offending key")
		})
	}
}

// TestARecordedRoundIsTheOneTheNumberingCounts holds the name --record writes
// accountable to the code that reads it back.
//
// Two places spell `ground-round-<N>.ndjson`: the formatter this file uses and
// groundRoundFileRe, which NextGroundRound counts with. Asserting the name as a
// literal in both would let them agree with the literal and disagree with each
// other, and the cost of that is not cosmetic — a round whose file the counter
// cannot see is a round the next one silently overwrites.
func TestARecordedRoundIsTheOneTheNumberingCounts(t *testing.T) {
	specPath := groundEmittedDir(t, 1)
	require.Equal(t, 1, mustNextGroundRound(t, specPath),
		"an emitted-but-unrecorded round is still the round to record")

	_, err := RecordGroundRound(specPath, 1, groundRecordPayload(groundNumberedRow(t, 1)))
	require.NoError(t, err)

	assert.Equal(t, 2, mustNextGroundRound(t, specPath),
		"the file --record writes is the file NextGroundRound counts")
}

func mustNextGroundRound(t *testing.T, specPath string) int {
	t.Helper()
	n, err := NextGroundRound(specPath)
	require.NoError(t, err)
	return n
}

// TestTheAllowedKeySetIsExactlySection72sTable binds the key set to the
// artifact it is derived from, in both directions.
//
// A cell added to §7.2 with no entry here would be rejected as unknown on every
// row that carried it; an entry here that §7.2 does not have would let a key
// through that no cell validates. Neither can be found by reading the map.
func TestTheAllowedKeySetIsExactlySection72sTable(t *testing.T) {
	inSpec := groundSection72Fields(t)
	require.NotEmpty(t, inSpec, "§7.2's table must be readable for this to be checkable")

	allowed := make([]string, 0, len(groundRowKeys))
	for k := range groundRowKeys {
		allowed = append(allowed, k)
	}
	slices.Sort(allowed)

	assert.ElementsMatch(t, inSpec, allowed,
		"the accepted top-level keys are exactly the fields §7.2's table names")
}
