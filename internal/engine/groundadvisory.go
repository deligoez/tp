package engine

import (
	"os"
	"regexp"
	"strconv"
)

// GroundAdvisory is §9's `ungrounded`: what `tp review` says once, in its
// top-level envelope, when the spec's claims have not all been checked against
// the world.
//
// It reports and does not refuse. Nothing here reaches an exit code, which is
// what makes §11 row 17's first half — the exit code is identical with and
// without ungrounded units — a property of the shape rather than of a branch
// somebody remembered not to write.
//
// The ratio is two integers and not a fraction, for the reason GroundCoverage
// gives: a fraction has to decide what to do about an empty floor, and a reader
// wanting one can divide.
type GroundAdvisory struct {
	// Round is the ground round these counts are about.
	Round int `json:"round"`
	// Undispositioned is how many emitted floor units no recorded row decided.
	Undispositioned int `json:"undispositioned"`
	// FloorSize is §8's denominator: the floor units the emission emitted. A
	// cut unit is not one of them (§2.2), so this is smaller than the index.
	FloorSize int `json:"floor_size"`
}

// groundFloorFileRe matches an emitted floor's filename and captures its round.
//
// Anchored at both ends, like groundRoundFileRe and for the same reason: a
// search would find "snapshot-ground-round-1.md" too, and the two files an
// emission writes would each answer for the round.
var groundFloorFileRe = regexp.MustCompile(`^floor-ground-round-(\d+)\.txt$`)

// LatestGroundAdvisory answers §9 for specPath: the latest ground round's
// coverage, or nil when there is nothing to say.
//
// **A round exists from its EMISSION, not from its record.** That is the
// reading of "the latest ground round" §9 leaves open, and it is the one under
// which the advisory says something in the state that most needs saying:
// between `tp ground` and `tp ground --record` nothing in the round has been
// decided, and review is about to be told the whole document is authoritative.
// Keying on the latest recorded round instead reports the PREVIOUS round's
// complete coverage and drops the key, so a spec whose current round nobody has
// come back to would read as fully grounded.
//
// nil, and never an error, on every failure it can meet — no emission, an
// unreadable state directory, an index or a round file that does not parse.
// §9 says review is told and not stopped: an advisory that could fail a command
// would be a gate, which Non-Goal 3 forbids, and a count derived from a file
// this could not read would be worse than silence.
func LatestGroundAdvisory(specPath string) *GroundAdvisory {
	round := latestEmittedGroundRound(specPath)
	if round == 0 {
		return nil
	}

	floor, err := readGroundFloor(specPath, round)
	if err != nil {
		return nil
	}
	rows, err := readGroundRoundRows(specPath, round)
	if err != nil {
		return nil
	}

	cov := GroundCoverageOf(floor, rows)
	open := cov.Emitted - cov.Dispositioned
	if open == 0 {
		return nil
	}
	return &GroundAdvisory{Round: round, Undispositioned: open, FloorSize: cov.Emitted}
}

// latestEmittedGroundRound is the highest round whose floor is on disk, or 0
// when the spec has had no emission.
//
// The floor is what it looks for rather than the round file, because §9's
// advisory is about the round a reader is inside: --record cannot write a round
// whose floor is absent, so this is never behind the recorded number.
func latestEmittedGroundRound(specPath string) int {
	entries, err := os.ReadDir(ReviewStateDir(specPath))
	if err != nil {
		return 0
	}
	highest := 0
	for _, e := range entries {
		m := groundFloorFileRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if n, convErr := strconv.Atoi(m[1]); convErr == nil && n > highest {
			highest = n
		}
	}
	return highest
}

// readGroundFloor reads the index that round's emission froze (§7.3).
func readGroundFloor(specPath string, round int) ([]FloorIndexRow, error) {
	data, err := os.ReadFile(GroundFloorPath(specPath, round))
	if err != nil {
		return nil, err
	}
	return ParseFloorIndex(string(data))
}

// readGroundRoundRows reads the round's recorded dispositions, answering none
// for a round that has been emitted and not recorded.
//
// An absent file is that state and not a failure — it is every round between
// `tp ground` and `tp ground --record` — so it returns no rows and no error,
// and coverage over no rows is the honest answer: nothing in this round has
// been decided yet.
func readGroundRoundRows(specPath string, round int) ([]GroundRow, error) {
	data, err := os.ReadFile(GroundRoundPath(specPath, round))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return ParseGroundRows(data)
}
