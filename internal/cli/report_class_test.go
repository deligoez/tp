package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func classRow(location, finding, class string) string {
	row := fmt.Sprintf(`{"severity":"low","category":"consistency","location":%q,"finding":%q,"suggestion":"fix"`, location, finding)
	if class != "" {
		row += fmt.Sprintf(`,"class":%q`, class)
	}
	return row + "}"
}

func TestReport_ByClassAndCandidates(t *testing.T) {
	dir := t.TempDir()

	// Round 1: two-rounds-class appears once; five-times-class appears 5x;
	// once-only-class appears once; one row with no class.
	r1Rows := []string{
		classRow("L1", "finding r1 a", "two-rounds-class"),
		classRow("L2", "finding r1 b1", "five-times-class"),
		classRow("L3", "finding r1 b2", "five-times-class"),
		classRow("L4", "finding r1 b3", "five-times-class"),
		classRow("L5", "finding r1 b4", "five-times-class"),
		classRow("L6", "finding r1 b5", "five-times-class"),
		classRow("L7", "finding r1 c", "once-only-class"),
		classRow("L8", "finding r1 d", ""),
	}
	// Round 2: two-rounds-class appears again (second distinct round).
	r2Rows := []string{
		classRow("L9", "finding r2 a", "two-rounds-class"),
	}

	r1 := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(r1, []byte(strings.Join(r1Rows, "\n")+"\n"), 0o600))
	r2 := filepath.Join(dir, "r2.ndjson")
	require.NoError(t, os.WriteFile(r2, []byte(strings.Join(r2Rows, "\n")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2)
	require.Equal(t, 0, code, "report failed: %s", stderr)

	var result struct {
		ByClass             map[string]int `json:"by_class"`
		MechanizeCandidates []struct {
			Class      string `json:"class"`
			RoundsSeen int    `json:"rounds_seen"`
			Total      int    `json:"total"`
		} `json:"mechanize_candidates"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// by_class groups only rows carrying a class
	assert.Equal(t, 2, result.ByClass["two-rounds-class"])
	assert.Equal(t, 5, result.ByClass["five-times-class"])
	assert.Equal(t, 1, result.ByClass["once-only-class"])
	assert.NotContains(t, result.ByClass, "")

	// Candidates: five-times-class (>=5 in one round), two-rounds-class
	// (>=2 distinct rounds); once-only-class is not a candidate.
	require.Len(t, result.MechanizeCandidates, 2)
	assert.Equal(t, "five-times-class", result.MechanizeCandidates[0].Class, "sorted by total descending")
	assert.Equal(t, 1, result.MechanizeCandidates[0].RoundsSeen)
	assert.Equal(t, 5, result.MechanizeCandidates[0].Total)
	assert.Equal(t, "two-rounds-class", result.MechanizeCandidates[1].Class)
	assert.Equal(t, 2, result.MechanizeCandidates[1].RoundsSeen)
	assert.Equal(t, 2, result.MechanizeCandidates[1].Total)
}

func TestReport_MechanizeCandidates_TieBreakAlphabetical(t *testing.T) {
	dir := t.TempDir()

	// Both classes appear in 2 rounds with total 2 — tie broken by class name.
	r1 := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(r1, []byte(
		classRow("L1", "f1", "zeta-class")+"\n"+classRow("L2", "f2", "alpha-class")+"\n"), 0o600))
	r2 := filepath.Join(dir, "r2.ndjson")
	require.NoError(t, os.WriteFile(r2, []byte(
		classRow("L3", "f3", "zeta-class")+"\n"+classRow("L4", "f4", "alpha-class")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2)
	require.Equal(t, 0, code, "report failed: %s", stderr)

	var result struct {
		MechanizeCandidates []struct {
			Class string `json:"class"`
			Total int    `json:"total"`
		} `json:"mechanize_candidates"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.Len(t, result.MechanizeCandidates, 2)
	assert.Equal(t, "alpha-class", result.MechanizeCandidates[0].Class)
	assert.Equal(t, "zeta-class", result.MechanizeCandidates[1].Class)
}

// TestReport_MechanizeRetainedNoFoundByThreshold guards §8.6: mechanize_candidates
// stays the cross-round class-frequency signal, and no found_by (multi-role)
// mechanize rule or threshold field is added — the per-role overlap report already
// surfaces repeated overlap.
func TestReport_MechanizeRetainedNoFoundByThreshold(t *testing.T) {
	dir := t.TempDir()

	// Round 1: overlap-class is found by two roles at one (location, class) —
	// found_by would be 2 after clustering — and two-rounds-class appears once.
	r1 := filepath.Join(dir, "r1.ndjson")
	require.NoError(t, os.WriteFile(r1, []byte(
		`{"severity":"low","role":"implementer","class":"overlap-class","location":"§5","finding":"a"}`+"\n"+
			`{"severity":"low","role":"tester","class":"overlap-class","location":"§5 more","finding":"b"}`+"\n"+
			`{"severity":"low","role":"implementer","class":"two-rounds-class","location":"§6","finding":"c"}`+"\n"), 0o600))
	// Round 2: two-rounds-class recurs -> a candidate purely by round frequency.
	r2 := filepath.Join(dir, "r2.ndjson")
	require.NoError(t, os.WriteFile(r2, []byte(
		`{"severity":"low","role":"implementer","class":"two-rounds-class","location":"§7","finding":"d"}`+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "review", "--report", r1, r2)
	require.Equal(t, 0, code, "report failed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	candidates, _ := result["mechanize_candidates"].([]any)

	// Only the cross-round class is a candidate; a found_by>=2 class seen in one
	// round never is (§8.6 adds no found_by mechanize rule).
	require.Len(t, candidates, 1, "found_by>=2 overlap-class is not mechanized from a single round")
	entry := candidates[0].(map[string]any)
	assert.Equal(t, "two-rounds-class", entry["class"])
	_, hasFoundBy := entry["found_by"]
	assert.False(t, hasFoundBy, "no found_by threshold field was added to mechanize candidates")
}
