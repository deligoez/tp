package engine

import (
	"cmp"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// The decision unit's three answers to an undecided request (spec/1.31.0.md
// §5).
const (
	FoundAnswerTask     = "task"
	FoundAnswerConforms = "conforms"
	FoundAnswerHuman    = "human"
)

// FoundAnswer is a decision unit's answer to one undecided request. A task
// answer carries the Fix fields (a blank one falls back to the request's);
// task and conforms answers carry the cited file and the quote tp checked in
// it, and a conforms answer the reproduction that passes.
type FoundAnswer struct {
	Kind         string
	Location     string
	Reproduction string
	Expected     string
	Observed     string
	Cited        string
	Quote        string
}

// foundCitableNames are the files besides the spec a quote may cite
// (spec/1.31.0.md §5), matched by base name wherever they sit.
var foundCitableNames = []string{"CLAUDE.md", "CONVENTIONS.md", "REFERENCE.md"}

// FoundCitable reports whether a task or conforms answer may cite path: the
// spec itself, or a CLAUDE.md, CONVENTIONS.md or REFERENCE.md. Any other file
// would let a quote from arbitrary text settle a choice.
func FoundCitable(path, specPath string) bool {
	return filepath.Clean(path) == filepath.Clean(specPath) || slices.Contains(foundCitableNames, filepath.Base(path))
}

// FoundQuoteIn reports whether quote appears verbatim in text once runs of
// whitespace are collapsed on both sides, so a quote may span a line break of
// the cited file. A blank quote is never found: it would be in every text.
func FoundQuoteIn(text, quote string) bool {
	q := strings.Join(strings.Fields(quote), " ")
	return q != "" && strings.Contains(strings.Join(strings.Fields(text), " "), q)
}

// PlanFoundAnswer plans recording ans for the undecided request id
// (spec/1.31.0.md §5). A task answer is applied like a Fix-shaped request
// through PlanFoundApply: attached, deduplicated against other requests,
// capped by foundMax and placed, but never a duplicate of the request it
// answers. A conforms answer becomes a decided queue entry and a human one an
// open entry. Every answer moves the request out of the undecided state; the
// quote check is the caller's, before it calls this.
func PlanFoundAnswer(reqs []FoundRequest, tasks []model.Task, queue []FoundQueueEntry, foundMax int, id string, ans *FoundAnswer) (FoundApplyPlan, error) {
	idx := slices.IndexFunc(reqs, func(r FoundRequest) bool { return r.ID == id })
	if idx < 0 {
		return FoundApplyPlan{}, fmt.Errorf("request %s is not in the found store", id)
	}
	req := &reqs[idx]
	if req.State != FoundStateUndecided {
		return FoundApplyPlan{}, fmt.Errorf("request %s is %s, not undecided", id, req.State)
	}
	switch ans.Kind {
	case FoundAnswerTask:
		return planTaskAnswer(reqs, idx, tasks, queue, foundMax, ans)
	case FoundAnswerConforms, FoundAnswerHuman:
		plan := FoundApplyPlan{Entries: []FoundQueueEntry{answerEntry(req, ans)}, FoundTasks: CountFoundTasks(tasks)}
		plan.Requests = slices.Clone(reqs)
		plan.Requests[idx].State = FoundStateApplied
		return plan, nil
	}
	return FoundApplyPlan{}, fmt.Errorf("unknown answer %q: want task, conforms or human", ans.Kind)
}

// planTaskAnswer applies a task answer as the Fix-shaped request it makes of
// reqs[idx]. The plan runs over that request, now unapplied, and the requests
// apply has already settled: other unapplied ones wait for tp found apply,
// and the answered one is no longer undecided, so it cannot match itself.
func planTaskAnswer(reqs []FoundRequest, idx int, tasks []model.Task, queue []FoundQueueEntry, foundMax int, ans *FoundAnswer) (FoundApplyPlan, error) {
	answered := reqs[idx]
	answered.Location = cmp.Or(strings.TrimSpace(ans.Location), answered.Location)
	answered.Reproduction = cmp.Or(strings.TrimSpace(ans.Reproduction), answered.Reproduction)
	answered.Expected = cmp.Or(strings.TrimSpace(ans.Expected), answered.Expected)
	answered.Observed = cmp.Or(strings.TrimSpace(ans.Observed), answered.Observed)
	answered.Question, answered.Alternatives = "", nil
	answered.State = FoundStateUnapplied
	if !answered.FixShaped() {
		return FoundApplyPlan{}, fmt.Errorf("a task answer needs all four Fix fields (location, reproduction, expected, observed)")
	}
	sub := make([]FoundRequest, 0, len(reqs))
	for i := range reqs {
		switch {
		case i == idx:
			sub = append(sub, answered)
		case reqs[i].State != FoundStateUnapplied:
			sub = append(sub, reqs[i])
		}
	}
	plan := PlanFoundApply(sub, tasks, queue, foundMax)
	for i := range plan.Entries {
		if plan.Entries[i].Request == answered.ID {
			plan.Entries[i].Cited, plan.Entries[i].Quote = ans.Cited, ans.Quote
		}
	}
	settled := plan.Requests[slices.IndexFunc(plan.Requests, func(r FoundRequest) bool { return r.ID == answered.ID })]
	plan.Requests = slices.Clone(reqs)
	stored := &plan.Requests[idx]
	stored.Location, stored.Reproduction = answered.Location, answered.Reproduction
	stored.Expected, stored.Observed = answered.Expected, answered.Observed
	stored.State, stored.Outcome = settled.State, settled.Outcome
	return plan, nil
}

// answerEntry is the queue entry of a conforms or human answer. The queue is
// committed and the request store is not, so the entry carries the request
// itself; a conforms entry is decided and carries its evidence, and a human
// entry of a blocking request holds the finder its defect blocks.
func answerEntry(req *FoundRequest, ans *FoundAnswer) FoundQueueEntry {
	e := FoundQueueEntry{
		State:        FoundQueueOpen,
		Request:      req.ID,
		Location:     req.Location,
		Question:     req.Question,
		Alternatives: slices.Clone(req.Alternatives),
		Reproduction: cmp.Or(strings.TrimSpace(ans.Reproduction), req.Reproduction),
		Expected:     req.Expected,
		Observed:     req.Observed,
	}
	if ans.Kind == FoundAnswerConforms {
		e.State, e.Answer = FoundQueueDecided, FoundAnswerConforms
		e.Cited, e.Quote = ans.Cited, ans.Quote
		return e
	}
	if req.Blocks && req.Finder.Task != "" {
		e.Finders = []string{req.Finder.Task}
	}
	return e
}
