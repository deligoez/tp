package model

import (
	"encoding/json"
	"fmt"
	"slices"
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

// ClaimedBy names the agent spawn a task is claimed for: the Agent tool call's
// tool_use_id, and the session that made the call when the caller knew it.
type ClaimedBy struct {
	Session string `json:"session,omitempty"`
	Spawn   string `json:"spawn,omitempty"`
	// Token is the claim token of the brief the spawn was made from; an
	// agent's close in its own worktree must name it.
	Token string `json:"token,omitempty"`
	// Retired holds the tokens takeovers cleared: a spawn from a replaced
	// brief would share its token with the agent it replaced.
	Retired []string `json:"retired,omitempty"`
}

// Released is what a claim leaves on its task when its agent is released
// (unclaim, reopen, a takeover's spawn): the tokens a spawn from a replaced
// brief must not reuse, nil when there are none.
func (c *ClaimedBy) Released() *ClaimedBy {
	if c == nil {
		return nil
	}
	retired := slices.Clone(c.Retired)
	if c.Token != "" {
		retired = append(retired, c.Token)
	}
	if len(retired) == 0 {
		return nil
	}
	return &ClaimedBy{Retired: retired}
}

// Origin is where a found task came from: its finder, either the task whose
// unit found the request or the audit round and row key that raised it, and
// the request's location (file:line).
type Origin struct {
	FinderTask string `json:"finder_task,omitempty"`
	AuditRound int    `json:"audit_round,omitempty"`
	AuditRow   string `json:"audit_row,omitempty"`
	Location   string `json:"location"`
	// QueueEntry names the queue entry the person settled into this task
	// (spec/1.31.0.md §6); such a task does not count against found_max.
	QueueEntry string `json:"queue_entry,omitempty"`
}

// FoundNote is a request attached to an open task in place of a found task of
// its own. It carries the request's fields, since the request store it came
// from is not committed.
type FoundNote struct {
	Request      string   `json:"request"`
	Location     string   `json:"location"`
	Reproduction string   `json:"reproduction"`
	Expected     string   `json:"expected,omitempty"`
	Observed     string   `json:"observed,omitempty"`
	Question     string   `json:"question,omitempty"`
	Alternatives []string `json:"alternatives,omitempty"`
}

// Task represents a single atomic work item.
type Task struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Status          string   `json:"status"`
	Tags            []string `json:"tags"`
	DependsOn       []string `json:"depends_on"`
	EstimateMinutes int      `json:"estimate_minutes"`
	Acceptance      string   `json:"acceptance"`
	SourceSections  []string `json:"source_sections"`
	SourceLines     string   `json:"source_lines,omitempty"`
	// Origin anchors a found task: the request that opened it, carried in
	// place of source_sections, never beside it (spec/1.31.0.md §3).
	Origin  *Origin  `json:"origin,omitempty"`
	Touches []string `json:"touches,omitempty"`
	// FoundNotes is managed by tp: the requests `tp found apply` attached to
	// this task because their location falls inside its touches
	// (spec/1.31.0.md §2).
	FoundNotes []FoundNote `json:"found_notes,omitempty"`
	// Solo is managed by tp: a task whose side-by-side unit failed to
	// integrate runs alone from then on (spec/backlog/parallel-implement.md).
	Solo bool `json:"solo,omitempty"`
	// Waiting says what a task is waiting on. On a wip task it is a parked
	// claim (a background job, a person), so tp resume can tell one from an
	// abandoned claim; on an open task it is a hold, a gate that is not a
	// task, which keeps the task out of the ready set (Held,
	// spec/backlog/filled-pool.md). A close, a claim, an unclaim or a reopen
	// clears it.
	Waiting string `json:"waiting,omitempty"`
	// QueueEntry is the open found-queue entry that holds this task as a
	// finder (spec/1.31.0.md §4), derived from the queue file when the task
	// file is read and never stored: settling the entry is what lifts it.
	QueueEntry string `json:"-"`
	// ClaimedBy is managed by tp: the spawn whose agent holds this task,
	// set by `tp claim --spawn` and cleared with started_at by an unclaim,
	// a reopen or a run's release (spec/backlog/agent-bound-claims.md).
	ClaimedBy         *ClaimedBy `json:"claimed_by,omitempty"`
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
	// TaskAuditOwed is managed by tp: the resolved task_audit_roles at the
	// moment the task closed, the code audit this task owes. Absent when no
	// role was owed; cleared by tp reopen.
	TaskAuditOwed []string `json:"task_audit_owed,omitempty"`
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

// SetTaskAuditOwed stamps the task with a copy of the roles its close owes a
// code audit, so a later change to the workflow cannot reach the record. An
// empty list clears the stamp, which then marshals with no key.
func (t *Task) SetTaskAuditOwed(roles []string) {
	if len(roles) == 0 {
		t.TaskAuditOwed = nil
		return
	}
	t.TaskAuditOwed = append([]string(nil), roles...)
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

// Held reports whether t is an open task held by a gate that is not a task:
// waiting set while open, or an open found-queue entry holding it as a
// finder (QueueEntry). A held task is never ready.
func (t *Task) Held() bool {
	return t.Status == StatusOpen && (t.Waiting != "" || t.QueueEntry != "")
}

// Startable reports whether t is open, not held, and every dependency is in
// done: the ready set every picker reads.
func (t *Task) Startable(done map[string]bool) bool {
	return t.Status == StatusOpen && !t.Held() && t.DepsDone(done)
}

// DepsDone reports whether every dependency of t is in done.
func (t *Task) DepsDone(done map[string]bool) bool {
	for _, dep := range t.DependsOn {
		if !done[dep] {
			return false
		}
	}
	return true
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
