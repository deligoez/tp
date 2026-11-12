package model

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"
)

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
	// ForbiddenEdges lists the dependencies the project forbids
	// (spec/1.8.0.md §10).
	ForbiddenEdges []EpicForbiddenEdge `json:"forbidden_edges,omitempty"`
	Epics          []Epic              `json:"epics"`
}

// EpicForbiddenEdge forbids a direct edge, or an any-of member, from an epic
// From selects to one To selects, for the reason Why.
type EpicForbiddenEdge struct {
	From EpicSelector `json:"from"`
	To   EpicSelector `json:"to"`
	Why  string       `json:"why"`
}

// EpicSelector selects epics by exactly one of an epic id, a milestone, a
// tag, or the track holding a milestone (tracks carry no name of their own).
type EpicSelector struct {
	Epic      string `json:"epic,omitempty"`
	Milestone string `json:"milestone,omitempty"`
	Tag       string `json:"tag,omitempty"`
	TrackOf   string `json:"track_of,omitempty"`
}

// UnmarshalJSON decodes a selector holding exactly one of its four keys, a
// string; no key, two keys or any other key is a decode error, so every
// reader refuses it as it refuses a value of the wrong type.
func (s *EpicSelector) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("a selector is an object: %w", err)
	}
	if len(raw) != 1 {
		return fmt.Errorf("a selector holds exactly one of epic, milestone, tag, track_of; this one holds %d keys", len(raw))
	}
	fields := map[string]*string{"epic": &s.Epic, "milestone": &s.Milestone, "tag": &s.Tag, "track_of": &s.TrackOf}
	for key, value := range raw {
		field, ok := fields[key]
		if !ok {
			return fmt.Errorf("selector key %q is not one of epic, milestone, tag, track_of", key)
		}
		if err := json.Unmarshal(value, field); err != nil {
			return fmt.Errorf("selector %s: %w", key, err)
		}
	}
	return nil
}

// EpicGate is one milestone's gate: the member epics it requires, the ones it
// may defer, the exit threshold in the operator's words, the milestone's
// purpose, and the record of that threshold being met. A purpose that is
// empty or only whitespace counts as unset everywhere (spec/backlog/
// epic-scope.md, Part B).
type EpicGate struct {
	Members    []string `json:"members"`
	Deferrable []string `json:"deferrable,omitempty"`
	Threshold  string   `json:"threshold,omitempty"`
	Purpose    string   `json:"purpose,omitempty"`
	Met        *GateMet `json:"met,omitempty"`
}

// GateMet records a gate's threshold as met: when, on what evidence, and the
// threshold it was judged against, copied so a later edit shows as a change.
type GateMet struct {
	At        time.Time `json:"at"`
	Evidence  string    `json:"evidence"`
	Threshold string    `json:"threshold"`
}

// EpicTagCheck names the epic that provides a tag's check. Direct asks every
// carrier for a plain direct edge to the provider rather than any path to it
// (spec/1.7.0.md §3); written only when true.
type EpicTagCheck struct {
	ProvidedBy string `json:"provided_by"`
	Direct     bool   `json:"direct,omitempty"`
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
	// Description is the epic's optional markdown text: purpose, scope,
	// non-goals, decisions and open questions (spec/backlog/
	// epic-description.md). tp never parses it; empty or whitespace-only is
	// unset and is not written.
	Description string `json:"description,omitempty"`
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

// EpicDep is one depends_on entry: a plain edge to the epic ID, or an any-of
// group whose AnyOf members satisfy it when one is done (spec/1.7.0.md §7).
// Exactly one of the two is set; Why is required by the schema either way.
type EpicDep struct {
	ID    string   `json:"id,omitempty"`
	AnyOf []string `json:"any_of,omitempty"`
	Why   string   `json:"why"`
}

// IsGroup reports whether d is an any-of group: it carries an any_of list,
// an empty one included, whatever its id (any-of-shape judges the shape).
func (d EpicDep) IsGroup() bool {
	return d.AnyOf != nil
}

// Members returns a group's distinct members in stored order; nil for a
// plain edge.
func (d EpicDep) Members() []string {
	if !d.IsGroup() {
		return nil
	}
	out := make([]string, 0, len(d.AnyOf))
	for _, m := range d.AnyOf {
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}

// MemberSet is a group's identity: its distinct members, sorted. Two groups
// with one member set are one group; nil for a plain edge.
func (d EpicDep) MemberSet() []string {
	set := d.Members()
	slices.Sort(set)
	return set
}

// SameGroup reports whether d and o are groups with one member set.
func (d EpicDep) SameGroup(o EpicDep) bool {
	return d.IsGroup() && o.IsGroup() && slices.Equal(d.MemberSet(), o.MemberSet())
}

// Names returns every epic id d names, distinct: a plain edge's id, a
// group's members, and both for an entry that wrongly carries both.
func (d EpicDep) Names() []string {
	var out []string
	if d.ID != "" {
		out = append(out, d.ID)
	}
	for _, m := range d.Members() {
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	return out
}
