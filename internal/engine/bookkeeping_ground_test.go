package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// groundBookkeeping runs DeriveBookkeeping over the named files of a fresh
// project's .tp-review/ directory and returns its entries indexed by base name.
//
// The task file is deliberately left clean. The closure branch reads the task
// file at HEAD, and this fixture has no commits; the branch under test here is
// the round one, which reads nothing but the path.
func groundBookkeeping(t *testing.T, names ...string) map[string]BookkeepingEntry {
	t.Helper()
	start := t.TempDir()
	// A .git directory anchors ProjectRoot at start, so the paths below are
	// repo-root-relative in exactly the form DeriveBookkeeping is given them.
	require.NoError(t, os.MkdirAll(filepath.Join(start, ".git"), 0o755))
	specPath := filepath.Join(start, "spec.md")

	dirty := make([]string, 0, len(names))
	for _, name := range names {
		dirty = append(dirty, ".tp-review/spec/"+name)
	}

	bookkeeping, remaining := DeriveBookkeeping(
		start, filepath.Join(start, "spec.tasks.json"), specPath, dirty, &model.TaskFile{})
	require.Empty(t, remaining,
		"every fixture path is under the spec's .tp-review/ directory, so none is an unexplained change")
	require.Len(t, bookkeeping, len(names))

	byName := make(map[string]BookkeepingEntry, len(bookkeeping))
	for _, e := range bookkeeping {
		byName[filepath.Base(e.Path)] = e
	}
	return byName
}

// TestGroundArtifactsReportTheirRoundNumberLikeTheirSiblings is §7.3's widening
// of roundNumberRe. DeriveBookkeeping classifies every dirty path under
// .tp-review/ on the path prefix alone, so a ground artifact already reaches the
// round branch whatever the alternation says; what the alternation decides is
// whether its ref is the round number or the filename.
//
// The ground artifacts are compared against the review sibling's ref rather than
// against a literal, because "like its review and audit siblings" is the
// acceptance and a literal would keep passing if the shared convention moved.
// state.json rides in the same call as the control: it is the branch the
// filename fallback exists for, so an implementation that answered digits for
// everything is visible here rather than merely unasserted.
func TestGroundArtifactsReportTheirRoundNumberLikeTheirSiblings(t *testing.T) {
	const stateFile = "state.json"
	names := []string{
		"review-round-3.ndjson",
		"audit-round-3.ndjson",
		"ground-round-3.ndjson",
		"snapshot-ground-round-3.md",
		"floor-ground-round-3.txt",
		stateFile,
	}
	bk := groundBookkeeping(t, names...)

	sibling, ok := bk["review-round-3.ndjson"]
	require.True(t, ok, "the review sibling is the convention the ground artifacts are compared against")
	require.Equal(t, "3", sibling.Ref, "a review round reports its number")

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			entry, found := bk[name]
			require.True(t, found)
			assert.Equal(t, BookkeepingRound, entry.Kind,
				"everything under .tp-review/ is round bookkeeping")

			if name == stateFile {
				assert.Equal(t, stateFile, entry.Ref,
					"a file carrying no round number falls back to its base name")
				return
			}
			assert.NotEqual(t, name, entry.Ref, "a round's ref is its number, not its filename")
			assert.Equal(t, sibling.Ref, entry.Ref,
				"every round-3 artifact reports the ref its review sibling reports")
		})
	}
}

// TestNextGroundRoundIsNotNumberedByTheBookkeepingRegex is the acceptance's
// second half: widening roundNumberRe widens the bookkeeping reference and
// nothing else. Numbering keeps its own matcher, which is anchored on
// ground-round-<N>, while roundNumberRe is unanchored and matches review and
// audit rounds by construction — so numbering routed through it would read the
// review round 9 below and hand the next ground round the number 10.
//
// This guard is green before the widening as well as after; the mutant it exists
// for is numbering that shares the bookkeeping regex.
func TestNextGroundRoundIsNotNumberedByTheBookkeepingRegex(t *testing.T) {
	names := []string{"review-round-9.ndjson", "audit-round-7.ndjson", "snapshot-round-9.md"}
	for _, name := range names {
		require.Regexp(t, roundNumberRe, name,
			"the fixture only discriminates while the bookkeeping regex matches %s", name)
	}

	n, err := NextGroundRound(groundRoundFixture(t, names...))
	require.NoError(t, err)
	assert.Equal(t, 1, n, "a review or audit round is not a ground round")
}
