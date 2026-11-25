package cli

import (
	"fmt"

	"github.com/deligoez/tp/internal/engine"
)

// The kinds of a needs_you entry.
const (
	needBlocker  = "blocker"
	needDecision = "decision"
	needCap      = "cap"
	needLand     = "land"
)

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
