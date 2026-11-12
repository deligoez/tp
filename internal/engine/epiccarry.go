package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicCarry is one emission's split of a slice's units (spec/1.3.0.md §5):
// the units asked this round and those carried from the round before, both
// in slice order. Changed is true when an epic or gate unit's hash differs
// from the round before's or has none recorded (D7); Fixed holds the round
// before's rows on an epic or gate unit resolved fixed. Either one appends
// the regression role (D2), which reads only those kinds
// (EpicRegressionKinds); ScopeFixed holds the fixed rows on any other unit,
// which its own role confirms. Decided holds every earlier round's rows
// accepted wontfix or duplicate on an asked unit, each with its round. Before
// holds, for each asked unit whose own block changed, the block the round
// before read.
type EpicCarry struct {
	Asked      []EpicReviewUnit
	Carried    []EpicReviewUnit
	Changed    bool
	Fixed      []map[string]any
	ScopeFixed []map[string]any
	Decided    []EpicDecision
	Before     map[string]string
	// VerifyOnly is true when the round before left nothing open on an epic
	// or gate unit and fixed no blocking finding there, at least one
	// non-blocking one: the change must still be read, but by the regression
	// role alone, beside the roles a scope unit or a purpose change asks.
	VerifyOnly bool
	// PurposeStale holds, in slice order, the units Asked does not hold whose
	// milestone's purpose changed since the round before's snapshot
	// (spec/backlog/epic-scope.md, Part B). Only a role with purpose: true is
	// asked them; a purpose change is no unit change, so they set neither
	// Changed nor VerifyOnly.
	PurposeStale []EpicReviewUnit
}

// EpicRegressionKinds are the unit kinds the built-in regression role reads.
// A scope unit's change or fix never appends it: the scope role reads that.
var EpicRegressionKinds = []string{model.RoleUnitEpics, model.RoleUnitGates}

// epicRegressionKind reports whether a unit key is of EpicRegressionKinds.
func epicRegressionKind(key string) bool {
	return slices.Contains(EpicRegressionKinds, EpicUnitKind(key))
}

// EpicDecision is one earlier row accepted wontfix or duplicate, and the
// round that recorded it.
type EpicDecision struct {
	Round int
	Row   map[string]any
}

// Regression reports whether the round appends the built-in regression role.
func (c *EpicCarry) Regression() bool {
	return c.Changed || len(c.Fixed) > 0
}

// EpicReviewCarry decides which units round asks. Round 1 asks every unit.
// From round 2 a unit is carried when its hash equals the one snapshotted at
// the round before's emission and no row of that round on it is open; a row
// is open unless it was accepted wontfix or duplicate with evidence, so a row
// resolved fixed re-asks its unit (D2). The rows are read now, at emission.
func EpicReviewCarry(stateDir string, round int, units []EpicReviewUnit) (EpicCarry, error) {
	if round < 2 {
		return EpicCarry{Asked: units}, nil
	}
	prev, err := readEpicSnapshotFile(stateDir, round-1)
	if err != nil {
		return EpicCarry{}, err
	}
	c := EpicCarry{Before: map[string]string{}}
	reask, err := c.readPriorRows(filepath.Join(stateDir, epicRoundFile(round-1)))
	if err != nil {
		return EpicCarry{}, err
	}
	for _, u := range units {
		hash, ok := prev.Units[u.Key]
		changed := !ok || hash != u.Hash
		c.Changed = c.Changed || changed && epicRegressionKind(u.Key)
		if !changed && !reask[u.Key] {
			c.Carried = append(c.Carried, u)
			continue
		}
		c.Asked = append(c.Asked, u)
		if before, had := prev.Blocks[u.Key]; had && before != u.Block {
			c.Before[u.Key] = before
		}
	}
	c.PurposeStale = epicPurposeStale(prev.Purposes, units, c.Asked)
	if err := c.readDecided(stateDir, round); err != nil {
		return EpicCarry{}, err
	}
	return c, nil
}

// epicPurposeStale returns, in units' order, the units outside skip whose
// milestone's purpose differs from the one before recorded: an absent entry
// is no purpose, so an old snapshot with none re-asks nothing while no
// purpose is written, and a removed purpose is a change too.
func epicPurposeStale(before map[string]string, units, skip []EpicReviewUnit) []EpicReviewUnit {
	asked := map[string]bool{}
	for _, u := range skip {
		asked[u.Key] = true
	}
	var out []EpicReviewUnit
	for _, u := range units {
		if !asked[u.Key] && before[u.Milestone] != u.Purpose {
			out = append(out, u)
		}
	}
	return out
}

// readDecided collects every earlier round's rows accepted wontfix or
// duplicate on a unit this round asks, purpose-stale ones included, so the
// prompt can say they are decided rather than have a role raise them again.
func (c *EpicCarry) readDecided(stateDir string, round int) error {
	asked := map[string]bool{}
	for _, u := range append(slices.Clone(c.Asked), c.PurposeStale...) {
		asked[u.Key] = true
	}
	for r := 1; r < round; r++ {
		path := filepath.Join(stateDir, epicRoundFile(r))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}
		rows, ok := readNDJSONRows(path)
		if !ok {
			return fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
		}
		for _, row := range rows {
			loc, _ := row["location"].(string)
			if asked[loc] && RowAccepted(row) {
				c.Decided = append(c.Decided, EpicDecision{Round: r, Row: row})
			}
		}
	}
	return nil
}

// EpicIndexedRow is one row of a recorded round and its 0-based index there,
// the index tp epic review --resolve takes.
type EpicIndexedRow struct {
	Index int
	Row   map[string]any
}

// EpicOpenRowsOn returns the rows of the round before round, the last
// recorded one, that are not accepted and sit on a unit keys names, each with
// its index: the rows that re-ask those units. Before round 2 there is none.
func EpicOpenRowsOn(stateDir string, round int, keys map[string]bool) ([]EpicIndexedRow, error) {
	if round < 2 || len(keys) == 0 {
		return nil, nil
	}
	path := filepath.Join(stateDir, epicRoundFile(round-1))
	rows, ok := readNDJSONRows(path)
	if !ok {
		return nil, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
	}
	var out []EpicIndexedRow
	for i, row := range rows {
		if loc, _ := row["location"].(string); keys[loc] && !RowAccepted(row) {
			out = append(out, EpicIndexedRow{Index: i, Row: row})
		}
	}
	return out, nil
}

// readPriorRows collects the round before's fixed rows into c, an epic or
// gate unit's into Fixed and any other into ScopeFixed, and returns the unit
// keys an open row names. Only the epic and gate rows decide VerifyOnly.
func (c *EpicCarry) readPriorRows(path string) (map[string]bool, error) {
	rows, ok := readNDJSONRows(path)
	if !ok {
		return nil, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
	}
	reask := map[string]bool{}
	onlyLowFixes := true
	for _, row := range rows {
		loc, _ := row["location"].(string)
		regression := epicRegressionKind(loc)
		fixed := rowFixed(row)
		switch {
		case fixed && regression:
			c.Fixed = append(c.Fixed, row)
		case fixed:
			c.ScopeFixed = append(c.ScopeFixed, row)
		}
		if regression && (fixed && ReviewRowBlocking(row) || !fixed && !RowAccepted(row)) {
			onlyLowFixes = false
		}
		if !RowAccepted(row) {
			reask[loc] = true
		}
	}
	c.VerifyOnly = onlyLowFixes && len(c.Fixed) > 0
	return reask, nil
}

// readEpicSnapshotFile returns round's snapshot, hashes and blocks; an absent
// one is empty. A snapshot written before blocks were kept has none.
func readEpicSnapshotFile(stateDir string, round int) (epicSnapshot, error) {
	data, err := os.ReadFile(filepath.Join(stateDir, epicSnapshotFile(round)))
	if os.IsNotExist(err) {
		return epicSnapshot{Units: map[string]string{}}, nil
	}
	if err != nil {
		return epicSnapshot{}, err
	}
	var snap epicSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return epicSnapshot{}, fmt.Errorf("epic review snapshot of round %d: %w", round, err)
	}
	return snap, nil
}

// EpicCarriedContext returns the context of an emission asking only asked:
// the block of every neighbour of an asked epic that is not asked itself,
// carried or not, each once, in file order (D5).
func EpicCarriedContext(g *model.EpicGraph, asked []EpicReviewUnit) []EpicContextBlock {
	ids := map[string]bool{}
	for _, u := range asked {
		if id, ok := epicUnitID(u.Key); ok {
			ids[id] = true
		}
	}
	return epicSliceContext(g, ids)
}

// epicUnitID returns the epic id of an "epic:<id>" unit key.
func epicUnitID(key string) (string, bool) {
	return strings.CutPrefix(key, "epic:")
}
