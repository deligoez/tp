package model

import (
	"encoding/json"
	"time"
)

// Task status constants.
const (
	StatusOpen = "open"
	StatusWIP  = "wip"
	StatusDone = "done"
)

// Task represents a single atomic work item.
type Task struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description,omitempty"`
	Status          string     `json:"status"`
	Tags            []string   `json:"tags,omitempty"`
	DependsOn       []string   `json:"depends_on"`
	EstimateMinutes int        `json:"estimate_minutes"`
	Acceptance      string     `json:"acceptance"`
	SourceSections  []string   `json:"source_sections"`
	SourceLines     string     `json:"source_lines,omitempty"`
	StartedAt       *time.Time `json:"started_at"`
	ClosedAt        *time.Time `json:"closed_at"`
	ClosedReason    *string    `json:"closed_reason"`
	GatePassedAt    *time.Time `json:"gate_passed_at"`
	CommitSHA       *string    `json:"commit_sha"`
}

// UnmarshalJSON supports aliases: "deps" → "depends_on", "estimation_minutes" → "estimate_minutes".
func (t *Task) UnmarshalJSON(data []byte) error {
	type Alias Task
	aux := &struct {
		*Alias
		Deps              []string `json:"deps"`
		EstimationMinutes int      `json:"estimation_minutes"`
	}{Alias: (*Alias)(t)}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	// "deps" fills in when "depends_on" is absent
	if t.DependsOn == nil && aux.Deps != nil {
		t.DependsOn = aux.Deps
	}

	// "estimation_minutes" fills in when "estimate_minutes" is absent (0 is not a valid estimate)
	if t.EstimateMinutes == 0 && aux.EstimationMinutes != 0 {
		t.EstimateMinutes = aux.EstimationMinutes
	}
	return nil
}

// ValidTransition returns true if the status transition is allowed.
// Valid transitions: open→wip, wip→done, done→open.
func ValidTransition(from, to string) bool {
	switch {
	case from == StatusOpen && to == StatusWIP:
		return true
	case from == StatusWIP && to == StatusDone:
		return true
	case from == StatusDone && to == StatusOpen:
		return true
	default:
		return false
	}
}

// ValidStatus returns true if the status string is a known status.
func ValidStatus(s string) bool {
	return s == StatusOpen || s == StatusWIP || s == StatusDone
}
