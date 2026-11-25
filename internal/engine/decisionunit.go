package engine

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

// DecisionBatchMax is how many undecided requests one decision unit takes
// (spec/1.31.0.md §5). It is not found_max: that caps found tasks, this the
// work one unit is handed.
const DecisionBatchMax = 5

// decisionIDSep joins a decision unit's request ids into its unit id, the
// durable subject TP_UNIT_ID carries and its brief names.
const decisionIDSep = ","

// DecisionUnitText is what a decision unit is told beside its brief
// (spec/1.31.0.md §5): the three answers, human when in doubt, and that a
// rule-9 decision is never its to make.
const DecisionUnitText = " Answer each request with `tp found answer`: `task` or `conforms` only where a sentence you quote verbatim from the spec," +
	" a CLAUDE.md, CONVENTIONS.md or REFERENCE.md settles it, `human` for everything else; when in doubt, answer `human`." +
	" You never make a rule-9 decision (an operator decision such as raising a cap or accepting a finding): answer `human` instead."

// UndecidedRequestIDs returns the ids of up to limit undecided requests of the
// task file's cycle, in arrival order. A store that cannot be read holds none
// the run can act on.
func UndecidedRequestIDs(taskFile string, limit int) []string {
	reqs, err := ReadFoundRequests(CanonicalProjectRoot(filepath.Dir(taskFile)), taskFile)
	if err != nil {
		return nil
	}
	sort.SliceStable(reqs, func(i, j int) bool { return reqs[i].Seq < reqs[j].Seq })
	var ids []string
	for i := range reqs {
		if len(ids) == limit {
			break
		}
		if reqs[i].State == FoundStateUndecided {
			ids = append(ids, reqs[i].ID)
		}
	}
	return ids
}

// decisionUnits returns the one decision unit the cycle owes, or none when no
// request is undecided (spec/1.31.0.md §5).
func decisionUnits(target UnitTarget) []NextUnit {
	ids := UndecidedRequestIDs(target.TaskFile, DecisionBatchMax)
	if len(ids) == 0 {
		return nil
	}
	return []NextUnit{newNextUnit(UnitDecision, strings.Join(ids, decisionIDSep), target)}
}

// DecisionRequestIDs splits a decision unit's id into its request ids.
func DecisionRequestIDs(unitID string) []string {
	var ids []string
	for id := range strings.SplitSeq(unitID, decisionIDSep) {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// decisionAnswered is the decision kind's durable write: none of the unit's
// requests is still undecided. A unit naming no request has answered nothing.
func decisionAnswered(t UnitTarget) bool {
	ids := DecisionRequestIDs(t.ID)
	if len(ids) == 0 {
		return false
	}
	reqs, err := ReadFoundRequests(CanonicalProjectRoot(filepath.Dir(t.TaskFile)), t.TaskFile)
	if err != nil {
		return false
	}
	for i := range reqs {
		if reqs[i].State == FoundStateUndecided && slices.Contains(ids, reqs[i].ID) {
			return false
		}
	}
	return true
}
