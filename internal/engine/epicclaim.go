package engine

import (
	"fmt"
	"time"

	"github.com/deligoez/tp/internal/model"
)

// EpicClaimRefusedError reports a claim refused on an epic (spec/1.5.0.md
// §7); nothing was written.
type EpicClaimRefusedError struct {
	ID     string
	Reason string
}

func (e *EpicClaimRefusedError) Error() string {
	return fmt.Sprintf("epic %q cannot be claimed: %s", e.ID, e.Reason)
}

// ClaimEpic returns the mutate that writes id's claim for by at at
// (spec/1.5.0.md §7). A claim reserves, so a blocked epic or one in a closed
// milestone is claimed. It returns an *EpicNotFoundError for an id the graph
// does not hold, and an *EpicClaimRefusedError when a claim exists, on a
// condition, on a done epic and on an epic a task file found from start
// binds.
func ClaimEpic(id, by, start string, at time.Time) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		e := &g.Epics[i]
		if reason := epicClaimRefusal(e, EpicBoundFiles(start)[id]); reason != "" {
			return &EpicClaimRefusedError{ID: id, Reason: reason}
		}
		e.Claimed = &model.EpicClaim{At: at.UTC(), By: by}
		return nil
	}
}

// epicClaimRefusal names why e cannot be claimed, "" when it can. files are
// the task files bound to e; an epic with none is done by its done record.
func epicClaimRefusal(e *model.Epic, files []string) string {
	switch {
	case e.Claimed != nil:
		return "it is claimed by " + e.Claimed.By + " (tp epic claim " + e.ID + " --release frees it)"
	case e.Condition:
		return "it is a condition, which the operator decides rather than works"
	case len(files) > 0:
		return files[0] + " is bound to it, so its status is derived"
	case e.Done != nil:
		return "it is done"
	}
	return ""
}

// ReleaseEpicClaim returns the mutate that removes id's claim, on a bound
// epic too (spec/1.5.0-corrections.md C6), or an *EpicNotFoundError for an
// id the graph does not hold. An epic with no claim is left as it is.
func ReleaseEpicClaim(id string) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		g.Epics[i].Claimed = nil
		return nil
	}
}
