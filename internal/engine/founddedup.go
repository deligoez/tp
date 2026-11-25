package engine

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// FoundDuplicate is an unapplied request tp found apply opens nothing for
// (spec/1.31.0.md §2): it names the request and the kept one whose defect it
// repeats, plus that request's open outcome, a task or a queue entry, when it
// has one. A kept request in the same batch, or one still with the decision
// unit, has neither yet; the duplicate's finder is placed once it does.
type FoundDuplicate struct {
	Request     string
	KeptRequest string
	KeptTask    string
	KeptEntry   string
}

// trailingLine matches the line part of a file:line location.
var trailingLine = regexp.MustCompile(`:\d+$`)

// FoundDedupKey is what two requests share when they report one defect: the
// location's file, without its line, and the reproduction with its whitespace
// collapsed. Keying on file:line would split one defect filed at two lines
// and merge two defects filed at one (spec/1.31.0.md §2, Rejected).
func FoundDedupKey(location, reproduction string) string {
	file := trailingLine.ReplaceAllString(strings.TrimSpace(location), "")
	return file + "\x00" + strings.Join(strings.Fields(reproduction), " ")
}

func foundRequestKey(req *FoundRequest) string {
	return FoundDedupKey(req.Location, req.Reproduction)
}

// DedupFoundRequests splits a spec's unapplied requests into the ones apply
// acts on and the duplicates it opens nothing for (spec/1.31.0.md §2). A
// request is a duplicate when its key matches an earlier request of its batch,
// a request still undecided, an open queue entry's request, or an applied
// request whose outcome task is open or wip. A request whose outcome has ended
// (its task done, a handed-back task included, or its entry decided) is not
// matched. Kept requests come back in arrival order.
func DedupFoundRequests(reqs []FoundRequest, tasks []model.Task, queue []FoundQueueEntry) (kept []FoundRequest, dups []FoundDuplicate) {
	open := openFoundOutcomes(reqs, tasks, queue)
	batch := make([]FoundRequest, 0, len(reqs))
	for i := range reqs {
		if reqs[i].State == FoundStateUnapplied {
			batch = append(batch, reqs[i])
		}
	}
	slices.SortStableFunc(batch, func(a, b FoundRequest) int { return cmp.Compare(a.Seq, b.Seq) })
	for i := range batch {
		key := foundRequestKey(&batch[i])
		if outcome, ok := open[key]; ok {
			outcome.Request = batch[i].ID
			dups = append(dups, outcome)
			continue
		}
		open[key] = FoundDuplicate{KeptRequest: batch[i].ID}
		kept = append(kept, batch[i])
	}
	return kept, dups
}

// openFoundOutcomes indexes by key the requests whose defect is still open
// outside the batch: undecided ones, ones an open queue entry holds, and
// applied ones whose outcome task is open or wip.
func openFoundOutcomes(reqs []FoundRequest, tasks []model.Task, queue []FoundQueueEntry) map[string]FoundDuplicate {
	byID := make(map[string]*FoundRequest, len(reqs))
	for i := range reqs {
		byID[reqs[i].ID] = &reqs[i]
	}
	open := map[string]FoundDuplicate{}
	for i := range reqs {
		req := &reqs[i]
		switch {
		case req.State == FoundStateUndecided:
			open[foundRequestKey(req)] = FoundDuplicate{KeptRequest: req.ID}
		case req.State == FoundStateApplied && foundTaskIsOpen(tasks, req.Outcome):
			open[foundRequestKey(req)] = FoundDuplicate{KeptRequest: req.ID, KeptTask: req.Outcome}
		}
	}
	for i := range queue {
		req, ok := byID[queue[i].Request]
		if queue[i].State != FoundQueueOpen || !ok {
			continue
		}
		key := foundRequestKey(req)
		if _, taken := open[key]; !taken {
			open[key] = FoundDuplicate{KeptRequest: req.ID, KeptEntry: queue[i].ID}
		}
	}
	return open
}

// foundTaskIsOpen reports whether id names a task that is open or wip.
func foundTaskIsOpen(tasks []model.Task, id string) bool {
	if id == "" {
		return false
	}
	for i := range tasks {
		if tasks[i].ID == id {
			return tasks[i].Status != model.StatusDone
		}
	}
	return false
}
