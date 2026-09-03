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

// GroundRoundPath is where a recorded ground round lives:
// spec/.tp-review/<base>/ground-round-N.ndjson.
//
// It joins GroundSnapshotPath and GroundFloorPath for the same reason: the
// writer is in the engine and the reader that reports the path is in the CLI,
// so a second spelling would let the two disagree about the file --record just
// wrote. groundRoundFileName stays unexported behind it, so this is the only
// way out of the package to that name.
func GroundRoundPath(specPath string, round int) string {
	return filepath.Join(ReviewStateDir(specPath), groundRoundFileName(round))
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
// emit" would be unverifiable.
//
// The snapshot goes first, and that orders THIS call's two writes and nothing
// else. Each file is written atomically; the PAIR is not, and emit takes no
// lock — `--record` runs under WithFileLock(specPath), emit runs under nothing.
// So two emitters on one spec, or a reader running beside one, can find a
// snapshot and a floor derived from different texts. Per-file atomicity does
// not help, because the reachable harm is a mismatched pair rather than a torn
// file. An earlier version of this comment claimed the opposite — "a floor on
// disk always has the text it was derived from beside it" — and the same claim
// sat over the index derivation in runGround; both were false, and a falsified
// invariant left standing is what the next auditor spends a round on.
//
// The lock is deliberately NOT added. It is a new abstraction at a sink this
// release does not otherwise touch, and the hole needs two concurrent
// processes: `tp run` has no ground unit kind, so no unattended run reaches it.
//
// To rebuild the measurement rather than believe it: point a symlink at one of
// two specs with different text, take each one's floor from a lone emission as
// a reference, flip the symlink in a loop while N `tp ground` processes run on
// it, and classify the two files on disk against those references. Control the
// detector first or it measures nothing — it must read GREEN on each lone
// emission and RED on a hand-built snapshot-of-A/floor-of-B pair. Note that two
// different claims come out of the same rig and they are not interchangeable.
// Sampling the directory while writes are still in flight, which is what a
// concurrent reader does, shows the mismatch readily and in both directions.
// Sampling only after every writer has exited is a claim about the settled
// state, and that one is thinly supported: an audit round reported three such
// pairs, and re-running it at the commit that wrote this comment produced none
// in 140 settled iterations at 8 and 16 writers. The correction above does not
// rest on either count — the two writes below are independent and nothing
// orders one process's against another's.
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
