package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundWireRow builds one `ground-round-N.ndjson` line from a base row plus
// this case's own overrides.
//
// A key set to nil is written as JSON `null`; a key in omit is left out
// entirely. The two are different inputs to §7.2 — `unit_id: null` is a
// reader-added claim while a missing `unit_id` is a rejected row — so a fixture
// helper that could not express both would make half this table unwriteable.
func groundWireRow(t *testing.T, base, set map[string]any, omit []string) []byte {
	t.Helper()
	row := make(map[string]any, len(base))
	for k, v := range base {
		row[k] = v
	}
	for k, v := range set {
		row[k] = v
	}
	for _, k := range omit {
		delete(row, k)
	}
	line, err := json.Marshal(row)
	require.NoError(t, err)
	return line
}

// groundClaimRow is a minimal legal row for a verdict that is a claim: the four
// unconditional cells plus the kind/tier/evidence trio §7.2 requires there.
func groundClaimRow() map[string]any {
	return map[string]any{
		"unit_id":  "u7",
		"anchor":   "§7.2",
		"text_sha": "0123456789ab",
		"ordinal":  1,
		"verdict":  "PASS",
		"kind":     "document",
		"tier":     "read",
		"evidence": "read spec/1.0.0.md line 508",
	}
}

// groundNotAClaimRow is a minimal legal row for the one verdict that carries no
// kind, tier or evidence.
func groundNotAClaimRow() map[string]any {
	return map[string]any{
		"unit_id":  "u7",
		"anchor":   "§7.2",
		"text_sha": "0123456789ab",
		"ordinal":  1,
		"verdict":  "NOT-A-CLAIM",
	}
}

// groundRowCase is one row of the rejection table: an input, and the §7.2 field
// the rejection must name.
type groundRowCase struct {
	name  string
	base  map[string]any
	set   map[string]any
	omit  []string
	field string
}

// groundRowRejectionCases is the rejection half of §7.2's table, one case per
// way a cell can fail.
//
// It is a function rather than a literal in one test because two tests read it:
// the one that runs every case, and the one that checks the set of fields these
// cases name against the field list parsed out of §7.2 itself. The second is
// what stops this table quietly falling behind the spec — a cell added to the
// table with no case here, or a case naming a field the table does not have,
// both fail there.
func groundRowRejectionCases() []groundRowCase {
	cases := []groundRowCase{
		{name: "unit_id absent", omit: []string{"unit_id"}, field: "unit_id"},
		{name: "unit_id is not u<N>", set: map[string]any{"unit_id": "3"}, field: "unit_id"},
		{name: "unit_id is u0", set: map[string]any{"unit_id": "u0"}, field: "unit_id"},
		{name: "unit_id is a number", set: map[string]any{"unit_id": 3}, field: "unit_id"},

		{name: "anchor absent", omit: []string{"anchor"}, field: "anchor"},
		{name: "anchor blank", set: map[string]any{"anchor": "   "}, field: "anchor"},
		{name: "anchor carries no section sigil", set: map[string]any{"anchor": "7.2"}, field: "anchor"},
		{name: "anchor is a number", set: map[string]any{"anchor": 7}, field: "anchor"},

		{name: "text_sha absent", omit: []string{"text_sha"}, field: "text_sha"},
		{name: "text_sha upper case", set: map[string]any{"text_sha": "0123456789AB"}, field: "text_sha"},
		{name: "text_sha eleven characters", set: map[string]any{"text_sha": "0123456789a"}, field: "text_sha"},
		{name: "text_sha the whole digest", set: map[string]any{"text_sha": strings.Repeat("ab", 32)}, field: "text_sha"},

		{name: "verdict absent", omit: []string{"verdict"}, field: "verdict"},
		{name: "verdict lower case", set: map[string]any{"verdict": "pass"}, field: "verdict"},

		{name: "kind absent on a claim", omit: []string{"kind"}, field: "kind"},
		{name: "kind outside the seven", set: map[string]any{"kind": "prose"}, field: "kind"},

		{name: "tier absent on a claim", omit: []string{"tier"}, field: "tier"},
		{name: "tier outside the six", set: map[string]any{"tier": "inspect"}, field: "tier"},

		{name: "evidence absent under a tier", omit: []string{"evidence"}, field: "evidence"},
		{name: "evidence blank under a tier", set: map[string]any{"evidence": "  "}, field: "evidence"},
		{
			name:  "evidence present with no tier",
			base:  groundNotAClaimRow(),
			set:   map[string]any{"evidence": "read spec/1.0.0.md line 512"},
			field: "evidence",
		},

		{name: "partial_kind on a PASS row", set: map[string]any{"partial_kind": "two-readings"}, field: "partial_kind"},
		{
			name:  "partial_kind absent on a PARTIAL row",
			set:   map[string]any{"verdict": "PARTIAL"},
			field: "partial_kind",
		},
		{
			name:  "partial_kind outside the three",
			set:   map[string]any{"verdict": "PARTIAL", "partial_kind": "two readings"},
			field: "partial_kind",
		},

		{
			name:  "held_at on a two-readings row",
			set:   map[string]any{"verdict": "PARTIAL", "partial_kind": "two-readings", "held_at": "v0.37.0"},
			field: "held_at",
		},
		{
			name:  "held_at absent on a true-when-written row",
			set:   map[string]any{"verdict": "PARTIAL", "partial_kind": "true-when-written"},
			field: "held_at",
		},
		{
			name:  "held_at blank on a true-when-written row",
			set:   map[string]any{"verdict": "PARTIAL", "partial_kind": "true-when-written", "held_at": ""},
			field: "held_at",
		},

		{name: "causes on a PASS row", set: map[string]any{"causes": groundThreeCauses()}, field: "causes"},
		{
			name:  "causes absent on a QUESTION row",
			set:   map[string]any{"verdict": "QUESTION"},
			field: "causes",
		},
		{
			name:  "causes null on a QUESTION row",
			set:   map[string]any{"verdict": "QUESTION", "causes": nil},
			field: "causes",
		},
		{
			name:  "causes not an array",
			set:   map[string]any{"verdict": "QUESTION", "causes": "three of them"},
			field: "causes",
		},

		{name: "ordinal absent", omit: []string{"ordinal"}, field: "ordinal"},
		{name: "ordinal zero", set: map[string]any{"ordinal": 0}, field: "ordinal"},
		{name: "ordinal negative", set: map[string]any{"ordinal": -1}, field: "ordinal"},
		{name: "ordinal a string", set: map[string]any{"ordinal": "1"}, field: "ordinal"},
		{name: "ordinal fractional", set: map[string]any{"ordinal": 1.5}, field: "ordinal"},

		{name: "carried_from zero", set: map[string]any{"carried_from": 0}, field: "carried_from"},
		{name: "carried_from a string", set: map[string]any{"carried_from": "2"}, field: "carried_from"},

		{name: "note is a number", set: map[string]any{"note": 1}, field: "note"},
	}
	for i := range cases {
		if cases[i].base == nil {
			cases[i].base = groundClaimRow()
		}
	}
	return cases
}

// groundThreeCauses is a legal §6 causes array, used both as a QUESTION row's
// own value and as the thing a non-QUESTION row must not carry.
func groundThreeCauses() []map[string]any {
	return []map[string]any{
		{"cause": "the scan is fence-blind", "prediction": "fencing the scan moves the count"},
		{"cause": "the anchor is resolved by text search", "prediction": "carrying a line number moves it"},
		{"cause": "the arms cut the unit", "prediction": "the index shows it as cut"},
	}
}

// TestAMinimalLegalRowParsesAndReadsBack pins the fixtures every rejection case
// is a one-cell edit of. Without it a rejection table proves only that the base
// is broken somewhere, which is the shape a whole table can pass under.
func TestAMinimalLegalRowParsesAndReadsBack(t *testing.T) {
	row, err := ParseGroundRow(groundWireRow(t, groundClaimRow(), nil, nil))
	require.NoError(t, err)

	require.NotNil(t, row.UnitID)
	assert.Equal(t, "u7", *row.UnitID)
	assert.Equal(t, "§7.2", row.Anchor)
	assert.Equal(t, "0123456789ab", row.TextSHA)
	assert.Equal(t, VerdictPass, row.Verdict)
	assert.Equal(t, KindDocument, row.Kind)
	assert.Equal(t, TierRead, row.Tier)
	assert.Equal(t, "read spec/1.0.0.md line 508", row.Evidence)
	assert.Equal(t, 1, row.Ordinal)
	assert.Empty(t, row.PartialKind)
	assert.Empty(t, row.HeldAt)
	assert.Nil(t, row.Causes)
	assert.Zero(t, row.CarriedFrom)
	assert.Empty(t, row.Note)

	_, err = ParseGroundRow(groundWireRow(t, groundNotAClaimRow(), nil, nil))
	require.NoError(t, err, "a NOT-A-CLAIM row carries neither kind, tier nor evidence")
}

// TestEveryCellOfTheRowTableRejectsNamingItsField runs §7.2's rejection table.
//
// Each case asserts the FIELD off the typed error rather than a substring of
// its sentence: §7.2 requires the rejection to name the field, and a
// `Contains` over the message is a local assertion inside an unbounded string
// that any rewording satisfies or breaks by accident.
func TestEveryCellOfTheRowTableRejectsNamingItsField(t *testing.T) {
	for _, tc := range groundRowRejectionCases() {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseGroundRow(groundWireRow(t, tc.base, tc.set, tc.omit))
			require.Error(t, err)
			var rowErr *GroundRowError
			require.ErrorAs(t, err, &rowErr, "a cell failure is rejected as a GroundRowError")
			assert.Equal(t, tc.field, rowErr.Field)
		})
	}
}

// TestTheRowsTheTableAllows is the other half of every conditional cell.
//
// A validator that rejects everything passes the rejection table entire, so
// each of these is the input its neighbouring rejection case differs from by
// one cell — a NOT-A-CLAIM row that DOES carry kind, tier and evidence, a
// PARTIAL row without `held_at`, a QUESTION row with `causes`.
func TestTheRowsTheTableAllows(t *testing.T) {
	cases := []struct {
		name   string
		base   map[string]any
		set    map[string]any
		omit   []string
		assert func(t *testing.T, row GroundRow)
	}{
		{
			// §7.2's bold carve-out: the first run had five units stating a
			// decision AND a checked fact, and had to bury the evidence in
			// `note`. On a NOT-A-CLAIM row the trio is optional, not forbidden.
			name: "NOT-A-CLAIM carrying a kind, a tier and evidence",
			base: groundNotAClaimRow(),
			set: map[string]any{
				"kind":     "corpus",
				"tier":     "query",
				"evidence": "grep -c over spec/*.md returned 54",
			},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, KindCorpus, row.Kind)
				assert.Equal(t, TierQuery, row.Tier)
			},
		},
		{
			// §7.2 says kind and tier are optional on NOT-A-CLAIM and says
			// nothing about carrying one without the other, so the literal
			// reading of "optional" is what ships. Recorded here so the
			// undecided case has a stated answer rather than an accidental one.
			name: "NOT-A-CLAIM carrying a kind and no tier",
			base: groundNotAClaimRow(),
			set:  map[string]any{"kind": "document"},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, KindDocument, row.Kind)
				assert.Empty(t, row.Tier)
				assert.Empty(t, row.Evidence)
			},
		},
		{
			name: "PARTIAL two-readings without held_at",
			set:  map[string]any{"verdict": "PARTIAL", "partial_kind": "two-readings"},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, PartialTwoReadings, row.PartialKind)
				assert.Empty(t, row.HeldAt)
			},
		},
		{
			name: "PARTIAL true-when-written with held_at",
			set: map[string]any{
				"verdict": "PARTIAL", "partial_kind": "true-when-written", "held_at": "v0.36.0",
			},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, PartialTrueWhenWritten, row.PartialKind)
				assert.Equal(t, "v0.36.0", row.HeldAt)
			},
		},
		{
			name: "QUESTION with three causes",
			set:  map[string]any{"verdict": "QUESTION", "causes": groundThreeCauses()},
			assert: func(t *testing.T, row GroundRow) {
				require.Len(t, row.Causes, 3)
				assert.Equal(t, "the scan is fence-blind", row.Causes[0].Cause)
				assert.Equal(t, "fencing the scan moves the count", row.Causes[0].Prediction)
			},
		},
		{
			// §3: UNVERIFIABLE carries the deepest tier attempted, and §7.2's
			// per-verdict rule puts no acceptability constraint on it. Nothing
			// about `causes` either.
			name: "UNVERIFIABLE with a tier and no causes",
			set:  map[string]any{"verdict": "UNVERIFIABLE", "kind": "mechanism", "tier": "read"},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, VerdictUnverifiable, row.Verdict)
				assert.Nil(t, row.Causes)
			},
		},
		{
			name: "a reader-added claim writes unit_id null",
			set:  map[string]any{"unit_id": nil},
			assert: func(t *testing.T, row GroundRow) {
				assert.Nil(t, row.UnitID, "null is how §8 tells a reader-added row from a floor unit")
			},
		},
		{
			name: "a carried-forward disposition names the round it came from",
			set:  map[string]any{"carried_from": 2},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, 2, row.CarriedFrom)
			},
		},
		{
			// `note` is the one string cell §7.2 leaves free, so an empty one
			// is not a failed cell the way an empty `evidence` is.
			name: "an empty note",
			set:  map[string]any{"note": ""},
			assert: func(t *testing.T, row GroundRow) {
				assert.Empty(t, row.Note)
			},
		},
		{
			name: "a unit before the first heading anchors to §0",
			set:  map[string]any{"anchor": "§0"},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, "§0", row.Anchor)
			},
		},
		{
			name: "a deep anchor",
			set:  map[string]any{"anchor": "§4.2.1"},
			assert: func(t *testing.T, row GroundRow) {
				assert.Equal(t, "§4.2.1", row.Anchor)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base := tc.base
			if base == nil {
				base = groundClaimRow()
			}
			row, err := ParseGroundRow(groundWireRow(t, base, tc.set, tc.omit))
			require.NoError(t, err)
			tc.assert(t, row)
		})
	}
}

// TestALineThatIsNotAJSONObjectIsRejected covers the one rejection that names
// no field, because no cell is reached: §7.1's exit 1 lists "a line that is not
// JSON (a trailing partial line included)" beside the cell failures.
func TestALineThatIsNotAJSONObjectIsRejected(t *testing.T) {
	for _, line := range []string{"", "{", "null", "[]", `"a row"`, "12"} {
		_, err := ParseGroundRow([]byte(line))
		require.Error(t, err, "%q is not a row", line)
		var rowErr *GroundRowError
		assert.NotErrorAs(t, err, &rowErr,
			"a line that is not an object fails before any cell, so there is no field to name")
	}
}

// TestEveryFieldSection72NamesHasARejectionCase binds this type to the artifact
// it is derived from.
//
// The field list is parsed out of §7.2's own table rather than restated here,
// and the match is asserted in both directions. A cell added to the spec with
// no case in this file fails on the left; a field name misspelled in the
// implementation — which would then be the Field of a rejection nobody can act
// on — fails on the right.
func TestEveryFieldSection72NamesHasARejectionCase(t *testing.T) {
	inSpec := groundSection72Fields(t)
	require.Len(t, inSpec, 13, "§7.2's table has thirteen field rows")

	covered := make([]string, 0, len(inSpec))
	for _, tc := range groundRowRejectionCases() {
		if !slices.Contains(covered, tc.field) {
			covered = append(covered, tc.field)
		}
	}

	assert.ElementsMatch(t, inSpec, covered,
		"every field §7.2 names must be a cell this type rejects on, and no case may name a field §7.2 does not have")
}

// groundSection72Fields reads the field column of §7.2's table out of the spec.
//
// It stops at the first line after the table that is not a row, which is what
// keeps the per-verdict table further down §7.2 — whose first column holds
// `PASS` and `QUESTION` — out of the result.
func groundSection72Fields(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "spec", "1.0.0.md"))
	require.NoError(t, err, "this type is §7.2's table; the spec must be readable for that to be checkable")

	lines := strings.Split(string(data), "\n")
	i := slices.Index(lines, "### 7.2 The row")
	require.GreaterOrEqual(t, i, 0, "§7.2 must be findable by its heading")

	for i < len(lines) && !strings.HasPrefix(lines[i], "|") {
		i++
	}
	require.Less(t, i, len(lines), "§7.2 must carry a table")

	cell := regexp.MustCompile("^\\|\\s*`([a-z][a-z_]*)`\\s*\\|")
	fields := make([]string, 0, 16)
	for ; i < len(lines) && strings.HasPrefix(lines[i], "|"); i++ {
		if m := cell.FindStringSubmatch(lines[i]); m != nil {
			fields = append(fields, m[1])
		}
	}
	return fields
}

// TestARowCarryingWhatTheFloorEmittedParses holds the three shape rules
// accountable to the code that produces the values, not to this file's literals.
//
// `unit_id`, `anchor` and `text_sha` are each pinned to a pattern here, and a
// pattern is only as good as its agreement with the emitter: if FloorIndexRows
// stopped writing `u<N>`, or FloorAnchorOf stopped writing `§0` before the
// first heading, hand-written fixtures would go on passing while no real row
// could be recorded. The fixture carries a unit before its first heading and a
// unit under one, so both anchor forms travel.
func TestARowCarryingWhatTheFloorEmittedParses(t *testing.T) {
	text := "A preamble sentence naming 12 rounds.\n\n## 7. The record\n\nThe round is recorded in 3 steps.\n"
	rows := FloorIndexRows(text, FloorAnchorOf(text))
	require.NotEmpty(t, rows)

	anchors := make([]string, 0, len(rows))
	graded := 0
	for _, r := range rows {
		if r.TextSHA == "" {
			continue // a unit the arms cut carries no hash and no obligation
		}
		graded++
		anchors = append(anchors, r.Anchor)

		row, err := ParseGroundRow(groundWireRow(t, groundClaimRow(), map[string]any{
			"unit_id":  r.ID,
			"anchor":   r.Anchor,
			"text_sha": r.TextSHA,
			"ordinal":  r.Ordinal,
		}, nil))
		require.NoError(t, err, "the emitted floor row %+v must be recordable", r)
		require.NotNil(t, row.UnitID)
		assert.Equal(t, r.ID, *row.UnitID)
		assert.Equal(t, r.Anchor, row.Anchor)
		assert.Equal(t, r.TextSHA, row.TextSHA)
		assert.Equal(t, r.Ordinal, row.Ordinal)
	}

	require.GreaterOrEqual(t, graded, 2, "the fixture must reach the floor for this to assert anything")
	assert.Contains(t, anchors, "§0", "the fixture must carry a unit before the first heading")
	assert.Contains(t, anchors, "§7", "and one under a numbered heading")
}

// TestThePartialKindEnumIsThreeValues pins §7.2's third enum the way §3's six
// and §4.1's seven and six are pinned: closed, round-tripping, and rejecting a
// near-miss rather than folding it.
func TestThePartialKindEnumIsThreeValues(t *testing.T) {
	for _, wire := range []string{"two-readings", "reason-not-conclusion", "true-when-written"} {
		v, ok := ParseGroundPartialKind(wire)
		require.True(t, ok, "%q is one of §7.2's three partial_kind values", wire)
		assert.Equal(t, wire, string(v))
	}
	assert.Len(t, GroundPartialKinds(), 3)

	for _, s := range []string{"", "two readings", "Two-Readings", "true_when_written", "stale"} {
		v, ok := ParseGroundPartialKind(s)
		assert.False(t, ok, "partial_kind %q is outside §7.2's three", s)
		assert.Empty(t, string(v))
	}
}
