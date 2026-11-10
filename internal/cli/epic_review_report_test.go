package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// epicReportClass is one class of tp epic review --report.
type epicReportClass struct {
	Class  string   `json:"class"`
	Epics  []string `json:"epics"`
	Slices []struct {
		Key    *epicSliceKeyPayload `json:"key"`
		Rounds []int                `json:"rounds"`
	} `json:"slices"`
}

// runEpicReport runs tp epic review --report <args>, decodes its classes on
// exit 0 and returns the raw payload's top-level keys too.
func runEpicReport(t *testing.T, dir string, args ...string) (classes []epicReportClass, keys map[string]json.RawMessage) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"epic", "review", "--report"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.NoError(t, json.Unmarshal([]byte(stdout), &keys), "stdout: %s", stdout)
	var out struct {
		Classes []epicReportClass `json:"classes"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out.Classes, keys
}

// epicClassRow is a recorded finding of class on location.
func epicClassRow(class, location string) string {
	return `{"severity":"low","finding":"` + class + ` on ` + location + `","location":"` + location + `","evidence":"e","class":"` + class + `"}` + "\n"
}

// epicReportFixture records two rounds on milestone A and one on --epic a:
// unchecked-acceptance on a (A round 1, epic a round 1) and on b (A round 2),
// vague-why on b (A round 1), and one row with no class.
func epicReportFixture(t *testing.T) string {
	t.Helper()
	dir := epicGraphDir(t, epicThreeMilestones)
	runEpicReview(t, dir, "--milestone", "A")
	recordEpicRound(t, dir, epicClassRow("unchecked-acceptance", "epic:a")+epicClassRow("vague-why", "epic:b")+
		`{"severity":"low","finding":"no class","location":"epic:a","evidence":"e"}`+"\n", "--milestone", "A")
	runEpicReview(t, dir, "--epic", "a")
	recordEpicRound(t, dir, epicClassRow("unchecked-acceptance", "epic:a"), "--epic", "a")
	runEpicReview(t, dir, "--milestone", "A")
	recordEpicRound(t, dir, epicClassRow("unchecked-acceptance", "epic:b"), "--milestone", "A")
	return dir
}

// TestEpicReviewReport_AllListsEachClassWithItsSlicesRoundsAndEpics is §3's
// report over every slice directory: unchecked-acceptance raised on A's
// rounds 1 and 2 and on --epic a's round 1 names the distinct epics a and b
// once each, though a was raised in two slices; vague-why names A's round 1
// and b; a row with no class is in no class; and the payload carries no
// mechanize_candidates. Counting a twice, or suggesting mechanisation, fails
// it.
func TestEpicReviewReport_AllListsEachClassWithItsSlicesRoundsAndEpics(t *testing.T) {
	t.Parallel()
	dir := epicReportFixture(t)

	classes, keys := runEpicReport(t, dir, "--all")
	assert.NotContains(t, keys, "mechanize_candidates")
	require.Len(t, classes, 2)
	unchecked := classes[0]
	assert.Equal(t, "unchecked-acceptance", unchecked.Class, "the class raised on more epics comes first")
	assert.Equal(t, []string{"a", "b"}, unchecked.Epics)
	rounds := map[string][]int{}
	for _, s := range unchecked.Slices {
		require.NotNil(t, s.Key)
		rounds[s.Key.Kind+":"+s.Key.Value] = s.Rounds
	}
	assert.Equal(t, map[string][]int{"milestone:A": {1, 2}, "epic:a": {1}}, rounds)
	vague := classes[1]
	assert.Equal(t, "vague-why", vague.Class)
	assert.Equal(t, []string{"b"}, vague.Epics)
	require.Len(t, vague.Slices, 1)
	assert.Equal(t, []int{1}, vague.Slices[0].Rounds)
}

