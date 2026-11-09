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

// EpicReviewUnit is one unit of a slice: an epic, keyed "epic:<id>", or a
// milestone's gate, keyed "gate:<m>". Hash covers the unit's own block and,
// for an epic, the blocks of its direct dependencies.
type EpicReviewUnit struct {
	Key   string
	Block string
	Hash  string
}

// EpicContextBlock is the block of an epic shown below the units as context.
// Dependency is true when an asked epic depends on it; false, it only depends
// on an asked epic.
type EpicContextBlock struct {
	ID         string
	Block      string
	Dependency bool
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
	for _, m := range sliceGates(g, key, asked) {
		block := epicGateBlock(m, g.Gates[m])
		s.Units = append(s.Units, EpicReviewUnit{Key: "gate:" + m, Block: block, Hash: epicHash(block)})
	}
	s.Context = epicSliceContext(g, asked)
	return s, nil
}

// sliceGates names the gate units a slice asks, sorted: a milestone slice's
// own gate, and for a set slice the gate of every milestone whose every epic
// the set lists.
func sliceGates(g *model.EpicGraph, key EpicSliceKey, asked map[string]bool) []string {
	if key.Milestone != "" {
		if _, ok := g.Gates[key.Milestone]; ok {
			return []string{key.Milestone}
		}
		return nil
	}
	if len(key.Epics) == 0 {
		return nil
	}
	var out []string
	for _, m := range sortedMapKeys(g.Gates) {
		listed, held := true, false
		for _, e := range g.Epics {
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
		for _, e := range g.Epics {
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
		return nil, fmt.Errorf("milestone %q is in no track", key.Milestone)
	}
	var ids []string
	seen := map[string]bool{}
	for _, e := range g.Epics {
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
// an asked epic.
func epicSliceContext(g *model.EpicGraph, asked map[string]bool) []EpicContextBlock {
	near := map[string]bool{}
	dependency := map[string]bool{}
	for _, e := range g.Epics {
		for _, d := range e.DependsOn {
			if asked[e.ID] {
				near[d.ID] = true
				dependency[d.ID] = true
			}
			if asked[d.ID] {
				near[e.ID] = true
			}
		}
	}
	var ctx []EpicContextBlock
	seen := map[string]bool{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if near[e.ID] && !asked[e.ID] && !seen[e.ID] {
			seen[e.ID] = true
			ctx = append(ctx, EpicContextBlock{ID: e.ID, Block: epicBlock(e), Dependency: dependency[e.ID]})
		}
	}
	return ctx
}

// epicUnit builds an epic's unit; a dependency the graph does not hold adds
// nothing to the hash.
func epicUnit(e *model.Epic, byID map[string]*model.Epic) EpicReviewUnit {
	block := epicBlock(e)
	var hashed strings.Builder
	hashed.WriteString(block)
	for _, d := range e.DependsOn {
		if dep, ok := byID[d.ID]; ok {
			hashed.WriteString("\x00" + epicBlock(dep))
		}
	}
	return EpicReviewUnit{Key: "epic:" + e.ID, Block: block, Hash: epicHash(hashed.String())}
}

// epicBlock renders an epic's id, title, milestone, tags, acceptance and each
// outgoing edge with its why.
func epicBlock(e *model.Epic) string {
	var b strings.Builder
	b.WriteString("epic: " + e.ID + "\n")
	b.WriteString("title: " + e.Title + "\n")
	b.WriteString("milestone: " + e.Milestone + "\n")
	if len(e.Tags) > 0 {
		b.WriteString("tags: " + strings.Join(e.Tags, ", ") + "\n")
	}
	if e.Acceptance != "" {
		b.WriteString("acceptance: " + e.Acceptance + "\n")
	}
	if len(e.DependsOn) > 0 {
		b.WriteString("depends_on:\n")
		for _, d := range e.DependsOn {
			b.WriteString("- " + d.ID + ": " + d.Why + "\n")
		}
	}
	return b.String()
}

// epicGateBlock renders a milestone's gate: its members and deferrables.
func epicGateBlock(milestone string, gate model.EpicGate) string {
	block := "gate: " + milestone + "\nmembers: " + strings.Join(gate.Members, ", ") + "\n"
	if len(gate.Deferrable) > 0 {
		block += "deferrable: " + strings.Join(gate.Deferrable, ", ") + "\n"
	}
	return block
}

func epicHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
