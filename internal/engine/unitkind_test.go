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

func TestUnitKind_DurableWrite_UnknownKindIsNeverComplete(t *testing.T) {
	dir := t.TempDir()
	writeUnitFile(t, unitTaskFile(dir), taskFileJSON("done"))
	writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "{}\n")
	target := UnitTarget{TaskFile: unitTaskFile(dir), Spec: unitSpec(dir), RoundDir: unitRoundDir(dir), Round: 1, ID: "t1"}
	assert.False(t, UnitKind("mystery").DurableWrite(target))
	assert.False(t, UnitKind("").DurableWrite(target))
}

// TestUnitKind_RolePredicate_ContentLinesOnly is tests 48 and 59: the role
// predicate reads only content lines, so a trailing newline and a file of blank
// lines both pass, while an unparseable content line fails.
func TestUnitKind_RolePredicate_ContentLinesOnly(t *testing.T) {
	cases := []struct {
		name    string
		content string // written to the role file; "" means write nothing
		write   bool
		want    bool
	}{
		{name: "row with trailing newline", content: "{\"class\":\"c\"}\n", write: true, want: true},
		{name: "row without trailing newline", content: "{\"class\":\"c\"}", write: true, want: true},
		{name: "two rows and a blank line between", content: "{\"a\":1}\n\n{\"b\":2}\n", write: true, want: true},
		{name: "only blank and whitespace-only lines", content: "\n   \n\t\n\n", write: true, want: true},
		{name: "empty file", content: "", write: true, want: true},
		{name: "unparseable content line", content: "{\"a\":1}\nnot json\n", write: true, want: false},
		{name: "truncated row", content: "{\"a\":1}\n{\"b\":", write: true, want: false},
		{name: "content line that is not an object", content: "[1,2]\n", write: true, want: false},
		{name: "missing file", write: false, want: false},
	}

	for _, kind := range []UnitKind{UnitReviewRole, UnitAuditRole} {
		for _, c := range cases {
			t.Run(string(kind)+"/"+c.name, func(t *testing.T) {
				dir := t.TempDir()
				roundDir := unitRoundDir(dir)
				if c.write {
					writeUnitFile(t, RoleFindingsPath(roundDir, "architect"), c.content)
				}
				got := kind.DurableWrite(UnitTarget{RoundDir: roundDir, ID: "architect"})
				assert.Equal(t, c.want, got)
			})
		}
	}
}

// TestUnitKind_ReviewResolve_ReadsTheMergedFile pins the two edges of the
// resolve predicate: a round with nothing to dispose is complete, and a merged
// file that cannot be read is not.
func TestUnitKind_ReviewResolve_ReadsTheMergedFile(t *testing.T) {
	t.Run("no findings to dispose", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "\n\n")
		assert.True(t, UnitReviewResolve.DurableWrite(UnitTarget{RoundDir: unitRoundDir(dir)}))
	})
	t.Run("merged file unparseable", func(t *testing.T) {
		dir := t.TempDir()
		writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), "{\"resolved\":{}}\nnot json\n")
		assert.False(t, UnitReviewResolve.DurableWrite(UnitTarget{RoundDir: unitRoundDir(dir)}))
	})
}

// TestUnitKind_AuditFix_RowSelector covers the role:item_id key the audit-fix
// predicate selects its row with.
func TestUnitKind_AuditFix_RowSelector(t *testing.T) {
	const rows = "{\"role\":\"go-safety\",\"item_id\":\"item-4\",\"resolved\":{\"status\":\"fixed\"}}\n" +
		"{\"role\":\" ax-contract \",\"item_id\":\"item-9\",\"resolved\":{\"status\":\"duplicate\"}}\n" +
		"{\"role\":\"go-safety\",\"item_id\":\"item-7\"}\n"

	cases := []struct {
		id   string
		want bool
	}{
		{"go-safety:item-4", true},
		// The role is read the way every other audit reader reads it: trimmed.
		{"ax-contract:item-9", true},
		{"go-safety:item-7", false},     // row present, not disposed
		{"go-safety:item-99", false},    // no such row
		{"spec-coverage:item-4", false}, // right item, wrong role
		{"go-safety", false},            // no selector separator
		{"", false},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			dir := t.TempDir()
			writeUnitFile(t, MergedFindingsPath(unitRoundDir(dir)), rows)
			got := UnitAuditFix.DurableWrite(UnitTarget{RoundDir: unitRoundDir(dir), ID: c.id})
			assert.Equal(t, c.want, got)
		})
	}
}

// TestUnitKind_FindingsPaths pins the round-scoped artifact names §3.3 and §6.3
// address by name, so a later unit writing them and this predicate reading them
// cannot drift apart. The .part is the one tp emits into the role's prompt and
// the one §6.3's allowlist grants; dropping the suffix would make the emitted
// name the driver's final name, which a role unit is denied.
func TestUnitKind_FindingsPaths(t *testing.T) {
	assert.Equal(t, filepath.Join("rd", "role-architect.ndjson"), RoleFindingsPath("rd", "architect"))
	assert.Equal(t, filepath.Join("rd", "role-architect.ndjson.part"), RoleFindingsPartPath("rd", "architect"))
	assert.Equal(t, filepath.Join("rd", "merged.ndjson"), MergedFindingsPath("rd"))
}
