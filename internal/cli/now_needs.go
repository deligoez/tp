package cli

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/deligoez/tp/internal/engine"
)

// The kinds of a needs_you entry.
const (
	needBlocker    = "blocker"
	needEscalation = "escalation"
	needDecision   = "decision"
	needCap        = "cap"
	needLand       = "land"
)

// escalationTexts words each operator decision a unit can escalate
// (engine.EscalationDecisions) as what the unit asks of you.
var escalationTexts = map[string]string{
	engine.EscalateSkipGate:            "A unit asks to skip the quality gate.",
	engine.EscalateRaiseReviewCap:      "A unit asks to raise the spec review's round limit.",
	engine.EscalateRaiseAuditCap:       "A unit asks to raise the audit's round limit.",
	engine.EscalateRaiseFoundCap:       "A unit asks to raise the limit on found tasks.",
	engine.EscalateImportForce:         "A unit asks to overwrite the task list.",
	engine.EscalateAuditConvergeOn:     "A unit asks to let the audit finish on blocking findings only.",
	engine.EscalateAcceptFinding:       "A unit asks to accept a finding without a change.",
	engine.EscalateDiscardEmission:     "A unit asks to discard a round nobody recorded.",
	engine.EscalateEpicDone:            "A unit asks to mark an epic as done.",
	engine.EscalateGateMet:             "A unit asks to mark a milestone gate as met.",
	engine.EscalateEpicBinding:         "A unit asks to bind the task file to an epic.",
	engine.EscalateGrantRound:          "A unit asks for one more epic review round.",
	engine.EscalateRewritePastBlocking: "A unit asks to start the spec over past an unresolved blocking finding.",
	engine.EscalateOther:               "A unit stopped the run and needs your decision.",
}

// escalationNeeds reads the escalation records of the run st when it stopped
// on one, in sequence order, each as a need naming the unit that asked. The
// record's evidence is the unit's own words and may quote a command, so the
// text is the decision's, never the evidence.
func escalationNeeds(root string, st *engine.RunState) []nowNeed {
	out := []nowNeed{}
	if st.StopReason == nil || *st.StopReason != engine.StopEscalation {
		return out
	}
	paths, err := filepath.Glob(engine.EscalationPath(engine.RunDir(root, st.RunID), "*"))
	if err != nil {
		return out
	}
	sort.Strings(paths)
	for _, path := range paths {
		e, readErr := engine.ReadEscalation(path)
		if readErr != nil {
			continue
		}
		out = append(out, escalationNeed(e))
	}
	return out
}

// escalationNeed words one escalation record: a named operator decision is
// of kind decision, everything else (other) of kind escalation.
func escalationNeed(e *engine.Escalation) nowNeed {
	kind := needDecision
	if e.Decision == engine.EscalateOther {
		kind = needEscalation
	}
	return nowNeed{Kind: kind, ID: e.UnitID, Text: escalationTexts[e.Decision]}
}

// blockerNeeds turns tp resume's blockers into needs_you entries. Each text
// is written here from the blocker's code and data, never copied from its
// message, which names the commands that clear it: the pane shows the text
// to a person (CLAUDE.md rule 14). An agent-clearable blocker waits on the
// agent, not on a person, so it is left out.
func blockerNeeds(blockers []engine.Blocker) []nowNeed {
	out := []nowNeed{}
	for i := range blockers {
		if blockers[i].Class == engine.ClassAgentClearable {
			continue
		}
		out = append(out, blockerNeed(&blockers[i]))
	}
	return out
}

// blockerNeed words one escalate blocker. A blocker whose data names an
// operator decision is of kind decision; a round cap, of kind cap.
func blockerNeed(b *engine.Blocker) nowNeed {
	kind := needBlocker
	if _, ok := b.Data["operator"]; ok {
		kind = needDecision
	}
	switch b.Code {
	case "no-ready-task":
		if _, held := b.Data["held"]; held {
			return nowNeed{Kind: kind, Text: "No task is ready: the open tasks wait on a hold or on unfinished dependencies."}
		}
		return nowNeed{Kind: kind, Text: "No task is ready: the open tasks wait on unfinished dependencies."}
	case "epic-blocked":
		epic, _ := b.Data["epic"].(string)
		if kind == needDecision {
			return nowNeed{Kind: kind, ID: epic, Text: fmt.Sprintf("No task can start: epic %s waits on a condition only you can mark as met.", epic)}
		}
		return nowNeed{Kind: kind, ID: epic, Text: fmt.Sprintf("No task can start: epic %s waits on other epics to finish.", epic)}
	case "review-carried-blocking":
		return nowNeed{Kind: kind, Text: "A blocking review finding from an earlier round is still unresolved."}
	case "review-budget-exhausted":
		return nowNeed{Kind: needCap, Text: "The spec review reached its round limit with findings still open."}
	case "audit-budget-exhausted":
		return nowNeed{Kind: needCap, Text: "The audit reached its round limit with findings still open."}
	case "spec-stale":
		if kind == needDecision {
			return nowNeed{Kind: kind, Text: "The spec changed after its last review, and the next review round needs your decision."}
		}
		return nowNeed{Kind: kind, Text: "The spec changed after its last review and needs a new review round."}
	}
	return nowNeed{Kind: kind, Text: fmt.Sprintf("The work is stopped by a blocker (%s).", b.Code)}
}
