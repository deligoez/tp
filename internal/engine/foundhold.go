package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// PlanFoundApplyRecorded is PlanFoundApply over the requests apply may take
// now (spec/1.31.0.md §2): an audit unit's request is applied only after its
// round is recorded, so an unapplied request whose finder's audit round is
// above auditRecorded is held. A held request takes no part in the plan, dedup
// included, and stays in the store as it was filed.
func PlanFoundApplyRecorded(reqs []FoundRequest, tasks []model.Task, queue []FoundQueueEntry, foundMax, auditRecorded int) FoundApplyPlan {
	ready := make([]FoundRequest, 0, len(reqs))
	for i := range reqs {
		if !foundHeld(&reqs[i], auditRecorded) {
			ready = append(ready, reqs[i])
		}
	}
	plan := PlanFoundApply(ready, tasks, queue, foundMax)
	planned := make(map[string]FoundRequest, len(plan.Requests))
	for i := range plan.Requests {
		planned[plan.Requests[i].ID] = plan.Requests[i]
	}
	plan.Requests = make([]FoundRequest, len(reqs))
	for i := range reqs {
		plan.Requests[i] = reqs[i]
		if r, ok := planned[reqs[i].ID]; ok {
			plan.Requests[i] = r
		}
	}
	return plan
}

// foundHeld reports whether apply holds req: unapplied, and filed from an
// audit round later than the latest recorded one.
func foundHeld(req *FoundRequest, auditRecorded int) bool {
	return req.State == FoundStateUnapplied && req.Finder.AuditRound > auditRecorded
}

// RecordedAuditRound is the latest audit round recorded for specPath, 0 when
// none is or the spec's review state cannot be read: apply then holds every
// audit request rather than apply one mid-round.
func RecordedAuditRound(specPath string) int {
	if specPath == "" {
		return 0
	}
	st, err := LoadReviewState(specPath)
	if err != nil || st == nil {
		return 0
	}
	return latestRecordedRound(st.AuditRounds)
}
