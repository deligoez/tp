package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// The labels a found task's description carries its request's Fix fields
// under: newFoundTask writes them, and a found task's brief reads them back.
const (
	foundLabelReproduction = "Reproduction: "
	foundLabelExpected     = "\nExpected: "
	foundLabelObserved     = "\nObserved: "
)

// BriefFoundRequest is the request a found task answers, carried in its brief
// in place of a spec excerpt (spec/1.31.0.md §3): the request's location,
// reproduction, expected and observed behaviour, and the first step. A found
// task whose description is not the one tp found apply writes shows that
// description whole instead of the three parsed fields.
type BriefFoundRequest struct {
	Location     string `json:"location"`
	Reproduction string `json:"reproduction,omitempty"`
	Expected     string `json:"expected,omitempty"`
	Observed     string `json:"observed,omitempty"`
	Description  string `json:"description,omitempty"`
	FirstStep    string `json:"first_step"`
}

// foundTaskDescription is the description tp found apply gives a found task:
// the request's three Fix fields after the location, one label each.
func foundTaskDescription(reproduction, expected, observed string) string {
	return foundLabelReproduction + reproduction + foundLabelExpected + expected + foundLabelObserved + observed
}

// parseFoundTaskDescription reads back what foundTaskDescription wrote; ok is
// false for any other description.
func parseFoundTaskDescription(desc string) (reproduction, expected, observed string, ok bool) {
	rest, found := strings.CutPrefix(desc, foundLabelReproduction)
	if !found {
		return "", "", "", false
	}
	reproduction, rest, found = strings.Cut(rest, foundLabelExpected)
	if !found {
		return "", "", "", false
	}
	expected, observed, found = strings.Cut(rest, foundLabelObserved)
	if !found {
		return "", "", "", false
	}
	return reproduction, expected, observed, true
}

// foundFirstStep is a found task's first step: the reproduction, and the
// handback when it does not fail for the stated reason (spec/1.31.0.md §3).
func foundFirstStep(id string) string {
	return fmt.Sprintf("Run the reproduction first. When it does not fail for the stated reason, "+
		"run `tp found handback %s \"<why>\"` and stop: the task closes as done, marked handed back, "+
		"and its request goes to the queue.", id)
}

// foundBriefRequest is the request block of a found task's brief.
func foundBriefRequest(task *model.Task) *BriefFoundRequest {
	r := &BriefFoundRequest{Location: task.Origin.Location, FirstStep: foundFirstStep(task.ID)}
	if rep, exp, obs, ok := parseFoundTaskDescription(task.Description); ok {
		r.Reproduction, r.Expected, r.Observed = rep, exp, obs
	} else {
		r.Description = task.Description
	}
	return r
}

// renderFoundRequestText renders a found task's request block for the text
// brief.
func renderFoundRequestText(r *BriefFoundRequest) string {
	var s strings.Builder
	s.WriteString("\nrequest:\n")
	fmt.Fprintf(&s, "location: %s\n", r.Location)
	if r.Description != "" {
		fmt.Fprintf(&s, "description: %s\n", r.Description)
	} else {
		fmt.Fprintf(&s, "reproduction: %s\nexpected: %s\nobserved: %s\n", r.Reproduction, r.Expected, r.Observed)
	}
	fmt.Fprintf(&s, "\nfirst step: %s\n", r.FirstStep)
	return s.String()
}
