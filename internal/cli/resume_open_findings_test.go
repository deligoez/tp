package cli_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeOneRecordedRound writes a state holding exactly one recorded round of
// the given phase ("review" or "audit"), stamped with the current spec hash,
// whose round file carries rows verbatim.
func writeOneRecordedRound(t *testing.T, dir, phase string, clean bool, rows ...string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	require.NoError(t, err)
	hash := fmt.Sprintf("sha256:%x", sha256.Sum256(data))
	file := phase + "-round-1.ndjson"
	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, file), []byte(strings.Join(rows, "\n")+"\n"), 0o600))
	round := []map[string]any{{"round": 1, "clean": clean, "spec_hash": hash, "file": file, "findings": len(rows)}}
	st := map[string]any{"spec": "spec.md", "review_rounds": []map[string]any{}, "audit_rounds": []map[string]any{}}
	st[phase+"_rounds"] = round
	out, err := json.Marshal(st)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), out, 0o600))
}

// TestResume_CleanAuditRoundHasNoUnresolvedFindings: a PASS row is not a
// finding, so a round of three PASS rows leaves nothing unresolved. resume
// used to count rows, and reported 3.
func TestResume_CleanAuditRoundHasNoUnresolvedFindings(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeOneRecordedRound(t, dir, "audit", true,
		`{"id":"a-1","role":"spec-coverage","status":"PASS","item":"one"}`,
		`{"id":"a-2","role":"spec-coverage","status":"PASS","item":"two"}`,
		`{"id":"a-3","role":"go-safety","status":"PASS","item":"three"}`,
	)
	res := resumeResult(t, dir)
	require.Equal(t, "audit", res["phase"])
	_, payload := nextAction(res)
	assert.Equal(t, float64(2), payload["round"])
	assert.Equal(t, float64(0), payload["unresolved_findings"], "three PASS rows are no open finding")
}

// TestResume_AuditOpenFindingsCountOnlyUndisposedNonPass: the same predicate
// on the audit side — a FAIL row is open, a PASS row is not a finding, and a
// non-PASS row accepted duplicate with evidence is cleared.
func TestResume_AuditOpenFindingsCountOnlyUndisposedNonPass(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeOneRecordedRound(t, dir, "audit", false,
		`{"id":"a-1","role":"spec-coverage","status":"PASS","item":"one"}`,
		`{"id":"a-2","role":"spec-coverage","status":"FAIL","item":"two"}`,
		`{"id":"a-3","role":"go-safety","status":"PARTIAL","item":"three","resolved":{"status":"duplicate","evidence":"same as a-2","resolved_at":"2026-09-11T00:00:00Z"}}`,
	)
	_, payload := nextAction(resumeResult(t, dir))
	assert.Equal(t, float64(1), payload["unresolved_findings"])
}

// TestReviewOpenFindingAgreesAcrossStatusAndResume: `tp review --status` and
// `tp resume` read one predicate for whether a review finding is still open.
// A duplicate with evidence used to read as cleared on --status (the round is
// clean) and as unresolved on resume; a wontfix with blank evidence the other
// way round. A fixed finding keeps its round unclean until a later round
// re-reads it, so both surfaces keep counting it.
func TestReviewOpenFindingAgreesAcrossStatusAndResume(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		resolved string
		open     bool
	}{
		{"duplicate with evidence", `{"status":"duplicate","evidence":"same as f-0","resolved_at":"2026-09-11T00:00:00Z"}`, false},
		{"wontfix with evidence", `{"status":"wontfix","evidence":"out of scope","resolved_at":"2026-09-11T00:00:00Z"}`, false},
		{"wontfix with blank evidence", `{"status":"wontfix","evidence":"  ","resolved_at":"2026-09-11T00:00:00Z"}`, true},
		{"fixed", `{"status":"fixed","evidence":"repaired","resolved_at":"2026-09-11T00:00:00Z"}`, true},
		{"no disposition", ``, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := newPayloadRepo(t, `[]`)
			row := `{"id":"f-1","severity":"high","location":"§1","class":"ambiguity","finding":"x"`
			if tc.resolved != "" {
				row += `,"resolved":` + tc.resolved
			}
			row += `}`
			writeOneRecordedRound(t, dir, "review", false, row)

			out, stderr, code := runTP(t, dir, "review", "spec.md", "--status")
			require.Equal(t, 0, code, "status failed: %s", stderr)
			var status map[string]any
			require.NoError(t, json.Unmarshal([]byte(out), &status))
			rounds := status["review_rounds"].([]any)
			require.Len(t, rounds, 1)
			statusClean := rounds[0].(map[string]any)["clean"].(bool)

			res := resumeResult(t, dir)
			require.Equal(t, "review", res["phase"])
			_, payload := nextAction(res)

			want := float64(0)
			if tc.open {
				want = 1
			}
			assert.Equal(t, !tc.open, statusClean, "--status reads the round's only finding as open=%v", tc.open)
			assert.Equal(t, want, payload["unresolved_findings"], "resume reads the round's only finding as open=%v", tc.open)
		})
	}
}
