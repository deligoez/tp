package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteSnapshotAtomicSurvivesParallelSiblings is v0.36.0 audit round 7's
// only code defect, pinned where it happens.
//
// `skills/tp/REFERENCE.md` runs review-role and audit-role units "parallel with
// sibling roles", and every one of them emits, so every one of them writes the
// round's snapshot. With a fixed `final + ".tmp"` they share one temp path and
// race to rename it: the winner moves it away and every loser's rename fails
// with ENOENT. Measured on the real four-role panel through the brief_command
// strings `tp run --dry-run` emits: 0 of 20 failures run sequentially, 5 of 40
// review and 9 of 40 audit run in parallel, 24 of 80 at eight-way concurrency.
//
// The repo already holds the answer, with this exact failure written down --
// engine.writeFileAtomic uses os.CreateTemp because "two writers sharing one
// temp path race to rename it and the loser's rename fails after the winner has
// moved it away". reviewstate.go did not.
//
// The fixed name is v0.29.0 §10.2's, written before v0.35.0 made siblings
// concurrent; this is that contract meeting that parallelism.
func TestWriteSnapshotAtomicSurvivesParallelSiblings(t *testing.T) {
	for _, phase := range []string{PhaseReview, PhaseAudit} {
		t.Run(phase, func(t *testing.T) {
			dir := t.TempDir()
			spec := filepath.Join(dir, "spec.md")
			require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))

			// Eight writers, the concurrency at which the audit measured 24
			// failures in 80 attempts. Every sibling writes the same round's
			// snapshot with the same bytes -- that is the real arrangement, not
			// a contrived one.
			const writers = 8
			body := []byte("# Spec\n\nthe round's snapshot\n")

			var wg sync.WaitGroup
			errs := make([]error, writers)
			start := make(chan struct{})
			for i := 0; i < writers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-start
					errs[i] = WriteSnapshotAtomic(spec, phase, 1, body)
				}(i)
			}
			close(start)
			wg.Wait()

			for i, err := range errs {
				assert.NoError(t, err, "sibling %d must not lose a rename race", i)
			}

			// The snapshot is whole, and no temp file is left behind for the
			// next reader to trip over.
			final := filepath.Join(ReviewStateDir(spec), snapshotFilename(phase, 1))
			got, err := os.ReadFile(final) //nolint:gosec // a path this test built
			require.NoError(t, err, "the snapshot must exist after the race")
			assert.Equal(t, body, got, "and must be whole, not half-written")

			entries, err := os.ReadDir(ReviewStateDir(spec))
			require.NoError(t, err)
			for _, e := range entries {
				assert.NotContains(t, e.Name(), ".tmp",
					"no temp file survives the race: %s", e.Name())
			}
		})
	}
}

// TestWriteSnapshotAtomicIsWholeUnderARewrite guards the property the temp file
// exists for at all: a reader taking no lock sees the old bytes or the new
// ones, never a prefix.
//
// Separate from the race above because the two fail for different reasons — a
// unique temp name fixes the race and would not, on its own, tell anyone the
// write is still atomic.
func TestWriteSnapshotAtomicIsWholeUnderARewrite(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))

	final := filepath.Join(ReviewStateDir(spec), snapshotFilename(PhaseReview, 1))
	for i := 1; i <= 20; i++ {
		body := []byte(fmt.Sprintf("# round %d\n%s\n", i, string(make([]byte, i*512))))
		require.NoError(t, WriteSnapshotAtomic(spec, PhaseReview, 1, body))

		got, err := os.ReadFile(final) //nolint:gosec // a path this test built
		require.NoError(t, err)
		assert.Equal(t, body, got, "rewrite %d must land whole", i)
	}
}
