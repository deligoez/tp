package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Task status constants.
const (
	StatusOpen = "open"
	StatusWIP  = "wip"
	StatusDone = "done"
)

// DurationSource values record how started_at was set (§11.2).
const (
	DurationSourceClaimed  = "claimed"
	DurationSourceImplicit = "implicit"
)

// Task represents a single atomic work item.
type Task struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description,omitempty"`
	Status            string     `json:"status"`
	Tags              []string   `json:"tags"`
	DependsOn         []string   `json:"depends_on"`
	EstimateMinutes   int        `json:"estimate_minutes"`
	Acceptance        string     `json:"acceptance"`
	SourceSections    []string   `json:"source_sections"`
	SourceLines       string     `json:"source_lines,omitempty"`
	StartedAt         *time.Time `json:"started_at"`
	DurationSource    string     `json:"duration_source,omitempty"`
	ClosedAt          *time.Time `json:"closed_at"`
	ClosedReason      *string    `json:"closed_reason"`
	GatePassedAt      *time.Time `json:"gate_passed_at"`
	CommitSHA         *string    `json:"commit_sha"`
	CommitSHAs        []string   `json:"commit_shas,omitempty"`
	CommitFiles       []string   `json:"commit_files,omitempty"`
	CommitFilesTotal  int        `json:"commit_files_total,omitempty"`
	GateSkippedReason *string    `json:"gate_skipped_reason,omitempty"`
}

// UnmarshalJSON supports aliases and flexible types:
// - "deps" → "depends_on"
// - "estimation_minutes" → "estimate_minutes"
// - acceptance: string or []string (array joined with "\n- ")
func (t *Task) UnmarshalJSON(data []byte) error {
	type Alias Task
	aux := &struct {
		Deps              []string        `json:"deps"`
		EstimationMinutes int             `json:"estimation_minutes"`
		AcceptanceRaw     json.RawMessage `json:"acceptance"`
		*Alias
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

	// acceptance: try string first, then []string
	if len(aux.AcceptanceRaw) > 0 {
		var strVal string
		if err := json.Unmarshal(aux.AcceptanceRaw, &strVal); err == nil {
			t.Acceptance = strVal
		} else {
			var arrVal []string
			if err := json.Unmarshal(aux.AcceptanceRaw, &arrVal); err == nil {
				if len(arrVal) == 0 {
					return fmt.Errorf("acceptance must not be empty")
				}
				for i, item := range arrVal {
					if strings.TrimSpace(item) == "" {
						return fmt.Errorf("acceptance array element %d is empty", i)
					}
				}
				t.Acceptance = "- " + strings.Join(arrVal, "\n- ")
			} else {
				return fmt.Errorf("acceptance must be a string or array of strings")
			}
		}
	}

	// commit_shas is canonical; keep the commit_sha mirror on its first element
	if len(t.CommitSHAs) > 0 {
		primary := t.CommitSHAs[0]
		t.CommitSHA = &primary
	}

	return nil
}

// SetCommitSHAs records the ordered commits that implement the task and keeps
// the commit_sha mirror pointed at the primary (first) commit for
// backward-compatible readers. An empty slice clears both fields.
func (t *Task) SetCommitSHAs(shas []string) {
	if len(shas) == 0 {
		t.CommitSHAs = nil
		t.CommitSHA = nil
		return
	}
	t.CommitSHAs = shas
	primary := shas[0]
	t.CommitSHA = &primary
}

const maxCommitFiles = 50

// SetCommitFiles records the deduplicated, lexically (byte) sorted set of
// repo-root-relative paths a task's commits touched, capped at 50 entries.
// When the set exceeds the cap the first 50 are stored and the full count is
// recorded in CommitFilesTotal; otherwise it is zero. An empty set clears both.
func (t *Task) SetCommitFiles(files []string) {
	seen := make(map[string]bool, len(files))
	unique := make([]string, 0, len(files))
	for _, f := range files {
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		unique = append(unique, f)
	}
	if len(unique) == 0 {
		t.CommitFiles = nil
		t.CommitFilesTotal = 0
		return
	}
	sort.Strings(unique)
	if total := len(unique); total > maxCommitFiles {
		t.CommitFiles = append([]string(nil), unique[:maxCommitFiles]...)
		t.CommitFilesTotal = total
		return
	}
	t.CommitFiles = append([]string(nil), unique...)
	t.CommitFilesTotal = 0
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
