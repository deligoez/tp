package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicSliceKey names one tp epic review slice (spec/1.3.0.md §5,
// spec/1.4.0.md §6): exactly one of a milestone, an epic id, or a set of epic
// ids. Name, set only with Epics, keys a set's state by name instead of ids.
type EpicSliceKey struct {
	Milestone string
	Epic      string
	Epics     []string
	Name      string
}

// EpicReviewUnit is one unit of a slice: an epic, keyed "epic:<id>", a
// milestone's gate, keyed "gate:<m>", or the gate taken whole, keyed
// "scope:<m>" (spec/backlog/epic-scope.md, Part B). Hash covers the unit's
// own block and, for an epic, the blocks of its direct dependencies; a scope
// unit's hash is its scope fingerprint. Milestone is the milestone the unit
// belongs to: the epic's, or the gate's; Purpose is that milestone's gate
// purpose (GatePurpose), which no hash covers: a changed purpose re-asks the
// roles that take it (EpicCarry.PurposeStale) without changing the unit.
type EpicReviewUnit struct {
	Key       string
	Block     string
	Hash      string
	Milestone string
	Purpose   string
}

// The unit key prefixes, one per kind a graph role's units may list.
const (
	EpicUnitPrefix  = "epic:"
	GateUnitPrefix  = "gate:"
	ScopeUnitPrefix = "scope:"
)

// EpicUnitKind returns the role unit kind (model.RoleUnitEpics, Gates or
// Scope) a unit key names; "" for a key of no kind.
func EpicUnitKind(key string) string {
	switch {
	case strings.HasPrefix(key, EpicUnitPrefix):
		return model.RoleUnitEpics
	case strings.HasPrefix(key, GateUnitPrefix):
		return model.RoleUnitGates
	case strings.HasPrefix(key, ScopeUnitPrefix):
		return model.RoleUnitScope
	}
	return ""
}

// EpicContextBlock is the block of an epic shown below the units as context.
// Dependency is true when an asked epic depends on it; false, it only depends
// on an asked epic, and Brief holds what a neighbours-brief role is shown of
// it instead of Block (spec/1.7.0.md §4).
type EpicContextBlock struct {
	ID         string
	Block      string
	Dependency bool
	Brief      string
}

// EpicSlice is a slice's units in file order, the gate units last, and the
// context: every neighbour of a unit that is not itself a unit, each once, in
// file order (D5).
type EpicSlice struct {
	Units   []EpicReviewUnit
	Context []EpicContextBlock
}

// EpicReviewSlice derives the units and context of key's slice over g. An
// epic id or milestone the graph does not hold is an error.
func EpicReviewSlice(g *model.EpicGraph, key EpicSliceKey) (EpicSlice, error) {
	byID := map[string]*model.Epic{}
	for i := range g.Epics {
		if _, dup := byID[g.Epics[i].ID]; !dup {
			byID[g.Epics[i].ID] = &g.Epics[i]
		}
	}
	named, err := sliceEpicIDs(g, key, byID)
	if err != nil {
		return EpicSlice{}, err
	}
	var s EpicSlice
	asked := map[string]bool{}
	for _, id := range named {
		asked[id] = true
		s.Units = append(s.Units, epicUnit(byID[id], byID))
	}
	after := EffectiveOpensAfter(g)
	gates := sliceGates(g, key, asked)
	for _, m := range gates {
		block := epicGateBlock(g, m, after)
		s.Units = append(s.Units, EpicReviewUnit{Key: GateUnitPrefix + m, Block: block, Hash: epicHash(block), Milestone: m})
	}
	for _, m := range gates {
		if u, ok := epicScopeUnit(g, m, byID); ok {
			s.Units = append(s.Units, u)
		}
	}
	for i := range s.Units {
		s.Units[i].Purpose = GatePurpose(g, s.Units[i].Milestone)
	}
	s.Context = epicSliceContext(g, asked)
	return s, nil
}

// epicScopeUnit builds milestone's scope:<m> unit, which exists only while
// its gate has a threshold or a purpose: its block holds what the scope
// fingerprint hashes, all of it blind to status, so progress never reads as
// a change, and its hash is that fingerprint.
func epicScopeUnit(g *model.EpicGraph, milestone string, byID map[string]*model.Epic) (EpicReviewUnit, bool) {
	gate := g.Gates[milestone]
	purpose := GatePurpose(g, milestone)
	if gate.Threshold == "" && purpose == "" {
		return EpicReviewUnit{}, false
	}
	members, deferrable := GateMembers(g, milestone)
	var b strings.Builder
	b.WriteString("scope: " + milestone + "\nmembers: " + strings.Join(slices.Sorted(slices.Values(members)), ", ") + "\n")
	if len(deferrable) > 0 {
		b.WriteString("deferrable: " + strings.Join(slices.Sorted(slices.Values(deferrable)), ", ") + "\n")
	}
	if gate.Threshold != "" {
		b.WriteString("threshold: " + gate.Threshold + "\n")
	}
	if purpose != "" {
		b.WriteString("purpose: " + purpose + "\n")
	}
	b.WriteString("closure: " + strings.Join(blindClosure(byID, members), ", ") + "\n")
	return EpicReviewUnit{Key: ScopeUnitPrefix + milestone, Block: b.String(), Hash: gateScopeFingerprint(g, milestone, byID), Milestone: milestone}, true
}

// sliceGates names the gate units a slice asks, sorted: every milestone has a
// gate, implicit when gates names none, so a milestone slice asks its own,
// and a set slice the gate of every milestone, tracked or named by gates,
// whose every epic the set lists.
func sliceGates(g *model.EpicGraph, key EpicSliceKey, asked map[string]bool) []string {
	if key.Milestone != "" {
		return []string{key.Milestone}
	}
	if len(key.Epics) == 0 {
		return nil
	}
	names := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			names[m] = true
		}
	}
	for m := range g.Gates {
		names[m] = true
	}
	var out []string
	for _, m := range sortedMapKeys(names) {
		listed, held := true, false
		for i := range g.Epics {
			e := &g.Epics[i]
			if e.Milestone == m {
				held = true
				listed = listed && asked[e.ID]
			}
		}
		if held && listed {
			out = append(out, m)
		}
	}
	return out
}

// sliceEpicIDs returns the ids the slice names, in file order.
func sliceEpicIDs(g *model.EpicGraph, key EpicSliceKey, byID map[string]*model.Epic) ([]string, error) {
	if len(key.Epics) > 0 {
		want := map[string]bool{}
		for _, id := range key.Epics {
			if _, ok := byID[id]; !ok {
				return nil, fmt.Errorf("epic %q is not in the graph", id)
			}
			want[id] = true
		}
		var ids []string
		for i := range g.Epics {
			e := &g.Epics[i]
			if want[e.ID] {
				delete(want, e.ID)
				ids = append(ids, e.ID)
			}
		}
		return ids, nil
	}
	if key.Epic != "" {
		if _, ok := byID[key.Epic]; !ok {
			return nil, fmt.Errorf("epic %q is not in the graph", key.Epic)
		}
		return []string{key.Epic}, nil
	}
	if !milestoneDeclared(g, key.Milestone) {
		return nil, NewEpicMilestoneNotFoundError(g, key.Milestone)
	}
	var ids []string
	seen := map[string]bool{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.Milestone == key.Milestone && !seen[e.ID] {
			seen[e.ID] = true
			ids = append(ids, e.ID)
		}
	}
	return ids, nil
}

func milestoneDeclared(g *model.EpicGraph, name string) bool {
	for _, track := range g.Milestones {
		if slices.Contains(track, name) {
			return true
		}
	}
	return false
}

// epicSliceContext returns the block of every epic one edge away, in either
// direction, from an asked epic, unless it is asked itself. Dependency marks a
// block that some asked epic depends on, as against one that only depends on
// an asked epic. A group's every member is one edge away from its epic
// (spec/1.7.0.md §7).
func epicSliceContext(g *model.EpicGraph, asked map[string]bool) []EpicContextBlock {
	near := map[string]bool{}
	dependency := map[string]bool{}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			for _, id := range d.Names() {
				if asked[e.ID] {
					near[id] = true
					dependency[id] = true
				}
				if asked[id] {
					near[e.ID] = true
				}
			}
		}
	}
	var ctx []EpicContextBlock
	seen := map[string]bool{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if near[e.ID] && !asked[e.ID] && !seen[e.ID] {
			seen[e.ID] = true
			c := EpicContextBlock{ID: e.ID, Block: epicBlock(e), Dependency: dependency[e.ID]}
			if !c.Dependency {
				c.Brief = epicDependentBrief(e, asked)
			}
			ctx = append(ctx, c)
		}
	}
	return ctx
}

// epicDependentBrief renders a dependent of the asked epics by its id, title
// and milestone and its edges into them, each with its why, a group naming
// one of them whole (spec/1.7.0.md §7): a placement
// reader's view of what needs the asked units, without the acceptance text
// that makes a full block the bulk of a large slice's prompt.
func epicDependentBrief(e *model.Epic, asked map[string]bool) string {
	var b strings.Builder
	b.WriteString("epic: " + e.ID + "\ntitle: " + e.Title + "\nmilestone: " + e.Milestone + "\n")
	b.WriteString("depends_on (its edges into the asked units only):\n")
	for _, d := range e.DependsOn {
		if slices.ContainsFunc(d.Names(), func(id string) bool { return asked[id] }) {
			b.WriteString("- " + epicDepName(d) + ": " + d.Why + "\n")
		}
	}
	return b.String()
}

// epicUnit builds an epic's unit, hashing its block and the block of each
// epic its entries name, a group's members included (spec/1.7.0.md §7); a
// dependency the graph does not hold adds nothing to the hash.
func epicUnit(e *model.Epic, byID map[string]*model.Epic) EpicReviewUnit {
	block := epicBlock(e)
	var hashed strings.Builder
	hashed.WriteString(block)
	for _, d := range e.DependsOn {
		for _, id := range d.Names() {
			if dep, ok := byID[id]; ok {
				hashed.WriteString("\x00" + epicBlock(dep))
			}
		}
	}
	return EpicReviewUnit{Key: EpicUnitPrefix + e.ID, Block: block, Hash: epicHash(hashed.String()), Milestone: e.Milestone}
}

// epicBlock renders an epic's id, title, milestone, condition when true, tags,
// acceptance and each outgoing edge with its why, a group as one line.
func epicBlock(e *model.Epic) string {
	var b strings.Builder
	b.WriteString("epic: " + e.ID + "\n")
	b.WriteString("title: " + e.Title + "\n")
	b.WriteString("milestone: " + e.Milestone + "\n")
	if e.Condition {
		b.WriteString("condition: true\n")
	}
	if len(e.Tags) > 0 {
		b.WriteString("tags: " + strings.Join(e.Tags, ", ") + "\n")
	}
	if e.Acceptance != "" {
		b.WriteString("acceptance: " + e.Acceptance + "\n")
	}
	if len(e.DependsOn) > 0 {
		b.WriteString("depends_on:\n")
		for _, d := range e.DependsOn {
			b.WriteString("- " + epicDepName(d) + ": " + d.Why + "\n")
		}
	}
	return b.String()
}

// epicDepName names a depends_on entry in a block or a table: a plain edge
// by its id, a group as one of {A, B} (spec/1.7.0.md §7).
func epicDepName(d model.EpicDep) string {
	if d.IsGroup() {
		return epicDepLabel(d)
	}
	return d.ID
}

// epicGateBlock renders a milestone's gate, implicit when gates names none:
// its effective members (GateMembers), deferrables and threshold, then the
// milestone's effective opens_after (spec/1.6.0.md §1), so editing what the
// milestone opens after, its own entry or its track's order, re-asks it.
func epicGateBlock(g *model.EpicGraph, milestone string, after map[string][]string) string {
	gate := g.Gates[milestone]
	members, _ := GateMembers(g, milestone)
	block := "gate: " + milestone + "\nmembers: " + strings.Join(members, ", ") + "\n"
	if len(gate.Deferrable) > 0 {
		block += "deferrable: " + strings.Join(gate.Deferrable, ", ") + "\n"
	}
	if gate.Threshold != "" {
		block += "threshold: " + gate.Threshold + "\n"
	}
	list := "[]"
	if len(after[milestone]) > 0 {
		list = strings.Join(after[milestone], ", ")
	}
	return block + "opens_after: " + list + "\n"
}

func epicHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
