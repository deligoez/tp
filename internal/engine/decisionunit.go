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
// task file's cycle, in arrival order. A missing store holds none; a store that
// exists but cannot be read or parsed is ReadFoundRequests' error, never an
// empty store, so a corrupt one cannot pass for a cycle with nothing to decide.
func UndecidedRequestIDs(taskFile string, limit int) ([]string, error) {
	reqs, err := ReadFoundRequests(CanonicalProjectRoot(filepath.Dir(taskFile)), taskFile)
	if err != nil {
		return nil, err
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
	return ids, nil
}

// decisionUnits returns the one decision unit the cycle owes, or none when no
// request is undecided (spec/1.31.0.md §5). A store it cannot read is the
// error: the caller hands out no unit rather than the ones that would follow
// (tp resume has already reported it, through resumeBlockers).
func decisionUnits(target UnitTarget) ([]NextUnit, error) {
	ids, err := UndecidedRequestIDs(target.TaskFile, DecisionBatchMax)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	return []NextUnit{newNextUnit(UnitDecision, strings.Join(ids, decisionIDSep), target)}, nil
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
