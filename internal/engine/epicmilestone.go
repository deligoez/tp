package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicMilestoneExistsError reports a milestone insert whose name a track
// already holds (spec/1.4.0.md §7). It is refused by this check rather than
// through lint, so a file that already holds a duplicate name refuses too.
type EpicMilestoneExistsError struct {
	Name string
}

func (e *EpicMilestoneExistsError) Error() string {
	return fmt.Sprintf("milestone %q is already in a track", e.Name)
}

// EpicMilestoneNotFoundError reports a milestone no track holds, named by an
// insert's anchor, a gate field set or a --met. Known lists the milestones
// the tracks do hold, in track order, so the refusal answers what to name.
type EpicMilestoneNotFoundError struct {
	Name  string
	Known []string
}

// NewEpicMilestoneNotFoundError is the refusal of name over g's tracks.
func NewEpicMilestoneNotFoundError(g *model.EpicGraph, name string) *EpicMilestoneNotFoundError {
	return &EpicMilestoneNotFoundError{Name: name, Known: milestoneUnion(g, g)}
}

func (e *EpicMilestoneNotFoundError) Error() string {
	if len(e.Known) == 0 {
		return fmt.Sprintf("milestone %q is in no track; the tracks hold none", e.Name)
	}
	return fmt.Sprintf("milestone %q is in no track; the tracks hold %s", e.Name, strings.Join(e.Known, ", "))
}

// EpicMilestonePlace is where tp epic milestone add puts a name: right after
// or right before an existing milestone in its track, or in a new track of
// its own. Exactly one is set.
type EpicMilestonePlace struct {
	After, Before string
	NewTrack      bool
}

// InsertMilestone returns the mutation that inserts name at place.
func InsertMilestone(name string, place EpicMilestonePlace) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		if milestoneDeclared(g, name) {
			return &EpicMilestoneExistsError{Name: name}
		}
		if place.NewTrack {
			g.Milestones = append(g.Milestones, []string{name})
			return nil
		}
		anchor, offset := place.After, 1
		if anchor == "" {
			anchor, offset = place.Before, 0
		}
		for t := range g.Milestones {
			if i := slices.Index(g.Milestones[t], anchor); i >= 0 {
				g.Milestones[t] = slices.Insert(g.Milestones[t], i+offset, name)
				return nil
			}
		}
		return NewEpicMilestoneNotFoundError(g, anchor)
	}
}
