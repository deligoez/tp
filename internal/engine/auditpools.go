package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuditRolePool is one code role's pool at emission: how many files the list
// its prompt carried was cut from (the prompt's file_pool).
type AuditRolePool struct {
	Role string `json:"role"`
	Pool int    `json:"pool"`
	// Items are the file_check item ids the role's prompt listed; only a row
	// naming one of them counts as graded.
	Items []string `json:"items"`
}

// RoleGraded is "graded N of M" for one code role of a recorded audit round
// (spec/backlog/checklist-covers-what-changed.md, decision 4): Graded is the
// distinct file_check items the role returned a row for, whatever the
// verdict, and Pool is the pool its list was cut from, the prompt's file_pool
// under the same name.
type RoleGraded struct {
	Role   string `json:"role"`
	Graded int    `json:"graded"`
	Pool   int    `json:"pool"`
}

// auditPoolsFile is the on-disk form of a round's pools.
type auditPoolsFile struct {
	Round int             `json:"round"`
	Pools []AuditRolePool `json:"pools"`
}

// AuditPoolsPath is where an emission keeps a round's pools until --record:
// <state dir>/audit-inbox/round-<N>.pools.json. The inbox is the round's
// in-flight place — state.json may not exist yet, since an audit emission
// never calls EnsureReviewState, and the snapshot beside it is the spec's
// bytes, which the round's hash is taken from. The name ends in .json, not
// .ndjson, so it is no role file: the write fence refuses it, --merge never
// reads it, and RemoveRoundInbox takes it with the round's role files.
func AuditPoolsPath(specPath string, round int) string {
	return filepath.Join(roleInboxDir(specPath, PhaseAudit), fmt.Sprintf("round-%d.pools.json", round))
}

// WriteAuditPools writes a round's pools atomically. Every sibling unit of a
// round emits, each writing the same pools, so a concurrent writer replaces
// the file with the same bytes.
func WriteAuditPools(specPath string, round int, pools []AuditRolePool) error {
	path := AuditPoolsPath(specPath, round)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(auditPoolsFile{Round: round, Pools: pools}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(path, append(data, '\n'))
}

// ReadAuditPools reads a round's pools; found is false when no emission wrote
// any, which is the normal case for a round recorded without one.
func ReadAuditPools(specPath string, round int) (pools []AuditRolePool, found bool, err error) {
	path := AuditPoolsPath(specPath, round)
	data, err := os.ReadFile(path) //nolint:gosec // a path built from the spec's own state dir
	if os.IsNotExist(err) {
		// The round's record moved its pools beside the round file, so a
		// re-record grades its new rows against the same asked items.
		path = RecordedAuditPoolsPath(specPath, round)
		data, err = os.ReadFile(path) //nolint:gosec // a path built from the spec's own state dir
	}
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	var f auditPoolsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return f.Pools, true, nil
}

// RecordedAuditPoolsPath is where a recorded audit round keeps the pools its
// emission wrote, beside its round file: audit-round-<N>.pools.json. A
// re-record of the round (TP_ROUND) reads them there once --record has
// emptied the inbox, so a rewritten round is graded against what it was asked
// (spec/backlog/role-presence.md).
func RecordedAuditPoolsPath(specPath string, round int) string {
	return filepath.Join(ReviewStateDir(specPath), fmt.Sprintf("audit-round-%d.pools.json", round))
}

// KeepRecordedAuditPools copies a round's pools beside its round file.
func KeepRecordedAuditPools(specPath string, round int, pools []AuditRolePool) error {
	data, err := json.MarshalIndent(auditPoolsFile{Round: round, Pools: pools}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(RecordedAuditPoolsPath(specPath, round), append(data, '\n'))
}

// GradedPrefix renders a round's Graded entries as the sentence that opens
// next_action, "latest round graded security 7 of 46, quality 10 of 46. ", or
// "" when the round recorded none. It leads rather than follows: next_action
// ends with the command to run, and copying that command must copy nothing
// else.
func GradedPrefix(graded []RoleGraded) string {
	if len(graded) == 0 {
		return ""
	}
	parts := make([]string, 0, len(graded))
	for _, g := range graded {
		parts = append(parts, fmt.Sprintf("%s %d of %d", g.Role, g.Graded, g.Pool))
	}
	return "latest round graded " + strings.Join(parts, ", ") + ". "
}

// GradeAuditRoles returns, in pool order, how many distinct emitted
// file_check items each pooled role returned a row for. A row counts whatever
// its status, so a file read and found clean is graded; a row of another
// role, or one naming an item the role's prompt never listed, is not, so
// Graded never exceeds the prompt's file_count.
func GradeAuditRoles(pools []AuditRolePool, rows []map[string]any) []RoleGraded {
	emitted := make(map[string]map[string]bool, len(pools))
	seen := make(map[string]map[string]bool, len(pools))
	for _, p := range pools {
		emitted[p.Role] = make(map[string]bool, len(p.Items))
		for _, id := range p.Items {
			emitted[p.Role][id] = true
		}
		seen[p.Role] = make(map[string]bool)
	}
	for _, row := range rows {
		role, _ := row["role"].(string)
		id, _ := row["item_id"].(string)
		if role == "" {
			// A row that names no role is graded for the one pool that
			// asked its id; an id two pools asked stays unattributed.
			role = onlyPoolAsking(pools, id)
		}
		if emitted[role][id] {
			seen[role][id] = true
		}
	}
	graded := make([]RoleGraded, 0, len(pools))
	for _, p := range pools {
		graded = append(graded, RoleGraded{Role: p.Role, Graded: len(seen[p.Role]), Pool: p.Pool})
	}
	return graded
}

// SpecCoverageUnproven reports whether a recorded audit round leaves
// spec-coverage unproven (spec/backlog/role-presence.md §2.2, §2.3): the role
// answered fewer of its asked checklist items than it was asked, or the round
// was recorded since v1.20.0 without an emission. Graded counts only distinct
// asked ids, so Graded below Pool is containment failing. A round asked
// nothing, a code role's cut list and a round from before 1.20.0 are not.
func SpecCoverageUnproven(entry *ReviewRound) bool {
	return UnprovenReason(entry) != ""
}

// UnprovenReason is the clause that says why a round leaves spec-coverage
// unproven, leading next_action so a round recorded with no finding and not
// clean is never left unexplained; "" when it does not.
func UnprovenReason(entry *ReviewRound) string {
	if entry.Unasked {
		return "the round was recorded without its emission, so nothing shows what spec-coverage was asked and it is not clean; emit the round and record over it. "
	}
	for _, g := range entry.Graded {
		if g.Role == RoleSpecCoverage && g.Graded < g.Pool {
			return fmt.Sprintf("spec-coverage answered %d of the %d checklist items it was asked, so the round is not clean; run it again for the rest. ", g.Graded, g.Pool)
		}
	}
	return ""
}

// onlyPoolAsking returns the role of the one pool whose items hold id, or ""
// when none or more than one does.
func onlyPoolAsking(pools []AuditRolePool, id string) string {
	owner := ""
	for _, p := range pools {
		for _, item := range p.Items {
			if item != id {
				continue
			}
			if owner != "" {
				return ""
			}
			owner = p.Role
			break
		}
	}
	return owner
}
