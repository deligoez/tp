package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateParityCase is one value for a required key, and what §2's predicate must
// make of it. The value is a Go value rather than a fragment of JSON text: the
// row is marshalled below, so every line is valid JSON by construction. A
// hand-written table did not have that property — a raw control byte is not
// legal inside a JSON string, and the first one made `--record` abort as
// invalid JSON, which is a different refusal from the one under test and hid
// every row behind it.
type gateParityCase struct {
	name    string
	value   any
	missing bool
}

// gateParityTable is the input both gates are driven from. Its three groups are
// what make the verdict mean something:
//
//   - Ten whitespace codepoints, written as numbers so nothing depends on an
//     escape surviving an editor. Each satisfies unicode.IsSpace and so trims
//     away to nothing: a value that is present, is a string and is not empty
//     must still count as missing.
//   - The five non-string JSON types. missingFindingFields reads the key
//     through a string type assertion, so each of these arrives as "" — the
//     same verdict by a different route.
//   - Three legal controls, without which a gate that called everything
//     missing would satisfy every other assertion here. The third is the sharp
//     one: U+200B is zero-width and reads as blank, but unicode.IsSpace is
//     false for it, so it is a legal value and both gates must accept it. A
//     gate that rejected "anything that looks blank" goes red on that row
//     alone.
var gateParityTable = []gateParityCase{
	{"ws-tab-0009", string(rune(0x0009)), true},
	{"ws-newline-000a", string(rune(0x000A)), true},
	{"ws-vertical-tab-000b", string(rune(0x000B)), true},
	{"ws-form-feed-000c", string(rune(0x000C)), true},
	{"ws-carriage-return-000d", string(rune(0x000D)), true},
	{"ws-space-0020", string(rune(0x0020)), true},
	{"ws-next-line-0085", string(rune(0x0085)), true},
	{"ws-no-break-space-00a0", string(rune(0x00A0)), true},
	{"ws-em-space-2003", string(rune(0x2003)), true},
	{"ws-ideographic-space-3000", string(rune(0x3000)), true},

	{"type-number", 42, true},
	{"type-boolean", true, true},
	{"type-null", nil, true},
	{"type-object", map[string]any{"section": "sec-obj"}, true},
	{"type-array", []any{"sec-arr"}, true},

	{"control-plain", "sec-1", false},
	{"control-padded-but-not-empty", "  sec-2  ", false},
	{"control-zero-width-space-200b", "sec-3" + string(rune(0x200B)), false},
}

// TestReviewGates_MergeAndRecordAgreeOnEveryValue drives `--merge` and
// `--record` from one table and asserts they classify every row the same way.
//
// Parity is the claim, and it is not free even though both gates call
// missingFindingFields today: what a gate does with the predicate's answer is
// its own, and the two answered differently in the shipped tree once already —
// `--merge` on `x == ""` against `--record` on `strings.TrimSpace(x) == ""`,
// under which every whitespace row here merges through and is refused at
// record. That divergence is what this test exists to keep from coming back.
//
// Neither verdict is read out of matched text. The merge gate's is derived from
// which rows reached the merged output, by the `finding` each row carries as
// its own name; the record gate's is the set of line numbers its refusal names.
// Both are compared against the table and against each other, so a failure says
// which row and which gate.
func TestReviewGates_MergeAndRecordAgreeOnEveryValue(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))

	lines := make([]string, 0, len(gateParityTable))
	wantMissing := make([]int, 0, len(gateParityTable))
	wantLegal := make([]string, 0, len(gateParityTable))
	for i, c := range gateParityTable {
		// The fixture's own property, asserted rather than assumed: a case
		// marked missing because of whitespace must really be a string that
		// trims to nothing, and a legal string must really not. Without this
		// the table could claim a codepoint is space when it is not, and the
		// gates would agree with each other about the wrong answer.
		if s, ok := c.value.(string); ok {
			assert.Equal(t, c.missing, strings.TrimSpace(s) == "",
				"%s: the table's own verdict must match what TrimSpace does to it", c.name)
		}

		// `location` carries the case's value; `finding` carries its name, so a
		// row that survives the merge can be named. Every other required key is
		// legal on every row.
		//
		// `class` carries the name too, and that is load-bearing rather than
		// decorative: --merge clusters by (location, class) and keeps one
		// representative per cluster, so with a shared class every row whose
		// location normalises to the same key collapses into one. Measured
		// under the mutant below with `class` fixed at "gap": nine of the ten
		// whitespace rows vanished from the output through clustering, not
		// through the gate, and the merge side would have been reading the
		// wrong mechanism. A unique class makes every row its own cluster, so
		// absence from the output means the gate skipped it and nothing else.
		row, err := json.Marshal(map[string]any{
			"role":     "implementer",
			"severity": "high",
			"class":    c.name,
			"location": c.value,
			"finding":  c.name,
			"evidence": "read the cited section",
		})
		require.NoError(t, err)
		lines = append(lines, string(row))

		if c.missing {
			// The record gate reports 1-based line numbers and the rows are
			// written in table order, so the index is the line number.
			wantMissing = append(wantMissing, i+1)
		} else {
			wantLegal = append(wantLegal, c.name)
		}
	}
	// The fixture must actually be mixed: an all-missing table is satisfied by a
	// gate that refuses everything, an all-legal one by a gate that refuses
	// nothing.
	require.NotEmpty(t, wantMissing, "the table must carry rows both gates reject")
	require.NotEmpty(t, wantLegal, "the table must carry rows both gates accept")

	// --- the merge gate ------------------------------------------------------
	f := writeFindingsFile(t, dir, "role.ndjson", lines)
	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f)
	require.Equal(t, 0, code,
		"the legal rows keep the input from being dropped, so a skip is a warning here: %s", stderr)

	var merged struct {
		Findings []map[string]any `json:"findings"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &merged), "merge summary must be JSON: %s", stdout)
	survived := make(map[string]bool, len(merged.Findings))
	for _, row := range merged.Findings {
		name, _ := row["finding"].(string)
		survived[name] = true
	}
	mergeMissing := make([]int, 0, len(gateParityTable))
	mergeLegal := make([]string, 0, len(gateParityTable))
	for i, c := range gateParityTable {
		if survived[c.name] {
			mergeLegal = append(mergeLegal, c.name)
			continue
		}
		mergeMissing = append(mergeMissing, i+1)
	}

	// --- the record gate -----------------------------------------------------
	_, stderr, code = recordRound(t, dir, strings.Join(lines, "\n")+"\n")
	require.Equal(t, 1, code, "--record refuses a row missing a required field: %s", stderr)
	recordMissing := linesNamed(t, stderr)

	// --- the three claims ----------------------------------------------------
	assert.ElementsMatch(t, wantMissing, mergeMissing,
		"--merge skips exactly the rows whose location trims to nothing or is not a string")
	assert.ElementsMatch(t, wantLegal, mergeLegal,
		"--merge emits exactly the legal rows, U+200B among them")
	assert.ElementsMatch(t, wantMissing, recordMissing,
		"--record names exactly the same rows by line number")
	assert.ElementsMatch(t, mergeMissing, recordMissing,
		"the two gates disagree about no value in the table")
}
