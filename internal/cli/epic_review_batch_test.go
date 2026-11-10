package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicThreeMilestones is spec/1.7.0.md §5's fixture: tracks [[A, B], [L1]],
// a and b in A, c in B, d in L1.
const epicThreeMilestones = `{
  "version": 1,
  "milestones": [["A", "B"], ["L1"]],
  "epics": [
    {"id": "a", "title": "A", "milestone": "A"},
    {"id": "b", "title": "B", "milestone": "A", "depends_on": [{"id": "a", "why": "b needs a"}]},
    {"id": "c", "title": "C", "milestone": "B", "depends_on": [{"id": "b", "why": "c needs b"}]},
    {"id": "d", "title": "D", "milestone": "L1"}
  ]
}
`

// epicSliceKeyPayload is a slice key as the batch payloads print it.
type epicSliceKeyPayload struct {
	Kind  string   `json:"kind"`
	Value string   `json:"value"`
	IDs   []string `json:"ids"`
	Name  string   `json:"name"`
}

// epicBatchPayload is the part of a batch emission these tests read.
type epicBatchPayload struct {
	Slices  []epicReviewPayload `json:"slices"`
	Skipped []struct {
		Key    epicSliceKeyPayload `json:"key"`
		Reason string              `json:"reason"`
	} `json:"skipped"`
}

// runEpicBatch runs tp epic review with args and decodes its batch payload on
// exit 0.
func runEpicBatch(t *testing.T, dir string, args ...string) epicBatchPayload {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out epicBatchPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// epicBatchFixture returns the three-milestone project with review_max_rounds
// 1 and review_clean_rounds 1, where B recorded an open high finding (at its
// cap, not converged) and L1 recorded a clean round (done).
func epicBatchFixture(t *testing.T) string {
	t.Helper()
	dir := epicGraphDir(t, epicThreeMilestones)
	writeEpicProjectConfig(t, dir, `{"review_max_rounds": 1, "review_clean_rounds": 1}`)
	runEpicReview(t, dir, "--milestone", "B")
	recordEpicRound(t, dir, `{"severity":"high","finding":"c names no checker","location":"epic:c","evidence":"c"}`+"\n", "--milestone", "B")
	runEpicReview(t, dir, "--milestone", "L1")
	recordEpicRound(t, dir, "", "--milestone", "L1")
	return dir
}

// epicSkippedReasons maps each skipped milestone to its reason.
func epicSkippedReasons(p *epicBatchPayload) map[string]string {
	reasons := map[string]string{}
	for _, s := range p.Skipped {
		reasons[s.Key.Value] = s.Reason
	}
	return reasons
}

// TestEpicReviewBatch_AllEmitsEveryMilestoneAndSkipsWhatCannotEmit is §5's
// emission: --all emits each tracked milestone's round in one payload, and a
// slice at its cap (B) or done (L1) is listed under skipped with its reason
// while A still emits. Emitting only the first milestone, or refusing the
// whole call because one slice is capped, fails it.
func TestEpicReviewBatch_AllEmitsEveryMilestoneAndSkipsWhatCannotEmit(t *testing.T) {
	t.Parallel()
	dir := epicBatchFixture(t)

	out := runEpicBatch(t, dir, "--all")
	require.Len(t, out.Slices, 1, "only A can emit")
	assert.Equal(t, 1, out.Slices[0].Round)
	assert.Equal(t, []string{"epic:a", "epic:b", "gate:A"}, out.Slices[0].Units)
	assert.Equal(t, []string{"dod-satisfiability", "why-auditor", "phase-economist"}, promptRoles(&out.Slices[0]))
	assert.Contains(t, out.Slices[0].NextAction, "tp epic review --milestone A --record")
	reasons := epicSkippedReasons(&out)
	require.Len(t, reasons, 2, "skipped: %v", out.Skipped)
	assert.Contains(t, reasons["B"], "round budget exhausted")
	assert.Contains(t, reasons["L1"], "done")
	for _, s := range out.Skipped {
		assert.Equal(t, "milestone", s.Key.Kind)
	}
	_, err := os.Stat(filepath.Join(dir, out.Slices[0].StateDir, "snapshot-round-1.json"))
	require.NoError(t, err, "the emitted slice wrote its round's snapshot")
}

// TestEpicReviewBatch_MilestonesSkipAnUnknownOneAndApplyRoleToEach is §5's
// --milestones with --role: the listed milestones emit, a milestone no track
// lists is skipped with the reason a one-slice call exits with, and --role
// keeps the one role in every slice. A refusal of the whole list for Z, or a
// slice carrying every role, fails it.
func TestEpicReviewBatch_MilestonesSkipAnUnknownOneAndApplyRoleToEach(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicThreeMilestones)

	out := runEpicBatch(t, dir, "--milestones", "A,Z,B", "--role", "why-auditor")
	require.Len(t, out.Slices, 2)
	for _, s := range out.Slices {
		assert.Equal(t, []string{"why-auditor"}, promptRoles(&s), "--role applies to each slice")
	}
	assert.Contains(t, out.Slices[0].Units, "epic:a")
	assert.Contains(t, out.Slices[1].Units, "epic:c")
	reasons := epicSkippedReasons(&out)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons["Z"], `milestone "Z" is in no track`)
}

// TestEpicReviewBatch_AnUnknownRoleWritesNoSlice is §5's preparation before
// writes as far as a call can observe it: a --role no slice resolves exits 2,
// as a one-slice emission does, and no slice has written state. Filtering the
// roles after a slice's write would leave A's directory behind.
func TestEpicReviewBatch_AnUnknownRoleWritesNoSlice(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicThreeMilestones)
	_, stderr, code := runTP(t, dir, "epic", "review", "--all", "--role", "no-such-role")
	require.Equal(t, 2, code, "stderr: %s", stderr)
	_, err := os.Stat(filepath.Join(dir, ".tp", "epic-review"))
	assert.True(t, os.IsNotExist(err), "a refused batch writes no slice's state")
}

// TestEpicReviewBatch_RefusesAMixedOrBatchWriteCall is §5's exclusivity and
// the non-goals: --milestones and --all exclude each other and every
// one-slice flag, and batch --record, --resolve and --resolve-all are refused;
// each exits 2 and writes nothing.
func TestEpicReviewBatch_RefusesAMixedOrBatchWriteCall(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicThreeMilestones)
	record := epicRecordFile(t, dir, "")
	for _, args := range [][]string{
		{"--all", "--milestones", "A"},
		{"--all", "--milestone", "A"},
		{"--milestones", "A", "--epic", "a"},
		{"--milestones", "A", "--epics", "a,b"},
		{"--all", "--name", "n"},
		{"--milestones", ""},
		{"--milestones", "A,,B"},
		{"--all", "--record", record},
		{"--all", "--resolve", "0", "fixed"},
		{"--milestones", "A", "--resolve-all", "fixed"},
	} {
		_, stderr, code := runTP(t, dir, append([]string{"epic", "review"}, args...)...)
		assert.Equal(t, 2, code, "%v: stderr: %s", args, stderr)
	}
	_, err := os.Stat(filepath.Join(dir, ".tp", "epic-review"))
	assert.True(t, os.IsNotExist(err), "a refused call writes no state")
}
