package cli_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicDescriptionGraph is a two-epic graph: a depends on b, neither described.
const epicDescriptionGraph = `{
  "version": 1,
  "milestones": [["A"]],
  "epics": [
    {"id": "a", "title": "A step", "milestone": "A", "depends_on": [{"id": "b", "why": "a reads what b writes"}], "acceptance": "a ships"},
    {"id": "b", "title": "B step", "milestone": "A", "root": true, "acceptance": "b ships"}
  ]
}
`

// TestEpicSet_DescriptionWritesTheTextAndBlankUnsetsIt (spec/backlog/
// epic-description.md §1): tp epic set <id> description=<text> writes the
// text as given, and an empty or whitespace-only value leaves the key out of
// the file, as an unset optional field is.
func TestEpicSet_DescriptionWritesTheTextAndBlankUnsetsIt(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)

	_, stderr, code := runTP(t, dir, "epic", "set", "a", "description=Purpose: prove the chain.")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, "Purpose: prove the chain.", appliedEpic(t, &g, "a").Description)

	for _, blank := range []string{"description=", "description=  \n\t "} {
		_, stderr, code = runTP(t, dir, "epic", "set", "a", blank)
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.NotContains(t, string(readEpicGraphFile(t, dir)), `"description"`, "%q unsets the description", blank)
		_, stderr, code = runTP(t, dir, "epic", "set", "a", "description=back")
		require.Equal(t, 0, code, "stderr: %s", stderr)
	}
}

// TestEpicReviewStatus_ADescriptionEditReAsksTheDescribedEpicAlone is the
// own-block rule at the loop: after a recorded round, describing b makes the
// slice stale on epic:b only; a, which depends on b, is not re-asked.
func TestEpicReviewStatus_ADescriptionEditReAsksTheDescribedEpicAlone(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)

	_, stderr, code := runTP(t, dir, "epic", "set", "b", "description=Purpose: feed a.")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	stale := decodeStaleUnits(t, dir, slice...)
	require.NotNil(t, stale.StaleUnits)
	assert.Equal(t, []string{"epic:b"}, *stale.StaleUnits)
}

// epicDescribedSlice is milestone A (a, b) gated with a threshold, so it
// holds a scope unit, and milestone B (c, d), where a depends on c and d on
// b: c and d are A's context. Every epic carries a description naming it,
// a's a long one.
func epicDescribedSlice(longA string) string {
	return `{
  "version": 1,
  "milestones": [["A", "B"]],
  "gates": {"A": {"members": [], "threshold": "ten users"}},
  "epics": [
    {"id": "a", "title": "A step", "milestone": "A", "depends_on": [{"id": "b", "why": "a reads b"}, {"id": "c", "why": "a runs on c"}], "acceptance": "a ships", "description": "DESC-a ` + longA + `"},
    {"id": "b", "title": "B step", "milestone": "A", "root": true, "acceptance": "b ships", "description": "DESC-b\n\nscope: b alone"},
    {"id": "c", "title": "C step", "milestone": "B", "root": true, "acceptance": "c ships", "description": "DESC-c"},
    {"id": "d", "title": "D step", "milestone": "B", "depends_on": [{"id": "b", "why": "d extends b"}], "acceptance": "d ships", "description": "DESC-d"}
  ]
}
`
}

// TestEpicReview_PromptsShowTheAskedEpicsDescriptionsOnly is the prompt
// rule: every role asked epics reads each asked epic's description once, in
// full however long, inside that epic's block; no prompt shows a context
// epic's description, and gate-scope's prompt shows none at all.
func TestEpicReview_PromptsShowTheAskedEpicsDescriptionsOnly(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("long purpose text ", 400)
	dir := epicGraphDir(t, epicDescribedSlice(long))

	out := runEpicReview(t, dir, "--milestone", "A")
	require.Contains(t, promptRoles(&out), "gate-scope")
	for _, p := range out.Prompts {
		if p.Role == "gate-scope" {
			assert.NotContains(t, p.Prompt, "DESC-", "gate-scope reads no description")
			continue
		}
		assert.Equal(t, 1, strings.Count(p.Prompt, "DESC-a "+strings.TrimSpace(long)), "%s reads a's description once, in full", p.Role)
		assert.Equal(t, 1, strings.Count(p.Prompt, "description:\n  DESC-b\n\n  scope: b alone\n"), "%s reads b's description in b's block", p.Role)
		assert.NotContains(t, p.Prompt, "DESC-c", "%s: c is context", p.Role)
		assert.NotContains(t, p.Prompt, "DESC-d", "%s: d is context", p.Role)
	}
}

// epicAskedBytes runs tp epic review <args> --status and returns the
// asked_block_bytes of the payload, or of each batch row; a missing key
// fails.
func epicAskedBytes(t *testing.T, dir string, args ...string) []int {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append(append([]string{"epic", "review"}, args...), "--status")...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		AskedBlockBytes *int `json:"asked_block_bytes"`
		Slices          []struct {
			AskedBlockBytes *int `json:"asked_block_bytes"`
		} `json:"slices"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	if out.Slices == nil {
		require.NotNil(t, out.AskedBlockBytes, "--status carries asked_block_bytes: %s", stdout)
		return []int{*out.AskedBlockBytes}
	}
	sizes := make([]int, 0, len(out.Slices))
	for _, row := range out.Slices {
		require.NotNil(t, row.AskedBlockBytes, "each batch row carries asked_block_bytes: %s", stdout)
		sizes = append(sizes, *row.AskedBlockBytes)
	}
	return sizes
}

// TestEpicReviewStatus_ReportsTheBytesOfTheBlocksARoundWouldAsk is the size
// proxy (spec/backlog/epic-description.md, Prompt size): before any round the
// slice asks every unit, after a recorded round with nothing changed it asks
// none, and describing b asks b's block alone, the description's bytes
// included; a batch row reports what the slice's own --status does.
func TestEpicReviewStatus_ReportsTheBytesOfTheBlocksARoundWouldAsk(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}

	first := epicAskedBytes(t, dir, slice...)[0]
	assert.Positive(t, first)
	assert.Equal(t, []int{first}, epicAskedBytes(t, dir, "--milestones", "A"))
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)
	assert.Equal(t, []int{0}, epicAskedBytes(t, dir, slice...), "a fresh slice asks nothing")

	_, stderr, code := runTP(t, dir, "epic", "set", "b", "description=Purpose: feed a.")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	bBlock := "epic: b\ntitle: B step\nmilestone: A\nacceptance: b ships\ndescription:\n  Purpose: feed a.\n"
	assert.Equal(t, []int{len(bBlock)}, epicAskedBytes(t, dir, slice...))
	assert.Equal(t, []int{len(bBlock)}, epicAskedBytes(t, dir, "--all"))
}

// TestEpicReview_EachPromptReportsItsBytes: the emission names each role's
// real prompt size beside the prompt, in bytes.
func TestEpicReview_EachPromptReportsItsBytes(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescribedSlice("text"))
	stdout, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Prompts []struct {
			Role        string `json:"role"`
			Prompt      string `json:"prompt"`
			PromptBytes *int   `json:"prompt_bytes"`
		} `json:"prompts"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	require.NotEmpty(t, out.Prompts)
	for _, p := range out.Prompts {
		require.NotNil(t, p.PromptBytes, p.Role)
		assert.Equal(t, len(p.Prompt), *p.PromptBytes, p.Role)
	}
}

// TestEpicApply_SetRowCarriesAMultiLineDescription: an apply set row's JSON
// string value carries multi-line markdown with no shell quoting, and the
// file keeps it byte for byte.
func TestEpicApply_SetRowCarriesAMultiLineDescription(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	text := "## Purpose\n\nProve the chain.\n\n- non-goal: speed\n- open: which runner?\n"
	value, err := json.Marshal(text)
	require.NoError(t, err)
	payload := applyPayload(t, dir, `{"op":"set","id":"b","field":"description","value":`+string(value)+`}`)

	_, stderr, code := runTP(t, dir, "epic", "apply", payload)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, text, appliedEpic(t, &g, "b").Description)
}

// TestEpicAdd_TakesADescriptionAndTheLintKnowsTheKey: tp epic add accepts the
// key under its strict decode, and tp epic lint reports no unknown key for
// it; before the field existed both refused it.
func TestEpicAdd_TakesADescriptionAndTheLintKnowsTheKey(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)

	_, stderr, code := runTP(t, dir, "epic", "add", `{"id":"c","title":"C step","milestone":"A","root":true,"description":"Why c exists."}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	g := loadAppliedGraph(t, dir)
	assert.Equal(t, "Why c exists.", appliedEpic(t, &g, "c").Description)

	stdout, stderr, code := runTP(t, dir, "epic", "lint")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, stdout, "unknown-key")
}

// TestEpicAdd_AWhitespaceDescriptionIsNotWritten: a whitespace-only
// description is unset, so the write leaves the key out.
func TestEpicAdd_AWhitespaceDescriptionIsNotWritten(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)

	_, stderr, code := runTP(t, dir, "epic", "add", `{"id":"c","title":"C step","milestone":"A","root":true,"description":" \n "}`)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.False(t, strings.Contains(string(readEpicGraphFile(t, dir)), `"description"`))
}
