package model

import "time"

// EpicGraph models the committed .tp/epics.json file: the epic graph a project
// plans against. The file is primary — the one tp lints, reviews and writes —
// and any prose table is an export of it.
//
// Field order here is the key order a write emits, so a file in that form
// round-trips byte-identically. Optional collections are omitted when empty,
// so a file that leaves them out is not rewritten with an empty value.
type EpicGraph struct {
	Version int `json:"version"`
	// Milestones is a list of tracks, each an ordered list of milestone names.
	Milestones [][]string `json:"milestones"`
	// OpensAfter maps a milestone to the milestones it opens after
	// (spec/1.6.0.md §1); a milestone it does not name opens after the
	// previous milestone of its track.
	OpensAfter map[string][]string `json:"opens_after,omitempty"`
	// Gates maps a milestone name to the epics its gate covers.
	Gates map[string]EpicGate `json:"gates,omitempty"`
	// TagChecks maps a tag to the epic every carrier of the tag must depend on.
	TagChecks map[string]EpicTagCheck `json:"tag_checks,omitempty"`
	Epics     []Epic                  `json:"epics"`
}

// EpicGate is one milestone's gate: the member epics it requires, the ones it
// may defer, the exit threshold in the operator's words, and the record of
// that threshold being met.
type EpicGate struct {
	Members    []string `json:"members"`
	Deferrable []string `json:"deferrable,omitempty"`
	Threshold  string   `json:"threshold,omitempty"`
	Met        *GateMet `json:"met,omitempty"`
}

// GateMet records a gate's threshold as met: when, on what evidence, and the
// threshold it was judged against, copied so a later edit shows as a change.
type GateMet struct {
	At        time.Time `json:"at"`
	Evidence  string    `json:"evidence"`
	Threshold string    `json:"threshold"`
}

// EpicTagCheck names the epic that provides a tag's check.
type EpicTagCheck struct {
	ProvidedBy string `json:"provided_by"`
}

// Epic is one node of the graph. ID is opaque: a project's own naming, spaces
// and non-ASCII included, stays as it is. Title, Acceptance and every Why are
// content and keep their language.
type Epic struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	Milestone  string    `json:"milestone"`
	Tags       []string  `json:"tags,omitempty"`
	DependsOn  []EpicDep `json:"depends_on,omitempty"`
	Acceptance string    `json:"acceptance,omitempty"`
	// Root says the epic has no dependency on purpose; written only when true.
	Root bool `json:"root,omitempty"`
	// Condition marks a precondition the operator decides rather than work;
	// written only when true.
	Condition bool `json:"condition,omitempty"`
	// EstimateMinutes is the epic's own estimate; 0 is unset and not written.
	EstimateMinutes int `json:"estimate_minutes,omitempty"`
	// Claimed and Done are written by their commands and by tp epic import.
	Claimed *EpicClaim `json:"claimed,omitempty"`
	Done    *EpicDone  `json:"done,omitempty"`
}

// EpicClaim reserves an epic for one holder.
type EpicClaim struct {
	At time.Time `json:"at"`
	By string    `json:"by"`
}

// EpicDone records an epic as done on the operator's evidence.
type EpicDone struct {
	At       time.Time `json:"at"`
	Evidence string    `json:"evidence"`
}

// EpicDep is one dependency edge; Why is required by the schema.
type EpicDep struct {
	ID  string `json:"id"`
	Why string `json:"why"`
}
