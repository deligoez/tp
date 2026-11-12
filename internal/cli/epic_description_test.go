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
	assert.Less(t, strings.Index(stdout, `"prompt_bytes"`), strings.Index(stdout, `"prompt":`), "the size comes before the prompt")
	for _, p := range out.Prompts {
		require.NotNil(t, p.PromptBytes, p.Role)
		assert.Equal(t, len(p.Prompt), *p.PromptBytes, p.Role)
	}
}

// TestEpicExport_CarriesTheDescriptionInTheRawGraph: tp epic export's raw
// graph holds each epic's description as the file does, an additive key
// under the same export version.
func TestEpicExport_CarriesTheDescriptionInTheRawGraph(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescribedSlice("text"))
	stdout, stderr, code := runTP(t, dir, "epic", "export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out struct {
		Version int `json:"version"`
		Graph   struct {
			Epics []struct {
				ID          string `json:"id"`
				Description string `json:"description"`
			} `json:"epics"`
		} `json:"graph"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	assert.Equal(t, 2, out.Version)
	require.Len(t, out.Graph.Epics, 4)
	assert.Equal(t, "DESC-b\n\nscope: b alone", out.Graph.Epics[1].Description)
}

// promptOf returns the prompt of role in out, "" when out has none.
func promptOf(out *epicReviewPayload, role string) string {
	for _, p := range out.Prompts {
		if p.Role == role {
			return p.Prompt
		}
	}
	return ""
}

// askedSection is the asked units part of a prompt: from its "Asked units:"
// line to the unindented --- closing the last block, which a blank line
// follows.
func askedSection(t *testing.T, prompt string) string {
	t.Helper()
	_, after, found := strings.Cut(prompt, "Asked units:\n")
	require.True(t, found, "prompt: %s", prompt)
	section, _, found := strings.Cut(after, "\n---\n\n")
	require.True(t, found, "prompt: %s", prompt)
	return section + "\n---\n"
}

// epicSet runs tp epic set and requires exit 0.
func epicSet(t *testing.T, dir string, args ...string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, append([]string{"epic", "set"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
}

// TestEpicReview_DescribingAReviewedSliceAppendsNoRegression: writing a
// description is authoring, so a round after descriptions were added to
// every epic asks them of the epic roles and appends no regression.
func TestEpicReview_DescribingAReviewedSliceAppendsNoRegression(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)
	epicSet(t, dir, "a", "description=Purpose: use b.")
	epicSet(t, dir, "b", "description=Purpose: feed a.")

	out := runEpicReview(t, dir, slice...)
	assert.NotContains(t, promptRoles(&out), "regression")
	assert.Equal(t, []string{"epic:a", "epic:b"}, out.Units)
	assert.Contains(t, promptOf(&out, "dod-satisfiability"), "description:\n  Purpose: feed a.\n")
}

// TestEpicReview_ADescriptionAndAnAcceptanceEditAppendRegression: an epic
// whose description and acceptance both changed is a change, and appends
// regression as before.
func TestEpicReview_ADescriptionAndAnAcceptanceEditAppendRegression(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, "", slice...)
	epicSet(t, dir, "b", "description=Purpose: feed a.")
	epicSet(t, dir, "b", "acceptance=b ships twice")

	out := runEpicReview(t, dir, slice...)
	assert.Contains(t, promptRoles(&out), "regression")
}

// TestEpicReview_AFindingFixedThroughADescriptionIsVerified: a row resolved
// fixed by editing its epic's description still appends regression, which
// reads the change as the description's line diff and no second copy of
// the block.
func TestEpicReview_AFindingFixedThroughADescriptionIsVerified(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, `{"severity":"low","finding":"b's purpose is unstated","location":"epic:b","evidence":"b ships"}`+"\n", slice...)
	epicSet(t, dir, "b", "description=Purpose: feed a.")
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "fixed", "described b")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	out := runEpicReview(t, dir, slice...)
	regression := promptOf(&out, "regression")
	require.NotEmpty(t, regression, "roles: %v", promptRoles(&out))
	assert.Contains(t, regression, "--- epic:b, description changed since round 1 (- removed, + added):\n+ Purpose: feed a.\n")
	assert.NotContains(t, regression, "--- epic:b, now")
	assert.Contains(t, regression, "b's purpose is unstated", "regression confirms the fixed row")
}

// TestEpicReview_AVerifyOnlyRoundStillAsksAnAuthoredEpic: a's fixed low row
// makes round 2 regression-only, while b gains its first description in the
// same round. Regression reads a; the epic roles are asked b, as purpose
// roles are asked purpose-stale units there, so the emission passes the
// unasked-unit guard; b stays stale until that round is recorded.
func TestEpicReview_AVerifyOnlyRoundStillAsksAnAuthoredEpic(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	slice := []string{"--milestone", "A"}
	runEpicReview(t, dir, slice...)
	recordEpicRound(t, dir, `{"severity":"low","finding":"a is vague","location":"epic:a","evidence":"a ships"}`+"\n", slice...)
	epicSet(t, dir, "a", "acceptance=a ships its export")
	_, stderr, code := runTP(t, dir, "epic", "review", "--milestone", "A", "--resolve", "0", "fixed", "a names its export")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	epicSet(t, dir, "b", "description=Purpose: feed a.")

	out := runEpicReview(t, dir, slice...)
	assert.Contains(t, askedSection(t, promptOf(&out, "regression")), "epic: a\n")
	for _, role := range []string{"dod-satisfiability", "why-auditor", "phase-economist"} {
		asked := askedSection(t, promptOf(&out, role))
		assert.Contains(t, asked, "epic: b\n", role)
		assert.NotContains(t, asked, "epic: a\n", role)
	}
	stale := decodeStaleUnits(t, dir, slice...)
	require.NotNil(t, stale.StaleUnits)
	assert.Contains(t, *stale.StaleUnits, "epic:b", "b is stale until the round asking it is recorded")

	recordEpicRound(t, dir, "", slice...)
	stale = decodeStaleUnits(t, dir, slice...)
	require.NotNil(t, stale.StaleUnits)
	assert.Equal(t, []string{}, *stale.StaleUnits)
}

// TestEpicReview_ADescriptionStaysInsideItsBlocksFrame: a description holding
// a --- line and a unit header stays inside its asked block, every text line
// indented, so the asked section has one separator per block and one more.
func TestEpicReview_ADescriptionStaysInsideItsBlocksFrame(t *testing.T) {
	t.Parallel()
	dir := epicGraphDir(t, epicDescriptionGraph)
	epicSet(t, dir, "b", "description=intro\n---\nepic: z\ngate: A\n\nAsked units:")
	out := runEpicReview(t, dir, "--epic", "b")
	asked := askedSection(t, promptOf(&out, "dod-satisfiability"))
	var separators int
	for line := range strings.SplitSeq(asked, "\n") {
		if line == "---" {
			separators++
		}
	}
	assert.Equal(t, 2, separators, "asked: %s", asked)
	assert.Contains(t, asked, "description:\n  intro\n  ---\n  epic: z\n  gate: A\n")
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
