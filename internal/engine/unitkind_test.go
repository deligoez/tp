package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeUnitFile writes content at path, creating parent directories.
func writeUnitFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// unitTaskFile / unitSpec / unitRoundDir name the fixture artifacts a target
// points at inside a test's own directory.
func unitTaskFile(dir string) string { return filepath.Join(dir, "spec.tasks.json") }
func unitSpec(dir string) string     { return filepath.Join(dir, "spec.md") }
func unitRoundDir(dir string) string { return filepath.Join(dir, "rounds", "r3") }

// taskFileJSON renders a task file holding one task with the given status, or
// the empty init shell when status is "".
func taskFileJSON(status string) string {
	if status == "" {
		return `{"version":1,"tasks":[]}`
	}
	return `{"version":1,"tasks":[{"id":"t1","title":"T","status":"` + status + `"}]}`
}

func TestUnitKinds_EightKindsInTableOrder(t *testing.T) {
	want := []UnitKind{
		"implement", "review-role", "review-record", "review-resolve",
		"decompose", "audit-role", "audit-record", "audit-fix",
	}
	assert.Equal(t, want, UnitKinds(), "the eight §3.3 kinds, in table order")

	// The accessor hands out a copy: a caller cannot reorder the shared set.
	got := UnitKinds()
	got[0] = "clobbered"
	assert.Equal(t, want, UnitKinds())

	for _, k := range want {
		parsed, ok := ParseUnitKind(string(k))
		assert.True(t, ok, "%s parses", k)
		assert.Equal(t, k, parsed)
	}
	for _, bad := range []string{"", "Implement", "implement ", "review_role", "run", "role"} {
		_, ok := ParseUnitKind(bad)
		assert.False(t, ok, "%q is not a unit kind", bad)
	}
}

func TestUnitKind_ConcurrencyFromTable(t *testing.T) {
	cases := []struct {
		kind       UnitKind
		want       UnitConcurrency
		concurrent bool
	}{
		{UnitImplement, ConcurrencyAlone, false},
		{UnitReviewRole, ConcurrencySiblingRoles, true},
		{UnitReviewRecord, ConcurrencyAlone, false},
		{UnitReviewResolve, ConcurrencyAlone, false},
		{UnitDecompose, ConcurrencyAlone, false},
		{UnitAuditRole, ConcurrencySiblingRoles, true},
		{UnitAuditRecord, ConcurrencyAlone, false},
		{UnitAuditFix, ConcurrencyAlone, false},
		// An unrecognized kind is alone — the safe direction.
		{UnitKind("mystery"), ConcurrencyAlone, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.kind.Concurrency(), "%s concurrency", c.kind)
		assert.Equal(t, c.concurrent, c.kind.Concurrent(), "%s concurrent", c.kind)
	}
}

// TestUnitKind_DurableWrite_DecidableWithNoBaseline is test 18: every kind's
// predicate is decided by reading the artifacts §3.3 names for it, as they are.
// Each "present" case writes its artifacts before the predicate is ever called,
// so no "before" observation exists to compare against; each "absent" case is
// one named artifact short of satisfying it.
func TestUnitKind_DurableWrite_DecidableWithNoBaseline(t *testing.T) {
	cases := []struct {
		name    string
		kind    UnitKind
		target  func(dir string) UnitTarget
		present func(t *testing.T, dir string)
		absent  func(t *testing.T, dir string)
	}{
		{
			name:   "implement",
			kind:   UnitImplement,
			target: func(dir string) UnitTarget { return UnitTarget{TaskFile: unitTaskFile(dir), ID: "t1"} },
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, unitTaskFile(dir), taskFileJSON("done"))
			},
			absent: func(t *testing.T, dir string) {
				writeUnitFile(t, unitTaskFile(dir), taskFileJSON("wip"))
			},
		},
		{
			name:   "review-role",
			kind:   UnitReviewRole,
			target: func(dir string) UnitTarget { return UnitTarget{RoundDir: unitRoundDir(dir), ID: "architect"} },
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, RoleFindingsPath(unitRoundDir(dir), "architect"), "{\"class\":\"c\"}\n")
			},
			absent: func(t *testing.T, dir string) {
				// Only the .part the unit itself writes: the rename that
				// completes the durable write never happened (§3.3.1).
				writeUnitFile(t, RoleFindingsPath(unitRoundDir(dir), "architect")+".part", "{\"class\":\"c\"}\n")
			},
		},
		{
			name: "review-record",
			kind: UnitReviewRecord,
			target: func(dir string) UnitTarget {
				return UnitTarget{Spec: unitSpec(dir), RoundDir: unitRoundDir(dir), Round: 3, ID: "3"}
			},
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "{\"class\":\"c\"}\n")
				writeUnitFile(t, filepath.Join(ReviewStateDir(unitSpec(dir)), "review-round-3.ndjson"), "{}\n")
			},
			absent: func(t *testing.T, dir string) {
				// Merged but never recorded: the second named artifact is missing.
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "{\"class\":\"c\"}\n")
			},
		},
		{
			name: "review-resolve",
			kind: UnitReviewResolve,
			target: func(dir string) UnitTarget {
				return UnitTarget{RoundDir: unitRoundDir(dir), ID: "spec"}
			},
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)),
					"{\"class\":\"a\",\"resolved\":{\"status\":\"fixed\"}}\n{\"class\":\"b\",\"resolved\":{\"status\":\"wontfix\"}}\n")
			},
			absent: func(t *testing.T, dir string) {
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)),
					"{\"class\":\"a\",\"resolved\":{\"status\":\"fixed\"}}\n{\"class\":\"b\"}\n")
			},
		},
		{
			name:   "decompose",
			kind:   UnitDecompose,
			target: func(dir string) UnitTarget { return UnitTarget{TaskFile: unitTaskFile(dir), ID: "spec"} },
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, unitTaskFile(dir), taskFileJSON("open"))
			},
			absent: func(t *testing.T, dir string) {
				// The init shell: a task file that holds zero tasks.
				writeUnitFile(t, unitTaskFile(dir), taskFileJSON(""))
			},
		},
		{
			name:   "audit-role",
			kind:   UnitAuditRole,
			target: func(dir string) UnitTarget { return UnitTarget{RoundDir: unitRoundDir(dir), ID: "go-safety"} },
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, RoleFindingsPath(unitRoundDir(dir), "go-safety"), "{\"item_id\":\"i1\",\"status\":\"PASS\"}\n")
			},
			absent: func(t *testing.T, dir string) {
				// A sibling role's file is not this role's.
				writeUnitFile(t, RoleFindingsPath(unitRoundDir(dir), "ax-contract"), "{\"item_id\":\"i1\",\"status\":\"PASS\"}\n")
			},
		},
		{
			name: "audit-record",
			kind: UnitAuditRecord,
			target: func(dir string) UnitTarget {
				return UnitTarget{Spec: unitSpec(dir), RoundDir: unitRoundDir(dir), Round: 3, ID: "3"}
			},
			present: func(t *testing.T, dir string) {
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "{\"item_id\":\"i1\",\"status\":\"FAIL\"}\n")
				writeUnitFile(t, filepath.Join(ReviewStateDir(unitSpec(dir)), "audit-round-3.ndjson"), "{}\n")
			},
			absent: func(t *testing.T, dir string) {
				// A review round file for the same number is not an audit one.
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "{\"item_id\":\"i1\",\"status\":\"FAIL\"}\n")
				writeUnitFile(t, filepath.Join(ReviewStateDir(unitSpec(dir)), "review-round-3.ndjson"), "{}\n")
			},
		},
		{
			name: "audit-fix",
			kind: UnitAuditFix,
			target: func(dir string) UnitTarget {
				return UnitTarget{RoundDir: unitRoundDir(dir), ID: "go-safety:item-4"}
			},
			present: func(t *testing.T, dir string) {
				// No code change at all — the row's disposition is the whole
				// durable write (§3.3.1).
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)),
					"{\"role\":\"go-safety\",\"item_id\":\"item-4\",\"status\":\"FAIL\",\"resolved\":{\"status\":\"wontfix\"}}\n")
			},
			absent: func(t *testing.T, dir string) {
				writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)),
					"{\"role\":\"go-safety\",\"item_id\":\"item-4\",\"status\":\"FAIL\"}\n")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name+"/present", func(t *testing.T) {
			dir := t.TempDir()
			c.present(t, dir)
			target := c.target(dir)
			assert.True(t, c.kind.DurableWrite(target), "durable write present")
			// A state, never a delta: a second reading of the same artifacts
			// answers the same without any baseline carried between calls.
			assert.True(t, c.kind.DurableWrite(target), "predicate is stable")
		})
		t.Run(c.name+"/absent", func(t *testing.T) {
			dir := t.TempDir()
			c.absent(t, dir)
			assert.False(t, c.kind.DurableWrite(c.target(dir)), "one named artifact short")
		})
		t.Run(c.name+"/nothing-written", func(t *testing.T) {
			dir := t.TempDir()
			assert.False(t, c.kind.DurableWrite(c.target(dir)), "no artifacts at all")
		})
	}
}

