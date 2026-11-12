package engine

import (
	"regexp"
	"slices"
	"strings"
)

// EpicGraphDiff is the difference between two snapshots of a slice
// (spec/1.8.0.md §11): the units added and removed, the units whose hash
// changed, the edges added and removed, and each why and field changed with
// both texts, each list sorted by unit. HashesOnly is true when either
// snapshot was written before blocks were kept: then only the unit lists are
// known, and the edge, why and field lists are empty.
type EpicGraphDiff struct {
	UnitsAdded    []string          `json:"units_added"`
	UnitsRemoved  []string          `json:"units_removed"`
	UnitsChanged  []string          `json:"units_changed"`
	EdgesAdded    []EpicEdgeChange  `json:"edges_added"`
	EdgesRemoved  []EpicEdgeChange  `json:"edges_removed"`
	WhysChanged   []EpicWhyChange   `json:"whys_changed"`
	FieldsChanged []EpicFieldChange `json:"fields_changed"`
	HashesOnly    bool              `json:"hashes_only"`
	// Unchanged is the context of the difference: the units in both
	// snapshots under the same hash. A compact reader drops it.
	Unchanged []string `json:"unchanged,omitempty"`
}

// EpicEdgeChange is one edge a unit gained or lost: To names the dependency
// as the block does, a group as one of {A, B}.
type EpicEdgeChange struct {
	Unit string `json:"unit"`
	To   string `json:"to"`
	Why  string `json:"why"`
}

// EpicWhyChange is one edge whose why was rewritten.
type EpicWhyChange struct {
	Unit   string `json:"unit"`
	To     string `json:"to"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// EpicFieldChange is one field of a unit's block whose value changed, added
// or removed; an absent field reads as "".
type EpicFieldChange struct {
	Unit   string `json:"unit"`
	Field  string `json:"field"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// EpicReviewRoundsDiff is the difference between the snapshots of rounds
// from and to in a slice's state directory. An absent snapshot holds no unit.
func EpicReviewRoundsDiff(stateDir string, from, to int) (EpicGraphDiff, error) {
	before, err := readEpicSnapshotFile(stateDir, from)
	if err != nil {
		return EpicGraphDiff{}, err
	}
	after, err := readEpicSnapshotFile(stateDir, to)
	if err != nil {
		return EpicGraphDiff{}, err
	}
	return diffEpicSnapshots(before, after), nil
}

// EpicReviewCurrentDiff is the difference between round's snapshot and the
// slice's units as the graph now renders them.
func EpicReviewCurrentDiff(stateDir string, round int, units []EpicReviewUnit) (EpicGraphDiff, error) {
	before, err := readEpicSnapshotFile(stateDir, round)
	if err != nil {
		return EpicGraphDiff{}, err
	}
	now := epicSnapshot{Units: make(map[string]string, len(units)), Blocks: make(map[string]string, len(units))}
	for _, u := range units {
		now.Units[u.Key] = u.Hash
		now.Blocks[u.Key] = u.Block
	}
	return diffEpicSnapshots(before, now), nil
}

// diffEpicSnapshots compares two snapshots unit by unit, and block by block
// when both kept their blocks.
func diffEpicSnapshots(before, after epicSnapshot) EpicGraphDiff {
	d := EpicGraphDiff{
		UnitsAdded: []string{}, UnitsRemoved: []string{}, UnitsChanged: []string{},
		EdgesAdded: []EpicEdgeChange{}, EdgesRemoved: []EpicEdgeChange{},
		WhysChanged: []EpicWhyChange{}, FieldsChanged: []EpicFieldChange{},
		HashesOnly: !snapshotHasBlocks(before) || !snapshotHasBlocks(after),
	}
	for _, key := range sortedMapKeys(after.Units) {
		if _, ok := before.Units[key]; !ok {
			d.UnitsAdded = append(d.UnitsAdded, key)
		}
	}
	for _, key := range sortedMapKeys(before.Units) {
		hash, ok := after.Units[key]
		switch {
		case !ok:
			d.UnitsRemoved = append(d.UnitsRemoved, key)
		case hash != before.Units[key]:
			d.UnitsChanged = append(d.UnitsChanged, key)
		default:
			d.Unchanged = append(d.Unchanged, key)
		}
		if ok && !d.HashesOnly {
			d.diffBlocks(key, parseEpicBlock(before.Blocks[key]), parseEpicBlock(after.Blocks[key]))
		}
	}
	return d
}

// snapshotHasBlocks reports whether every unit of snap kept its block.
func snapshotHasBlocks(snap epicSnapshot) bool {
	for key := range snap.Units {
		if _, ok := snap.Blocks[key]; !ok {
			return false
		}
	}
	return true
}

// diffBlocks adds the field, edge and why changes of one unit to d.
func (d *EpicGraphDiff) diffBlocks(key string, before, after parsedEpicBlock) {
	for _, f := range unionKeys(before.fields, after.fields) {
		if b, a := before.fields[f], after.fields[f]; b != a {
			d.FieldsChanged = append(d.FieldsChanged, EpicFieldChange{Unit: key, Field: f, Before: b, After: a})
		}
	}
	for _, to := range after.edgeOrder {
		why, had := before.edges[to]
		switch {
		case !had:
			d.EdgesAdded = append(d.EdgesAdded, EpicEdgeChange{Unit: key, To: to, Why: after.edges[to]})
		case why != after.edges[to]:
			d.WhysChanged = append(d.WhysChanged, EpicWhyChange{Unit: key, To: to, Before: why, After: after.edges[to]})
		}
	}
	for _, to := range before.edgeOrder {
		if _, kept := after.edges[to]; !kept {
			d.EdgesRemoved = append(d.EdgesRemoved, EpicEdgeChange{Unit: key, To: to, Why: before.edges[to]})
		}
	}
}

// unionKeys returns the keys of a and b, sorted, each once.
func unionKeys(a, b map[string]string) []string {
	keys := make([]string, 0, len(a)+len(b))
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// parsedEpicBlock is a unit block read back: its fields by name, and its
// edges by the dependency they name, in block order.
type parsedEpicBlock struct {
	fields    map[string]string
	edges     map[string]string
	edgeOrder []string
}

// epicBlockFieldRe matches a field line of a block epicBlock or
// epicGateBlock renders.
var epicBlockFieldRe = regexp.MustCompile(`^([a-z_]+): (.*)$`)

// parseEpicBlock reads a block back into its fields and edges. A line that is
// neither continues the value before it, as a multi-line acceptance or why
// renders; the description, the block's last section when it has one
// (splitEpicDescription), is its field, un-indented.
func parseEpicBlock(block string) parsedEpicBlock {
	p := parsedEpicBlock{fields: map[string]string{}, edges: map[string]string{}}
	rest, desc, described := splitEpicDescription(block)
	if described {
		p.fields["description"] = desc
	}
	inDeps := false
	last := func(s string) {}
	for line := range strings.SplitSeq(strings.TrimSuffix(rest, "\n"), "\n") {
		if line == "depends_on:" {
			inDeps = true
			continue
		}
		if to, why, ok := strings.Cut(strings.TrimPrefix(line, "- "), ": "); inDeps && ok && strings.HasPrefix(line, "- ") {
			p.edges[to] = why
			p.edgeOrder = append(p.edgeOrder, to)
			last = func(s string) { p.edges[to] += "\n" + s }
			continue
		}
		if m := epicBlockFieldRe.FindStringSubmatch(line); m != nil && !inDeps {
			p.fields[m[1]] = m[2]
			name := m[1]
			last = func(s string) { p.fields[name] += "\n" + s }
			continue
		}
		last(line)
	}
	return p
}
