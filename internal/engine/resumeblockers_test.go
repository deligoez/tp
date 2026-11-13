package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func blockerCodes(bs []Blocker) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Code
	}
	return out
}

func TestBuildBlockers_UnexplainedChangesAgentClearable(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, Changes: []string{"a.txt", "b.txt"}, TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "unexplained-changes", bs[0].Code)
	assert.Equal(t, ClassAgentClearable, bs[0].Class)
	assert.NotEmpty(t, bs[0].Message)
	assert.Equal(t, 2, bs[0].Data["count"])
}

func TestBuildBlockers_NoReadyTaskEscalate(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "blocked", Status: model.StatusOpen, DependsOn: []string{"missing"}}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "no-ready-task", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, []string{"missing"}, bs[0].Data["blocked_by"])
}

func TestBuildBlockers_BudgetExhaustedAtCap(t *testing.T) {
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseReview, ReviewRounds: 4, ReviewMaxRounds: 4, TaskFile: &model.TaskFile{}})
	require.Len(t, bs, 1)
	assert.Equal(t, "review-budget-exhausted", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, 4, bs[0].Data["cap"])
}

func TestBuildBlockers_BudgetZeroNeverFires(t *testing.T) {
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseReview, ReviewRounds: 9, ReviewMaxRounds: 0, TaskFile: &model.TaskFile{}})
	assert.Empty(t, bs)
}

// TestBuildBlockers_AuditBudgetExhaustedFiresAtTheAuditCap: the audit loop's
// blocker fires once its round count reaches the cap, in the audit phase
// only, and names the cap in its data.
func TestBuildBlockers_AuditBudgetExhaustedFiresAtTheAuditCap(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusDone}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseAudit, AuditRounds: 3, AuditMaxRounds: 3, TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "audit-budget-exhausted", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, 3, bs[0].Data["cap"])

	bs = BuildBlockers(&BlockerInputs{Phase: PhaseReview, AuditRounds: 3, AuditMaxRounds: 3, TaskFile: tf})
	assert.Empty(t, bs, "the audit cap does not stop the review phase")
}

func TestBuildBlockers_SpecStaleEscalate(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, ReviewStale: true, SpecPath: "spec.md", TaskFile: tf})
	require.Len(t, bs, 1)
	assert.Equal(t, "spec-stale", bs[0].Code)
	assert.Equal(t, ClassEscalate, bs[0].Class)
	assert.Equal(t, "spec.md", bs[0].Data["spec"])
}

// TestBuildBlockers_SpecStaleNamesTheClearingSequence: the spec-stale message
// fills this spec's path into the two steps that clear it — emit a review round
// over the new text, then record it — in both phases the blocker fires in.
func TestBuildBlockers_SpecStaleNamesTheClearingSequence(t *testing.T) {
	t.Parallel()
	const spec = "docs/specs/feature.md"
	for _, tc := range []struct {
		phase string
		tf    *model.TaskFile
	}{
		{PhaseImplement, &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusOpen}}}},
		{PhaseAudit, &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusDone}}}},
	} {
		bs := BuildBlockers(&BlockerInputs{Phase: tc.phase, ReviewStale: true, SpecPath: spec, TaskFile: tc.tf})
		require.Len(t, bs, 1, tc.phase)
		assert.Equal(t, "spec-stale", bs[0].Code, tc.phase)
		assert.Equal(t, spec, bs[0].Data["spec"], tc.phase)
		assert.Contains(t, bs[0].Message, "tp review "+spec+",", "%s: names the emission", tc.phase)
		assert.Contains(t, bs[0].Message, "tp review "+spec+" --record <file>", "%s: names the recording", tc.phase)
	}
}

func TestBuildBlockers_FixedEmissionOrder(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusWIP}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, Changes: []string{"x"}, ReviewStale: true, SpecPath: "s.md", TaskFile: tf})
	assert.Equal(t, []string{"unexplained-changes", "spec-stale"}, blockerCodes(bs))
}

func TestBuildBlockers_CleanIsEmptyNotNil(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "ready", Status: model.StatusOpen}}}
	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf})
	assert.NotNil(t, bs)
	assert.Empty(t, bs)
}

// TestBudgetBlockerMessage_NamesTheOperatorOnlyForABlockingFixed: at the cap
// the blocker names the operator's two exits only when a blocking finding was
// marked fixed there. With none, the remaining findings are dispositioned like
// any other, and the message says so; with one, it says a fix no round re-read
// is the operator's to accept or to verify with one more round.
func TestBudgetBlockerMessage_NamesTheOperatorOnlyForABlockingFixed(t *testing.T) {
	t.Parallel()
	none := budgetBlockerMessage("review", 3, 0)
	assert.Contains(t, none, "disposition the remaining findings")
	assert.NotContains(t, none, "marked fixed")

	one := budgetBlockerMessage("review", 3, 1)
	assert.Contains(t, one, "1 blocking finding")
	assert.Contains(t, one, "marked fixed")
	assert.NotContains(t, one, "disposition the remaining findings")
}

// TestBuildBlockers_EpicBlockedNeedsTheImplementPhaseAndAnUndoneDependency is
// spec/1.5.0.md §6: a bound file whose epic waits on one undone dependency
// and holds no wip task reports epic-blocked in the implement phase; an epic
// block naming no dependency, or any other phase, reports none.
func TestBuildBlockers_EpicBlockedNeedsTheImplementPhaseAndAnUndoneDependency(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusOpen}}}
	blocked := &EpicBlock{Epic: "b", BlockedBy: []string{"c"}}

	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf, EpicBlock: blocked})
	require.Len(t, bs, 1)
	assert.Equal(t, "epic-blocked", bs[0].Code)
	assert.Equal(t, []string{"c"}, bs[0].Data["blocked_by_epics"])

	unblocked := &EpicBlock{Epic: "b", BlockedBy: []string{}}
	assert.Empty(t, BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf, EpicBlock: unblocked}), "nothing undone")
	assert.Empty(t, BuildBlockers(&BlockerInputs{Phase: PhaseReview, TaskFile: tf, EpicBlock: blocked}), "not implementing")
}

// TestBuildBlockers_APlainDependencyAddsNoAnyOfKey: REFERENCE.md's
// epic-blocked data is {epic, blocked_by_epics}, and blocked_by_any_of is
// added only while a group is unsatisfied, so an epic held by a plain
// dependency alone carries no such key, not even an empty one.
func TestBuildBlockers_APlainDependencyAddsNoAnyOfKey(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusOpen}}}
	blk := &EpicBlock{Epic: "b", BlockedBy: []string{"c"}}

	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf, EpicBlock: blk})

	require.Len(t, bs, 1)
	assert.NotContains(t, bs[0].Data, "blocked_by_any_of")
}

// TestBuildBlockers_AGroupAloneBlocksAndOffersTheOperatorsChoices is
// spec/1.7.0.md §7: an epic held only by the group one of {c1, c2} is
// epic-blocked, its message names the group, its data carries
// blocked_by_any_of beside an empty blocked_by_epics, and the operator
// action offers the group's records as alternatives. A blocker that judged
// BlockedBy alone reports nothing.
func TestBuildBlockers_AGroupAloneBlocksAndOffersTheOperatorsChoices(t *testing.T) {
	tf := &model.TaskFile{Tasks: []model.Task{{ID: "t", Status: model.StatusOpen}}}
	blk := &EpicBlock{Epic: "w", BlockedBy: []string{}, BlockedByAnyOf: [][]string{{"c1", "c2"}}, AnyOfConditions: [][]string{{"c1", "c2"}}}

	bs := BuildBlockers(&BlockerInputs{Phase: PhaseImplement, TaskFile: tf, EpicBlock: blk})
	require.Len(t, bs, 1)
	assert.Equal(t, "epic-blocked", bs[0].Code)
	assert.Equal(t, []string{}, bs[0].Data["blocked_by_epics"])
	assert.Equal(t, [][]string{{"c1", "c2"}}, bs[0].Data["blocked_by_any_of"])
	assert.Contains(t, bs[0].Message, "one of {c1, c2}")
	assert.Contains(t, bs[0].Message, "tp epic done c1 --evidence <evidence> or tp epic done c2 --evidence <evidence>")
	op, ok := bs[0].Data["operator"].(*EpicOperatorAction)
	require.True(t, ok)
	assert.Len(t, op.Choices, 1)
}

// blockerByCode returns the blocker with code, or nil.
func blockerByCode(bs []Blocker, code string) *Blocker {
	for i := range bs {
		if bs[i].Code == code {
			return &bs[i]
		}
	}
	return nil
}

// TestBuildBlockers_TheCarriedBlockerStandsInReviewAndNamesItsWayOut: an
// earlier unresolved carried row is a review-phase blocker whose command
// disposes it, with an epoch as the operator's way out; a row marked fixed at
// the cap offers only the operator's raise; outside review it never stands.
func TestBuildBlockers_TheCarriedBlockerStandsInReviewAndNamesItsWayOut(t *testing.T) {
	fixed := earlierHigh()
	fixed.Disposition = CarryResolved
	in := func(phase string, row CarriedRow, maxRounds int) *BlockerInputs {
		return &BlockerInputs{Phase: phase, SpecPath: "spec.md", TaskFile: &model.TaskFile{},
			ReviewRounds: 3, ReviewMaxRounds: maxRounds, ReviewLatestRound: 3, ReviewCarriedBlocking: []CarriedRow{row}}
	}

	open := blockerByCode(BuildBlockers(in(PhaseReview, earlierHigh(), 0)), "review-carried-blocking")
	require.NotNil(t, open)
	assert.Equal(t, 1, open.Data["count"])
	require.Len(t, open.Data["commands"], 1)
	assert.Contains(t, open.Data["commands"].([]string)[0], "--resolve 0 fixed|wontfix|duplicate")
	assert.Equal(t, EscalateRewritePastBlocking, open.Data["operator"].(map[string]any)["decision"])

	atCap := blockerByCode(BuildBlockers(in(PhaseReview, fixed, 3)), "review-carried-blocking")
	require.NotNil(t, atCap)
	assert.Empty(t, atCap.Data["commands"])
	assert.Equal(t, EscalateRaiseReviewCap, atCap.Data["operator"].(map[string]any)["decision"])

	assert.Nil(t, blockerByCode(BuildBlockers(in(PhaseImplement, earlierHigh(), 0)), "review-carried-blocking"))
}

// TestBuildBlockers_AtTheCapTheCarriedBlockerLeadsOnlyWhenTheLatestRoundIsDone:
// at the review cap the budget blocker points at the carried blocker only
// when the latest round is fully dispositioned with no blocking fix; a
// latest-round finding left open or marked fixed keeps its own message, and
// so does a cap with nothing carried.
func TestBuildBlockers_AtTheCapTheCarriedBlockerLeadsOnlyWhenTheLatestRoundIsDone(t *testing.T) {
	budget := func(carried []CarriedRow, undisposed, fixedAtCap int) string {
		b := blockerByCode(BuildBlockers(&BlockerInputs{Phase: PhaseReview, SpecPath: "spec.md", TaskFile: &model.TaskFile{},
			ReviewRounds: 3, ReviewMaxRounds: 3, ReviewLatestRound: 3, ReviewCarriedBlocking: carried,
			ReviewUndisposedAtCap: undisposed, ReviewBlockingFixedAtCap: fixedAtCap}), "review-budget-exhausted")
		require.NotNil(t, b)
		return b.Message
	}
	carried := []CarriedRow{earlierHigh()}
	assert.Equal(t, "review reached its 3-round cap with the latest round dispositioned; what holds it open is the carried blocking findings of review-carried-blocking",
		budget(carried, 0, 0))
	assert.Equal(t, budgetBlockerMessage("review", 3, 0), budget(carried, 1, 0))
	assert.Equal(t, budgetBlockerMessage("review", 3, 1), budget(carried, 0, 1))
	assert.Equal(t, budgetBlockerMessage("review", 3, 0), budget(nil, 0, 0))
}
