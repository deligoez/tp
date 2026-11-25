package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

const answerSpec = "A request with no location is refused\nwith exit 2.\n"

func TestFoundQuoteIn_FindsAVerbatimQuote(t *testing.T) {
	assert.True(t, FoundQuoteIn(answerSpec, "A request with no location is refused"))
}

func TestFoundQuoteIn_RefusesAQuoteAbsentFromTheText(t *testing.T) {
	assert.False(t, FoundQuoteIn(answerSpec, "A request with no location exits 0"))
}

// §10 row "The quote check normalises whitespace": the quote spans the
// spec's line break, so comparing raw bytes would refuse it.
func TestFoundQuoteIn_AQuoteSpanningALineBreakIsFound(t *testing.T) {
	assert.True(t, FoundQuoteIn(answerSpec, "is refused with   exit 2."))
}

// A blank quote is in every text, so accepting it would skip the check.
func TestFoundQuoteIn_ABlankQuoteIsNeverFound(t *testing.T) {
	assert.False(t, FoundQuoteIn(answerSpec, " \n\t"))
}

func TestFoundCitable_TheSpecAndTheDocumentedRuleFilesOnly(t *testing.T) {
	spec := "/p/spec/1.0.md"
	for _, ok := range []string{spec, "/p/CLAUDE.md", "/p/CONVENTIONS.md", "/p/skills/tp/REFERENCE.md"} {
		assert.True(t, FoundCitable(ok, spec), ok)
	}
	for _, bad := range []string{"/p/spec/2.0.md", "/p/README.md", "/p/x.go"} {
		assert.False(t, FoundCitable(bad, spec), bad)
	}
}

// undecidedRequest is a request the decision unit holds: a question at a
// location, filed by t1.
func undecidedRequest() FoundRequest {
	return FoundRequest{
		ID: "F1", Seq: 1, Location: "internal/q.go:9", Reproduction: "go test ./internal -run TestQ",
		Question: "which default?", Alternatives: []string{"a", "b"},
		Finder: FoundFinder{Task: "t1"}, State: FoundStateUndecided,
	}
}

func answerTasks() []model.Task {
	return []model.Task{{ID: "t1", Status: model.StatusOpen, DependsOn: []string{}}}
}

func taskAnswer() *FoundAnswer {
	return &FoundAnswer{
		Kind: FoundAnswerTask, Expected: "exit 0", Observed: "exit 1",
		Cited: "spec/x.md", Quote: "the default is a",
	}
}

// The answer keeps the request's location and reproduction, so its dedup key
// is the request's own: a task answer must still open a task rather than be
// taken for a duplicate of the request it answers.
func TestPlanFoundAnswer_ATaskAnswerIsNeverADuplicateOfItsRequest(t *testing.T) {
	plan, err := PlanFoundAnswer([]FoundRequest{undecidedRequest()}, answerTasks(), nil, 5, "F1", taskAnswer())

	require.NoError(t, err)
	assert.Empty(t, plan.Duplicates)
	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, "found-f1", plan.Tasks[0].ID)
	assert.Equal(t, []string{"t1"}, plan.Tasks[0].DependsOn, "placed like a Fix-shaped request")
	require.Len(t, plan.Requests, 1)
	assert.Equal(t, FoundStateApplied, plan.Requests[0].State)
	assert.Equal(t, "found-f1", plan.Requests[0].Outcome)
	assert.Equal(t, "exit 1", plan.Requests[0].Observed, "the store keeps the answer's Fix fields")
	assert.Equal(t, "which default?", plan.Requests[0].Question, "and the question it answered")
}

// The answer is a duplicate of another open request with its key, as any
// Fix-shaped request would be.
func TestPlanFoundAnswer_ATaskAnswerDedupsAgainstAnotherRequest(t *testing.T) {
	other := FoundRequest{
		ID: "F2", Seq: 2, Location: "internal/q.go:12", Reproduction: "go test ./internal -run TestQ",
		State: FoundStateApplied, Outcome: "found-f2",
	}
	tasks := append(answerTasks(), model.Task{ID: "found-f2", Status: model.StatusOpen, Origin: &model.Origin{}})

	plan, err := PlanFoundAnswer([]FoundRequest{undecidedRequest(), other}, tasks, nil, 5, "F1", taskAnswer())

	require.NoError(t, err)
	assert.Empty(t, plan.Tasks)
	require.Len(t, plan.Duplicates, 1)
	assert.Equal(t, "found-f2", plan.Requests[0].Outcome)
}

// Other unapplied requests wait for tp found apply; the answer applies only
// the request it answers.
func TestPlanFoundAnswer_LeavesOtherUnappliedRequestsAlone(t *testing.T) {
	other := FoundRequest{
		ID: "F2", Seq: 2, Location: "internal/z.go:1", Reproduction: "go test ./z",
		Expected: "0", Observed: "1", State: FoundStateUnapplied,
	}

	plan, err := PlanFoundAnswer([]FoundRequest{undecidedRequest(), other}, answerTasks(), nil, 5, "F1", taskAnswer())

	require.NoError(t, err)
	require.Len(t, plan.Tasks, 1)
	assert.Equal(t, other, plan.Requests[1])
}

func TestPlanFoundAnswer_ATaskAnswerPastTheCapIsAnOpenEntryWithItsQuote(t *testing.T) {
	tasks := append(answerTasks(), model.Task{ID: "found-f9", Status: model.StatusDone, Origin: &model.Origin{}})

	plan, err := PlanFoundAnswer([]FoundRequest{undecidedRequest()}, tasks, nil, 1, "F1", taskAnswer())

	require.NoError(t, err)
	assert.Empty(t, plan.Tasks)
	require.Len(t, plan.Entries, 1)
	e := plan.Entries[0]
	assert.Equal(t, FoundQueueOpen, e.State)
	assert.Equal(t, "F1", e.Request)
	assert.Equal(t, "the default is a", e.Quote)
	assert.Equal(t, "spec/x.md", e.Cited)
	assert.Equal(t, "go test ./internal -run TestQ", e.Reproduction)
	assert.Equal(t, FoundStateApplied, plan.Requests[0].State)
}

func TestPlanFoundAnswer_ATaskAnswerNeedsTheFixFields(t *testing.T) {
	ans := taskAnswer()
	ans.Observed = " "

	_, err := PlanFoundAnswer([]FoundRequest{undecidedRequest()}, answerTasks(), nil, 5, "F1", ans)

	require.ErrorContains(t, err, "Fix fields")
}

func TestPlanFoundAnswer_AConformsAnswerIsADecidedEntryAndNoTask(t *testing.T) {
	ans := &FoundAnswer{Kind: FoundAnswerConforms, Reproduction: "go test ./internal -run TestQPinned", Cited: "CLAUDE.md", Quote: "q"}

	plan, err := PlanFoundAnswer([]FoundRequest{undecidedRequest()}, answerTasks(), nil, 5, "F1", ans)

	require.NoError(t, err)
	assert.Empty(t, plan.Tasks)
	require.Len(t, plan.Entries, 1)
	e := plan.Entries[0]
	assert.Equal(t, FoundQueueDecided, e.State)
	assert.Equal(t, FoundAnswerConforms, e.Answer)
	assert.Equal(t, "go test ./internal -run TestQPinned", e.Reproduction)
	assert.Equal(t, "CLAUDE.md", e.Cited)
	assert.Equal(t, "q", e.Quote)
	assert.Equal(t, "which default?", e.Question)
	assert.Empty(t, e.Finders)
	assert.Equal(t, FoundStateApplied, plan.Requests[0].State)
}

func TestPlanFoundAnswer_AHumanAnswerIsAnOpenEntryWithTheQuestionAndAlternatives(t *testing.T) {
	req := undecidedRequest()
	req.Blocks = true

	plan, err := PlanFoundAnswer([]FoundRequest{req}, answerTasks(), nil, 5, "F1", &FoundAnswer{Kind: FoundAnswerHuman})

	require.NoError(t, err)
	assert.Empty(t, plan.Tasks)
	require.Len(t, plan.Entries, 1)
	e := plan.Entries[0]
	assert.Equal(t, FoundQueueOpen, e.State)
	assert.Equal(t, "F1", e.Request)
	assert.Equal(t, "internal/q.go:9", e.Location)
	assert.Equal(t, "which default?", e.Question)
	assert.Equal(t, []string{"a", "b"}, e.Alternatives)
	assert.Equal(t, []string{"t1"}, e.Finders, "a blocking request's entry holds its finder")
	assert.Equal(t, FoundStateApplied, plan.Requests[0].State)
}

func TestPlanFoundAnswer_RefusesARequestThatIsNotUndecided(t *testing.T) {
	req := undecidedRequest()
	req.State = FoundStateApplied

	_, err := PlanFoundAnswer([]FoundRequest{req}, answerTasks(), nil, 5, "F1", &FoundAnswer{Kind: FoundAnswerHuman})
	require.ErrorContains(t, err, "not undecided")

	_, err = PlanFoundAnswer([]FoundRequest{req}, answerTasks(), nil, 5, "F7", &FoundAnswer{Kind: FoundAnswerHuman})
	require.ErrorContains(t, err, "F7")
}
