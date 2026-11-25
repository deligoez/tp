package engine

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// dedupRequest is a Fix-shaped request in the given state, filed in arrival
// order seq.
func dedupRequest(seq int, location, reproduction, state string) FoundRequest {
	return FoundRequest{
		ID:           "F" + strconv.Itoa(seq),
		Seq:          seq,
		Location:     location,
		Reproduction: reproduction,
		Expected:     "exit 0",
		Observed:     "exit 1",
		Finder:       FoundFinder{Task: "finder"},
		State:        state,
	}
}

func keptIDs(kept []FoundRequest) []string {
	ids := make([]string, 0, len(kept))
	for i := range kept {
		ids = append(ids, kept[i].ID)
	}
	return ids
}

// §10 row "Dedup keys on the reproduction": one defect filed at two lines of
// one file is one request. The mutant that keys on file:line keeps both.
func TestDedupFoundRequests_OneReproductionAtTwoLinesOfAFileKeepsOne(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "internal/engine/x.go:40", "go test ./internal/engine -run TestX", FoundStateUnapplied),
		dedupRequest(2, "internal/engine/x.go:44", "go test  ./internal/engine\n -run TestX ", FoundStateUnapplied),
	}

	kept, dups := DedupFoundRequests(reqs, nil, nil)

	assert.Equal(t, []string{"F1"}, keptIDs(kept))
	require.Len(t, dups, 1)
	assert.Equal(t, FoundDuplicate{Request: "F2", KeptRequest: "F1"}, dups[0])
}

func TestDedupFoundRequests_TheSameReproductionInAnotherFileIsNotADuplicate(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "internal/engine/x.go:40", "go test ./...", FoundStateUnapplied),
		dedupRequest(2, "internal/engine/y.go:40", "go test ./...", FoundStateUnapplied),
	}

	kept, dups := DedupFoundRequests(reqs, nil, nil)

	assert.Equal(t, []string{"F1", "F2"}, keptIDs(kept))
	assert.Empty(t, dups)
}

func TestDedupFoundRequests_KeepsTheEarliestArrivalWhateverTheStoreOrder(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(2, "a.go:9", "make check", FoundStateUnapplied),
		dedupRequest(1, "a.go:3", "make check", FoundStateUnapplied),
	}

	kept, dups := DedupFoundRequests(reqs, nil, nil)

	assert.Equal(t, []string{"F1"}, keptIDs(kept))
	assert.Equal(t, []FoundDuplicate{{Request: "F2", KeptRequest: "F1"}}, dups)
}

// Round-4 finding dedup-pending-outcome-gap: a request still with the
// decision unit has no outcome yet, and a second request for its defect is a
// duplicate of it, not a second request for the decision unit.
func TestDedupFoundRequests_ARequestMatchingAnUndecidedOneIsADuplicate(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "a.go:3", "make check", FoundStateUndecided),
		dedupRequest(2, "a.go:7", "make check", FoundStateUnapplied),
	}

	kept, dups := DedupFoundRequests(reqs, nil, nil)

	assert.Empty(t, kept)
	assert.Equal(t, []FoundDuplicate{{Request: "F2", KeptRequest: "F1"}}, dups)
}

// Round-4 finding dedup-against-queued-request: an open queue entry is an open
// outcome, so a request for its defect adds no second entry.
func TestDedupFoundRequests_ARequestMatchingAnOpenQueueEntryIsADuplicate(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "a.go:3", "make check", FoundStateApplied),
		dedupRequest(2, "a.go:7", "make check", FoundStateUnapplied),
	}
	queue := []FoundQueueEntry{{ID: "Q1", State: FoundQueueOpen, Request: "F1", Location: "a.go:3"}}

	kept, dups := DedupFoundRequests(reqs, nil, queue)

	assert.Empty(t, kept)
	assert.Equal(t, []FoundDuplicate{{Request: "F2", KeptRequest: "F1", KeptEntry: "Q1"}}, dups)
}

func TestDedupFoundRequests_ADecidedQueueEntryIsNotAnOpenOutcome(t *testing.T) {
	reqs := []FoundRequest{
		dedupRequest(1, "a.go:3", "make check", FoundStateApplied),
		dedupRequest(2, "a.go:7", "make check", FoundStateUnapplied),
	}
	queue := []FoundQueueEntry{{ID: "Q1", State: FoundQueueDecided, Request: "F1", Location: "a.go:3", Answer: "conforms"}}

	kept, dups := DedupFoundRequests(reqs, nil, queue)

	assert.Equal(t, []string{"F2"}, keptIDs(kept))
	assert.Empty(t, dups)
}

func TestDedupFoundRequests_AnEntryNamingAnUnknownRequestMatchesNothing(t *testing.T) {
	reqs := []FoundRequest{dedupRequest(2, "a.go:7", "make check", FoundStateUnapplied)}
	queue := []FoundQueueEntry{{ID: "Q1", State: FoundQueueOpen, Request: "F9", Location: "a.go:3"}}

	kept, dups := DedupFoundRequests(reqs, nil, queue)

	assert.Equal(t, []string{"F2"}, keptIDs(kept))
	assert.Empty(t, dups)
}

func TestDedupFoundRequests_ARequestMatchingAnOpenOrWipTasksRequestIsADuplicate(t *testing.T) {
	for _, status := range []string{model.StatusOpen, model.StatusWIP} {
		t.Run(status, func(t *testing.T) {
			applied := dedupRequest(1, "a.go:3", "make check", FoundStateApplied)
			applied.Outcome = "found-f1"
			reqs := []FoundRequest{applied, dedupRequest(2, "a.go:7", "make check", FoundStateUnapplied)}
			tasks := []model.Task{{ID: "found-f1", Status: status}}

			kept, dups := DedupFoundRequests(reqs, tasks, nil)

			assert.Empty(t, kept)
			assert.Equal(t, []FoundDuplicate{{Request: "F2", KeptRequest: "F1", KeptTask: "found-f1"}}, dups)
		})
	}
}

// A done task's request has ended, and so has a handed-back one's: handing
// back closes the task as done, so this case covers it once its queue entry
// is no longer open.
func TestDedupFoundRequests_ARequestMatchingADoneTasksRequestIsNotADuplicate(t *testing.T) {
	applied := dedupRequest(1, "a.go:3", "make check", FoundStateApplied)
	applied.Outcome = "found-f1"
	reqs := []FoundRequest{applied, dedupRequest(2, "a.go:7", "make check", FoundStateUnapplied)}
	tasks := []model.Task{{ID: "found-f1", Status: model.StatusDone}}

	kept, dups := DedupFoundRequests(reqs, tasks, nil)

	assert.Equal(t, []string{"F2"}, keptIDs(kept))
	assert.Empty(t, dups)
}

func TestFoundDedupKey_StripsOnlyATrailingLineNumber(t *testing.T) {
	assert.Equal(t, FoundDedupKey("a.go:40", "x  y"), FoundDedupKey("a.go:44", "x y"))
	assert.Equal(t, FoundDedupKey("a.go", "x"), FoundDedupKey("a.go:1", "x"))
	assert.NotEqual(t, FoundDedupKey("a.go:40", "x"), FoundDedupKey("b.go:40", "x"))
	assert.NotEqual(t, FoundDedupKey("a.go:40", "x"), FoundDedupKey("a.go:40", "y"))
	assert.NotEqual(t, FoundDedupKey("c:/a.go", "x"), FoundDedupKey("c", "x"))
}
