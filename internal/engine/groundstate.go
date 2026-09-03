package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// groundSnapshotPhase is the namespace ground's snapshot takes in
// snapshotFilename, which produces "snapshot-ground-round-N.md" for it.
//
// It is a local constant and deliberately NOT a sixth entry in phase.go's
// lifecycle constants: Non-Goal 1 says grounding adds no phase to the oracle,
// and a value sitting in that block is one DetectPhase branch away from being
// one. What it names here is a filename namespace, nothing more.
const groundSnapshotPhase = "ground"

// groundRoundFileRe matches a recorded ground round's filename and captures its
// number.
//
// It is anchored, and the digits must end the name's first segment, so
// "ground-round-3x.ndjson" is not round 3. Anchoring is also what keeps the two
// artifacts emit writes — "snapshot-ground-round-N.md" and
// "floor-ground-round-N.txt" — out of the count: both contain the round file's
// name as a substring, and a search rather than a match would find them.
var groundRoundFileRe = regexp.MustCompile(`^ground-round-(\d+)(?:\.|$)`)

// NextGroundRound returns the number the next ground round on specPath takes:
// the highest existing ground-round-<N> plus one (§7.3), and 1 when the spec has
// no state directory yet — the first run of the ordering this release advocates,
// where grounding precedes review.
//
// A gap is preserved. A directory holding rounds 1 and 3 yields 4, never 2: the
// missing round is a deleted or lost artifact, and silently refilling its number
// would make two different rounds share an identifier. That is why the answer is
// the highest number present rather than the count of round files, which agree
// on an unbroken sequence and disagree on every other one.
//
// Only --record's round file numbers a round. Emit writes its snapshot and floor
// from the number this returns, so counting those would make the --record that
// follows an emit compute one more than the round its rows were graded against.
//
// An unreadable directory is an error rather than 1. It may hold anything, and
// answering 1 for it hands the next round a number an existing round already
// holds — precisely the collision above. hasAnyPrefixed refuses to guess on a
// directory it cannot list for the same reason.
func NextGroundRound(specPath string) (int, error) {
	entries, err := os.ReadDir(ReviewStateDir(specPath))
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, err
	}

	highest := 0
	for _, e := range entries {
		name := e.Name()
		// A crash mid-record leaves writeFileAtomic's unique sibling temp,
		// "<name>.<random>.tmp". That round was never recorded, so counting it
		// would skip a number.
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		m := groundRoundFileRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n > highest {
			highest = n
		}
	}
	return highest + 1, nil
}

// GroundSnapshotPath is where a ground round's snapshot lives:
// spec/.tp-review/<base>/snapshot-ground-round-N.md.
//
// The name comes from snapshotFilename rather than from a format string of its
// own, so ground's snapshot cannot drift out of the convention review's and
// audit's follow, and the "snapshot-ground-round-" entry §7.3 puts in
// groundFilePrefixes cannot stop matching the file it names.
func GroundSnapshotPath(specPath string, round int) string {
	return filepath.Join(ReviewStateDir(specPath), snapshotFilename(groundSnapshotPhase, round))
}

// GroundFloorPath is where a ground round's floor lives:
// spec/.tp-review/<base>/floor-ground-round-N.txt.
//
// This is the file --record validates against (§7.3). It is a path function
// rather than a literal at each call site because the writer and the reader are
// in different commands, and the one property that matters — that --record
// reads what the emission wrote — is the one a second spelling would break.
func GroundFloorPath(specPath string, round int) string {
	return filepath.Join(ReviewStateDir(specPath), fmt.Sprintf("floor-ground-round-%d.txt", round))
}

// WriteGroundEmission writes the two files §7.3 says an emission writes: the
// spec's text as this round read it, and the index derived from that text. The
// state directory is created when absent, which is the ordering this release
// advocates — grounding precedes review, so the first run on a spec has no
// spec/.tp-review/<base>/ to write into.
//
// Both are written here rather than one here and one at record, because a floor
// derived at record time would grade rows against a text its unit never saw: a
// spec edited mid-round would be silently re-floored, and "unit_id assigned at
// emit" would be unverifiable. The snapshot goes first so that a floor on disk
// always has the text it was derived from beside it.
//
// It writes no state.json and takes no state lock. A ground round is discovered
// by filename (§7.3, Non-Goal 2): SaveReviewState marshals a typed struct, so
// an index key this release added would be dropped by the next binary that does
// not know it.
func WriteGroundEmission(specPath string, round int, snapshot, floor []byte) error {
	if err := WriteSnapshotAtomic(specPath, groundSnapshotPhase, round, snapshot); err != nil {
		return err
	}
	return writeFileAtomic(GroundFloorPath(specPath, round), floor)
}
