package engine

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// EpicStoredPanel returns the panel round's emission stored in stateDir, in
// panel order, and whether a snapshot is there; nil roles with stored true is
// a snapshot written before the panel was kept.
func EpicStoredPanel(stateDir string, round int) (roles []EpicSnapshotRole, stored bool, err error) {
	if _, statErr := os.Stat(filepath.Join(stateDir, epicSnapshotFile(round))); os.IsNotExist(statErr) {
		return nil, false, nil
	}
	snap, err := readEpicSnapshotFile(stateDir, round)
	if err != nil {
		return nil, true, err
	}
	return snap.Roles, true, nil
}

// EpicEmissionChanges compares round's emission stored in stateDir with the
// one units and roles rebuild, over the whole panel (spec/backlog/
// epic-reemit-guard.md): the unit keys with their hashes and blocks, the
// milestones' purposes and the roles with their hashes. It returns one
// phrase per kind of change, empty when they match or nothing is stored. A
// snapshot written before blocks or roles were kept is compared on what it
// holds, so an upgrade alone never reads as a change. Output paths are not
// compared: they name where a role writes, not what it is asked.
func EpicEmissionChanges(stateDir string, round int, units []EpicReviewUnit, roles []EpicSnapshotRole) ([]string, error) {
	if _, err := os.Stat(filepath.Join(stateDir, epicSnapshotFile(round))); os.IsNotExist(err) {
		return nil, nil
	}
	old, err := readEpicSnapshotFile(stateDir, round)
	if err != nil {
		return nil, err
	}
	now := newEpicSnapshot(round, units, roles)
	changes := epicUnitChanges(&old, &now)
	if p := epicPurposeChanges(old.Purposes, now.Purposes); len(p) > 0 {
		changes = append(changes, "purposes changed: "+strings.Join(p, ", "))
	}
	if old.Roles != nil {
		changes = append(changes, epicRoleChanges(old.Roles, now.Roles)...)
	}
	return changes, nil
}

// epicUnitChanges names the units added, removed and changed between two
// snapshots; a block counts only where the old snapshot kept one.
func epicUnitChanges(old, now *epicSnapshot) []string {
	added, removed, both := keyDifference(old.Units, now.Units)
	var changed []string
	for _, k := range both {
		before, kept := old.Blocks[k]
		if old.Units[k] != now.Units[k] || kept && before != now.Blocks[k] {
			changed = append(changed, k)
		}
	}
	return labelled([]string{"units added: ", "units removed: ", "units changed: "}, added, removed, changed)
}

// epicPurposeChanges lists, sorted, the milestones whose purpose differs, an
// absent one reading as none.
func epicPurposeChanges(old, now map[string]string) []string {
	added, removed, both := keyDifference(old, now)
	out := slices.Concat(added, removed)
	for _, m := range both {
		if old[m] != now[m] {
			out = append(out, m)
		}
	}
	return sortedCopy(out)
}

// epicRoleChanges names the roles added to and removed from the panel and
// those whose role file changed.
func epicRoleChanges(old, now []EpicSnapshotRole) []string {
	before, after := roleHashes(old), roleHashes(now)
	added, removed, both := keyDifference(before, after)
	var changed []string
	for _, id := range both {
		if before[id] != after[id] {
			changed = append(changed, id)
		}
	}
	return labelled([]string{"roles added: ", "roles removed: ", "role files changed: "}, added, removed, changed)
}

// roleHashes maps each role's id to its hash.
func roleHashes(roles []EpicSnapshotRole) map[string]string {
	m := make(map[string]string, len(roles))
	for _, r := range roles {
		m[r.ID] = r.Hash
	}
	return m
}

// keyDifference splits the keys of two maps, each sorted, into those only now
// holds, those only old holds, and those both hold.
func keyDifference(old, now map[string]string) (added, removed, both []string) {
	for _, k := range sortedMapKeys(now) {
		if _, ok := old[k]; ok {
			both = append(both, k)
		} else {
			added = append(added, k)
		}
	}
	for _, k := range sortedMapKeys(old) {
		if _, ok := now[k]; !ok {
			removed = append(removed, k)
		}
	}
	return added, removed, both
}

// labelled renders each non-empty list after its label.
func labelled(labels []string, lists ...[]string) []string {
	var out []string
	for i, l := range lists {
		if len(l) > 0 {
			out = append(out, labels[i]+strings.Join(l, ", "))
		}
	}
	return out
}

// RemoveEpicInboxRound removes the slice's inbox files of round, the files
// its emission named (EpicReviewInboxPath), and returns the paths removed.
// An absent inbox removes nothing.
func RemoveEpicInboxRound(stateDir string, round int) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(EpicReviewInboxDir(stateDir), fmt.Sprintf("*-r%d.ndjson", round)))
	if err != nil {
		return nil, err
	}
	for _, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}
	return paths, nil
}

// EpicRoleFileHashes returns, by role id, the hash of each active graph role's
// file, by ResolveEpicRoleCorpus's rule: every .json file of a populated
// .tp/epic-reviewers/, whose stem is its id, else the built-in files. Each is
// the sha256 of the CRLF-normalized content, as ComputeRolesHash reads a
// file, so a checkout's line endings never read as an edit.
func EpicRoleFileHashes(startDir string) (map[string]string, error) {
	out := map[string]string{}
	if tpDir := DiscoverTPDir(startDir); tpDir != "" {
		dir := filepath.Join(tpDir, PhaseEpicReviewers)
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // a role file of the project's own corpus
			if err != nil {
				return nil, fmt.Errorf("cannot read role file %s: %w", e.Name(), err)
			}
			out[strings.TrimSuffix(e.Name(), ".json")] = roleContentHash(data)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	files, err := DefaultEpicCorpusFiles()
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		out[f.ID] = roleContentHash(f.Data)
	}
	return out, nil
}

// roleContentHash is the sha256 of a role file's CRLF-normalized content.
func roleContentHash(data []byte) string {
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(strings.ReplaceAll(string(data), "\r\n", "\n"))))
}

// EpicUnresolvedRows returns the rows of the round before round, the last
// recorded one, that carry no resolution and sit on a unit on reports, each
// with its index. A row resolved fixed passes, since removing a unit can be
// its fix; so does an accepted one. Before round 2 there is none.
func EpicUnresolvedRows(stateDir string, round int, on func(location string) bool) ([]EpicIndexedRow, error) {
	if round < 2 {
		return nil, nil
	}
	path := filepath.Join(stateDir, epicRoundFile(round-1))
	rows, ok := readNDJSONRows(path)
	if !ok {
		return nil, fmt.Errorf("epic review round file %s cannot be read as NDJSON", path)
	}
	var out []EpicIndexedRow
	for i, row := range rows {
		if loc, _ := row["location"].(string); on(loc) && !RowAccepted(row) && !rowFixed(row) {
			out = append(out, EpicIndexedRow{Index: i, Row: row})
		}
	}
	return out, nil
}
