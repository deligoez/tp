package engine

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

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
