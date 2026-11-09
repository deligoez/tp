package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicCarry is one emission's split of a slice's units (spec/1.3.0.md §5):
// the units asked this round and those carried from the round before, both
// in slice order. Changed is true when a unit's hash differs from the round
// before's or has none recorded (D7); Fixed holds the round before's rows
// resolved fixed. Either one appends the regression role (D2). Decided holds
// every earlier round's rows accepted wontfix or duplicate on an asked unit,
// each with its round. Before holds, for each asked unit whose own block
// changed, the block the round before read.
type EpicCarry struct {
	Asked   []EpicReviewUnit
	Carried []EpicReviewUnit
	Changed bool
	Fixed   []map[string]any
	Decided []EpicDecision
	Before  map[string]string
	// VerifyOnly is true when the round before left nothing open and fixed
	// no blocking finding, at least one non-blocking one: the change must still
	// be read, but by the regression role alone.
	VerifyOnly bool
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
		c.Changed = c.Changed || changed
		if !changed && !reask[u.Key] {
			c.Carried = append(c.Carried, u)
			continue
		}
		c.Asked = append(c.Asked, u)
		if before, had := prev.Blocks[u.Key]; had && before != u.Block {
			c.Before[u.Key] = before
		}
	}
	if err := c.readDecided(stateDir, round); err != nil {
		return EpicCarry{}, err
	}
	return c, nil
}

// readDecided collects every earlier round's rows accepted wontfix or
// duplicate on a unit this round asks, so the prompt can say they are
// decided rather than have a role raise them again.
func (c *EpicCarry) readDecided(stateDir string, round int) error {
	asked := map[string]bool{}
	for _, u := range c.Asked {
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

// readPriorRows collects the round before's fixed rows into c and returns the
// unit keys an open row names.
func (c *EpicCarry) readPriorRows(path string) (map[string]bool, error) {
	rows, ok := readNDJSONRows(path)
	if !ok {
		return nil, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
	}
	reask := map[string]bool{}
	onlyLowFixes := true
	for _, row := range rows {
		fixed := rowFixed(row)
		if fixed {
			c.Fixed = append(c.Fixed, row)
		}
		if fixed && ReviewRowBlocking(row) || !fixed && !RowAccepted(row) {
			onlyLowFixes = false
		}
		if !RowAccepted(row) {
			loc, _ := row["location"].(string)
			reask[loc] = true
		}
	}
	c.VerifyOnly = onlyLowFixes && len(c.Fixed) > 0
	return reask, nil
}

// readEpicSnapshot returns the unit hashes snapshotted at round's emission;
// an absent snapshot records no hash, so every unit counts as changed (D7).
func readEpicSnapshot(stateDir string, round int) (map[string]string, error) {
	snap, err := readEpicSnapshotFile(stateDir, round)
	return snap.Units, err
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
