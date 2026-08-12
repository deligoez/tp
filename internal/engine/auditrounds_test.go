package engine

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureAuditRoundNotices runs fn with os.Stderr redirected to a pipe and
// returns what output.Notice wrote. The advisories of §2.1 are the observable
// half of "this round contributes no rows", so every no-rows assertion below
// pins the wording; the ones that also read the return value pin the boolean
// alongside it.
func captureAuditRoundNotices(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	fn()
	require.NoError(t, w.Close())
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	return string(data)
}

// auditRoundFixture writes a spec and a recorded audit round file into a fresh
// temp state directory, returning the spec path. The round file is written
// verbatim so a fixture can pin trailing newlines and blank lines.
func auditRoundFixture(t *testing.T, fileName, contents string) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec\n"), 0o600))
	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o750))
	if fileName != "" {
		require.NoError(t, os.WriteFile(filepath.Join(stateDir, fileName), []byte(contents), 0o600))
	}
	return specPath
}

// Test 5 — the row-role predicate. A role is the trimmed `role` string when
// that field is a JSON string non-empty after trimming, and nothing otherwise.
// The two discriminating halves live in the same test on purpose: trimming
// without case folding is one rule, and an implementation that adds
// strings.ToLower "for robustness" passes the trimming half alone.
func TestAuditRowRole_TrimsWithoutCaseFolding(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]any
		want string
	}{
		{"absent key", map[string]any{"status": "PASS"}, ""},
		{"empty string", map[string]any{"role": ""}, ""},
		{"whitespace only", map[string]any{"role": "   "}, ""},
		{"tab and newline only", map[string]any{"role": "\t\n"}, ""},
		{"number", map[string]any{"role": float64(7)}, ""},
		{"boolean", map[string]any{"role": true}, ""},
		{"nil value", map[string]any{"role": nil}, ""},
		{"nil row", nil, ""},
		{"padded id", map[string]any{"role": "  spec-coverage  "}, "spec-coverage"},
		{"plain id", map[string]any{"role": "spec-coverage"}, "spec-coverage"},
		// Never case-folded: corpus ids are lowercase kebab-case filenames, so a
		// differing case is a different id and must stay distinct.
		{"uppercase variant stays distinct", map[string]any{"role": "Spec-Coverage"}, "Spec-Coverage"},
		// The reserved regression id is not special-cased: the audit corpus has
		// no such role, so a row carrying it is a mistake worth surfacing.
		{"reserved regression id is an ordinary role", map[string]any{"role": " regression "}, "regression"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AuditRowRole(tc.row))
		})
	}
}

// Test 6 — the row-PASS predicate: `status` is a JSON string exactly equal to
// PASS, with no trimming and no case folding. The " PASS " case is the
// discriminating one against an implementation that trims `status` by symmetry
// with `role`.
func TestAuditRowIsPass_ByteExact(t *testing.T) {
	cases := []struct {
		name string
		row  map[string]any
		want bool
	}{
		{"exact PASS", map[string]any{"status": "PASS"}, true},
		{"lowercase pass", map[string]any{"status": "pass"}, false},
		{"padded PASS", map[string]any{"status": " PASS "}, false},
		{"number", map[string]any{"status": float64(1)}, false},
		{"absent key", map[string]any{"role": "spec-coverage"}, false},
		{"nil value", map[string]any{"status": nil}, false},
		{"nil row", nil, false},
		{"FAIL", map[string]any{"status": "FAIL"}, false},
		{"PARTIAL", map[string]any{"status": "PARTIAL"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AuditRowIsPass(tc.row))
		})
	}
}

// Test 7 — blank lines are not parse failures. A recorded round file normally
// ends in a newline, so an implementation treating the resulting empty final
// line as unparseable makes every round contribute no rows and the release
// ships dead.
func TestReadAuditRoundRows_BlankLinesAreNotParseFailures(t *testing.T) {
	cases := []struct {
		name     string
		contents string
	}{
		{"trailing newline", `{"role":"spec-coverage","status":"PASS"}` + "\n" +
			`{"role":"go-safety","status":"FAIL"}` + "\n"},
		{"blank line between rows", `{"role":"spec-coverage","status":"PASS"}` + "\n\n" +
			`{"role":"go-safety","status":"FAIL"}` + "\n"},
		{"whitespace-only line between rows", `{"role":"spec-coverage","status":"PASS"}` + "\n   \t\n" +
			`{"role":"go-safety","status":"FAIL"}` + "\n"},
		// A CRLF-terminated results file behaves the same, since a line is blank
		// when it is empty after trimming.
		{"CRLF line endings", `{"role":"spec-coverage","status":"PASS"}` + "\r\n" +
			`{"role":"go-safety","status":"FAIL"}` + "\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := auditRoundFixture(t, "audit-round-1.ndjson", tc.contents)
			entry := &ReviewRound{Round: 1, File: "audit-round-1.ndjson", RolesHash: "sha256:panel"}

			var rows []map[string]any
			var ok bool
			stderr := captureAuditRoundNotices(t, func() {
				rows, ok = ReadAuditRoundRows(specPath, entry, "sha256:panel")
			})

			require.True(t, ok, "the round contributes its rows")
			assert.Len(t, rows, 2)
			assert.Equal(t, "spec-coverage", AuditRowRole(rows[0]))
			assert.True(t, AuditRowIsPass(rows[0]))
			assert.Equal(t, "go-safety", AuditRowRole(rows[1]))
			assert.False(t, AuditRowIsPass(rows[1]))
			assert.Empty(t, stderr, "no advisory fires on a readable round")
		})
	}
}

// Test 8, the discriminating half — Go unmarshals the bare literal `null` into
// a map without error and leaves the map nil, so a `null` line is a row here
// exactly as it is a counted finding on the record path. An implementation
// testing "is a JSON object" rather than "unmarshals into a map" discards a
// round the record path itself accepted.
func TestReadAuditRoundRows_BareNullLineIsARow(t *testing.T) {
	contents := `{"role":"spec-coverage","status":"PASS"}` + "\n" + `null` + "\n"
	specPath := auditRoundFixture(t, "audit-round-1.ndjson", contents)
	entry := &ReviewRound{Round: 1, File: "audit-round-1.ndjson", RolesHash: "sha256:panel"}

	var rows []map[string]any
	var ok bool
	stderr := captureAuditRoundNotices(t, func() {
		rows, ok = ReadAuditRoundRows(specPath, entry, "sha256:panel")
	})

	require.True(t, ok, "the round still contributes rows")
	require.Len(t, rows, 2, "the null line is a row, not a skipped line")
	assert.Nil(t, rows[1], "a null line unmarshals to a nil map")
	assert.Equal(t, "", AuditRowRole(rows[1]), "it carries no role")
	assert.False(t, AuditRowIsPass(rows[1]), "and it is non-PASS")
	assert.Empty(t, stderr)
}

// Test 9 — the four contributes-no-rows causes, each with its own advisory
// wording, emitted exactly once.
func TestReadAuditRoundRows_NoRowsCausesAndAdvisories(t *testing.T) {
	const panel = "sha256:panel"

	cases := []struct {
		name  string
		file  string
		write string
		entry ReviewRound
		// latest is the latest recorded round's stored roles_hash.
		latest string
		want   string
	}{
		{
			name:   "empty file entry",
			entry:  ReviewRound{Round: 2, File: "", RolesHash: panel},
			latest: panel,
			want:   "round 2 has no recorded rows file; skipping its rows",
		},
		{
			name:   "file cannot be read",
			entry:  ReviewRound{Round: 3, File: "audit-round-3.ndjson", RolesHash: panel},
			latest: panel,
			want:   "round 3 file audit-round-3.ndjson is missing; skipping its rows",
		},
		{
			// Test 8's other half. Every non-map literal — a number, a
			// string, a boolean, an array — reaches this one branch: the
			// reader unmarshals into a map[string]any and keys nothing on
			// the literal's type.
			name:   "line does not unmarshal into a map",
			file:   "audit-round-4.ndjson",
			write:  `{"role":"spec-coverage","status":"PASS"}` + "\n[1,2]\n",
			entry:  ReviewRound{Round: 4, File: "audit-round-4.ndjson", RolesHash: panel},
			latest: panel,
			want:   "round 4 file audit-round-4.ndjson has unparseable rows; skipping its rows",
		},
		{
			name:   "stored roles_hash differs",
			file:   "audit-round-5.ndjson",
			write:  `{"role":"spec-coverage","status":"PASS"}` + "\n",
			entry:  ReviewRound{Round: 5, File: "audit-round-5.ndjson", RolesHash: "sha256:other"},
			latest: panel,
			want:   "round 5 was recorded under a different auditor corpus; skipping its rows",
		},
		{
			// The fifth table row for the fourth cause: "this round records no
			// panel" is a different fact from "this round used a different
			// panel", and tp only knows the second.
			name:   "stored roles_hash is empty",
			file:   "audit-round-6.ndjson",
			write:  `{"role":"spec-coverage","status":"PASS"}` + "\n",
			entry:  ReviewRound{Round: 6, File: "audit-round-6.ndjson", RolesHash: ""},
			latest: panel,
			want:   "round 6 has no auditor-corpus hash; skipping its rows",
		},
		{
			// An empty stored hash is not a value that can match anything,
			// including another empty one — so it ends the walk even when the
			// latest round's hash is empty too.
			name:   "empty stored hash does not match an empty latest hash",
			file:   "audit-round-7.ndjson",
			write:  `{"role":"spec-coverage","status":"PASS"}` + "\n",
			entry:  ReviewRound{Round: 7, File: "audit-round-7.ndjson", RolesHash: ""},
			latest: "",
			want:   "round 7 has no auditor-corpus hash; skipping its rows",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := auditRoundFixture(t, tc.file, tc.write)
			entry := tc.entry

			var rows []map[string]any
			var ok bool
			stderr := captureAuditRoundNotices(t, func() {
				rows, ok = ReadAuditRoundRows(specPath, &entry, tc.latest)
			})

			assert.False(t, ok, "the round contributes no rows")
			assert.Nil(t, rows)
			assert.Equal(t, tc.want+"\n", stderr, "exactly one advisory, with the table's wording")
		})
	}
}

// Test 9's "exactly once" half at the row level: a file holding several
// unparseable lines still costs the reader one advisory, not one per line.
func TestReadAuditRoundRows_UnparseableAdvisoryFiresOnce(t *testing.T) {
	contents := "5\n[1]\n\"x\"\n"
	specPath := auditRoundFixture(t, "audit-round-1.ndjson", contents)
	entry := &ReviewRound{Round: 1, File: "audit-round-1.ndjson", RolesHash: "sha256:panel"}

	stderr := captureAuditRoundNotices(t, func() {
		_, _ = ReadAuditRoundRows(specPath, entry, "sha256:panel")
	})

	assert.Equal(t, "round 1 file audit-round-1.ndjson has unparseable rows; skipping its rows\n", stderr)
}

// Test 11 — cause precedence. Over a round that both differs in stored
// roles_hash and has a deleted file, the corpus wording is emitted, not the
// missing-file one. An implementation that reads the file before comparing
// hashes fails this.
func TestReadAuditRoundRows_CorpusWordingWinsOverMissingFile(t *testing.T) {
	specPath := auditRoundFixture(t, "", "")
	entry := &ReviewRound{Round: 2, File: "audit-round-2.ndjson", RolesHash: "sha256:other"}

	var ok bool
	stderr := captureAuditRoundNotices(t, func() {
		_, ok = ReadAuditRoundRows(specPath, entry, "sha256:panel")
	})

	assert.False(t, ok)
	assert.Equal(t, "round 2 was recorded under a different auditor corpus; skipping its rows\n", stderr)
	assert.NotContains(t, stderr, "is missing; skipping its rows")
}

// The empty-hash case is decided before the differing case, and it too outranks
// every file-based cause: a round with no stored hash and no file on disk is
// reported as recording no panel, never as a different panel and never as a
// missing file.
func TestReadAuditRoundRows_EmptyHashWordingWinsOverDifferingAndMissingFile(t *testing.T) {
	specPath := auditRoundFixture(t, "", "")
	entry := &ReviewRound{Round: 2, File: "audit-round-2.ndjson", RolesHash: ""}

	var ok bool
	stderr := captureAuditRoundNotices(t, func() {
		_, ok = ReadAuditRoundRows(specPath, entry, "sha256:panel")
	})

	assert.False(t, ok)
	assert.Equal(t, "round 2 has no auditor-corpus hash; skipping its rows\n", stderr)
}

// The empty `file` entry outranks the unreadable-file cause: reading "" would
// resolve to the state directory itself and report the missing-file wording for
// a round that never named a file. LoadRoundRows cannot tell the two apart,
// which is why §2 reads rounds with its own reader.
func TestReadAuditRoundRows_EmptyFileEntryIsDistinctFromAMissingFile(t *testing.T) {
	specPath := auditRoundFixture(t, "", "")

	empty := &ReviewRound{Round: 1, File: "", RolesHash: "sha256:panel"}
	stderrEmpty := captureAuditRoundNotices(t, func() {
		_, _ = ReadAuditRoundRows(specPath, empty, "sha256:panel")
	})
	assert.Equal(t, "round 1 has no recorded rows file; skipping its rows\n", stderrEmpty)

	// LoadRoundRows reports the same "not found" for both, the limitation §2.1
	// names; this pins that the two states are still distinguishable on disk.
	_, found := LoadRoundRows(specPath, empty)
	assert.False(t, found)
	_, found = LoadRoundRows(specPath, &ReviewRound{Round: 2, File: "audit-round-2.ndjson"})
	assert.False(t, found)
}

// A round with zero rows is not a no-rows round: the file is readable and
// parses, it simply carries nothing. The distinction matters because the
// contributes-no-rows causes end a walk while an empty round does not have to
// be explained by an advisory.
func TestReadAuditRoundRows_EmptyFileContributesZeroRowsNotNoRows(t *testing.T) {
	specPath := auditRoundFixture(t, "audit-round-1.ndjson", "\n\n")
	entry := &ReviewRound{Round: 1, File: "audit-round-1.ndjson", RolesHash: "sha256:panel"}

	var rows []map[string]any
	var ok bool
	stderr := captureAuditRoundNotices(t, func() {
		rows, ok = ReadAuditRoundRows(specPath, entry, "sha256:panel")
	})

	assert.True(t, ok)
	assert.NotNil(t, rows, "an emitted empty slice, never nil")
	assert.Empty(t, rows)
	assert.Empty(t, stderr)
}
