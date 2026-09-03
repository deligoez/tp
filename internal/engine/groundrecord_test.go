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

