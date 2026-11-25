package engine

import (
	"testing"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func batchTask(id string, touches ...string) model.Task {
	return model.Task{ID: id, Status: model.StatusOpen, EstimateMinutes: 5, Touches: touches}
}

// TestImplementBatch_TakesDisjointReadyTasksUpToTheCap: the batch leads with
// the task tp next would take and adds, in the same order, ready tasks whose
// touches are disjoint from every task already in it, up to run_max_parallel
// (spec/backlog/parallel-implement.md §2.2).
func TestImplementBatch_TakesDisjointReadyTasksUpToTheCap(t *testing.T) {
	t.Parallel()
	tf := &model.TaskFile{Tasks: []model.Task{
		batchTask("a", "internal/engine/"),
		batchTask("b", "internal/engine/lock.go"),
		batchTask("c", "internal/cli/"),
		batchTask("d", "docs/"),
	}}
	assert.Equal(t, []string{"a", "c", "d"}, ImplementUnits(tf, 3, nil))
	assert.Equal(t, []string{"a", "c"}, ImplementUnits(tf, 2, nil))
	assert.Equal(t, []string{"a"}, ImplementUnits(tf, 1, nil), "the default cap runs one at a time")
}

// TestTouchesWarnings: under run_max_parallel above 1 each open task without
// touches is named, since it joins a batch unscreened; at 1 nothing is.
func TestTouchesWarnings(t *testing.T) {
	t.Parallel()
	done := batchTask("d")
	done.Status = model.StatusDone
	tf := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("b", "x/"), batchTask("c"), done}}
	assert.Empty(t, TouchesWarnings(tf, 1))
	got := TouchesWarnings(tf, 3)
	require.Len(t, got, 1, "one finding names every such task")
	assert.Equal(t, Finding{Severity: "warning", Rule: "touches",
		Message: `2 open tasks have no touches, so they run beside other tasks unscreened, and on a clash the later unit to integrate is discarded and rerun alone (run_max_parallel 3): a, c; set each with tp set <id> 'touches=["path/to/file.go","dir/**"]'; a pool worktree starts without ignored files, which the operator's worktree_setup_cmd can prepare`}, got[0])
}

// TestTouchesWarnings_Boundaries: the rule starts at run_max_parallel 2, says
// nothing when every open task names its touches, and speaks of one task in the
// singular.
func TestTouchesWarnings_Boundaries(t *testing.T) {
	t.Parallel()
	one := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("b", "x/")}}
	got := TouchesWarnings(one, 2)
	require.Len(t, got, 1, "the cap of 2 already batches")
	assert.Contains(t, got[0].Message, "1 open task has no touches, so it runs beside")
	assert.Empty(t, TouchesWarnings(&model.TaskFile{Tasks: []model.Task{batchTask("b", "x/")}}, 3), "nothing to name")
	assert.True(t, TouchesOverlap([]string{"x/"}, nil), "an empty side still overlaps for any other caller")
}

// TestImplementBatch_TouchesAreAHint: a task without touches joins a batch,
// leading or behind, since only touches both tasks name can be compared; the
// pick and the gate catch a clash (v1.20.0, the operator's decision).
func TestImplementBatch_TouchesAreAHint(t *testing.T) {
	t.Parallel()
	noTouches := &model.TaskFile{Tasks: []model.Task{batchTask("a"), batchTask("c", "internal/cli/")}}
	assert.Equal(t, []string{"a", "c"}, ImplementUnits(noTouches, 3, nil))

	behind := &model.TaskFile{Tasks: []model.Task{batchTask("a", "x/"), batchTask("b"), batchTask("c", "x/y.go"), batchTask("d", "y/")}}
	assert.Equal(t, []string{"a", "b", "d"}, ImplementUnits(behind, 3, nil), "named touches still keep an overlapping task out")
}

// TestImplementBatch_ASoloOrWIPOrUnsafeTaskRunsAlone: a task a failed
// integration marked solo runs alone and never joins a batch; a wip task is
// resumed alone before any batch.
func TestImplementBatch_ASoloOrWIPOrUnsafeTaskRunsAlone(t *testing.T) {
	t.Parallel()
	solo := batchTask("a", "x/")
	solo.Solo = true
	soloFirst := &model.TaskFile{Tasks: []model.Task{solo, batchTask("c", "y/")}}
	assert.Equal(t, []string{"a"}, ImplementUnits(soloFirst, 3, nil))
	trailing := batchTask("z", "x/")
	trailing.Solo = true
	soloBehind := &model.TaskFile{Tasks: []model.Task{batchTask("c", "y/"), trailing}}
	assert.Equal(t, []string{"c"}, ImplementUnits(soloBehind, 3, nil), "a solo task never joins a batch")

	wip := batchTask("w", "x/")
	wip.Status = model.StatusWIP
	withWIP := &model.TaskFile{Tasks: []model.Task{batchTask("a", "y/"), wip}}
	assert.Equal(t, []string{"w"}, ImplementUnits(withWIP, 3, nil))

	assert.Empty(t, ImplementUnits(&model.TaskFile{}, 3, nil))

	unsafe := &model.TaskFile{Tasks: []model.Task{batchTask("a", "x/"), batchTask("b/c", "y/"), batchTask("d..e", "z/"), batchTask("f.", "w/")}}
	assert.Equal(t, []string{"a"}, ImplementUnits(unsafe, 3, nil), "an id unsafe in a branch name or a path never joins a batch")
}

// TestImplementUnits_AProjectConfigTaskRunsInTheMainTree: a task whose
// touches may name .tp/config.json, which only the main tree commits, never
// joins a pool: next in line it runs alone once the pool drains, and behind
// another it is passed over. A task without touches is not taken for one
// (spec/backlog/worktree-dispatch.md §2.1).
func TestImplementUnits_AProjectConfigTaskRunsInTheMainTree(t *testing.T) {
	t.Parallel()
	for _, glob := range []string{".tp/config.json", ".tp/", ".tp/**", ".tp/*.json"} {
		// tp next's order breaks the tie by id: a leads, z trails.
		lead := &model.TaskFile{Tasks: []model.Task{batchTask("a", glob), batchTask("c", "y/")}}
		assert.Equal(t, []string{"a"}, ImplementUnits(lead, 3, nil), "%s: it runs alone", glob)
		assert.Empty(t, ImplementUnits(lead, 3, []string{"busy"}), "%s: it waits for the pool to drain", glob)
		behind := &model.TaskFile{Tasks: []model.Task{batchTask("c", "y/"), batchTask("z", glob)}}
		assert.Equal(t, []string{"c"}, ImplementUnits(behind, 3, nil), "%s: it never joins a pool", glob)
	}
	elsewhere := &model.TaskFile{Tasks: []model.Task{batchTask("a", ".tp/roles/"), batchTask("b"), batchTask("c", "y/")}}
	assert.Equal(t, []string{"a", "b", "c"}, ImplementUnits(elsewhere, 3, nil), "other .tp paths and a task without touches still pool")
}

// TestClaimOverlap: only a held claim, wip with a spawn or a session, keeps
// an overlapping task back; a wip task without a claim, a claim holding only
// retired tokens, a task without touches and the claimed task itself do not
// (spec/backlog/worktree-dispatch.md §2.3).
func TestClaimOverlap(t *testing.T) {
	t.Parallel()
	wip := func(task model.Task, claim *model.ClaimedBy) model.Task {
		task.Status, task.ClaimedBy = model.StatusWIP, claim
		return task
	}
	tf := &model.TaskFile{Tasks: []model.Task{
		wip(batchTask("held", "go.mod"), &model.ClaimedBy{Spawn: "toolu_1"}),
		wip(batchTask("run", "x/"), nil),
		wip(batchTask("retired", "y/"), &model.ClaimedBy{Retired: []string{"tok"}}),
		batchTask("lock", "go.mod", "go.sum"),
		batchTask("bare"),
		batchTask("inx", "x/a.go"),
		batchTask("iny", "y/a.go"),
	}}
	got := ClaimOverlap(tf, &tf.Tasks[3])
	require.NotNil(t, got)
	assert.Equal(t, "held", got.ID)
	assert.Nil(t, ClaimOverlap(tf, &tf.Tasks[4]), "a task without touches is unscreened")
	assert.Nil(t, ClaimOverlap(tf, &tf.Tasks[5]), "a wip task no one claimed holds nothing")
	assert.Nil(t, ClaimOverlap(tf, &tf.Tasks[6]), "a retired-only claim is no claim")
	assert.Nil(t, ClaimOverlap(tf, &tf.Tasks[0]), "a claim does not overlap itself")
}

// TestImplementUnits_RefillsAroundARunningPool: with the pool's tasks named
// as running, the oracle leaves them out, takes only tasks disjoint from
// theirs, and fills the free slots alone (spec/backlog/rolling-implement.md
// §2). A wip task that is not running, or a solo task next in line, waits for
// the pool to drain, and then runs alone.
func TestImplementUnits_RefillsAroundARunningPool(t *testing.T) {
	t.Parallel()
	running := func(task model.Task) model.Task {
		task.Status = model.StatusWIP
		return task
	}
	pool := &model.TaskFile{Tasks: []model.Task{
		running(batchTask("a", "x/")),
		batchTask("b", "x/y.go"),
		batchTask("c", "y/"),
		batchTask("d", "z/"),
	}}
	assert.Equal(t, []string{"c", "d"}, ImplementUnits(pool, 3, []string{"a"}), "b overlaps the running a")
	assert.Equal(t, []string{"c"}, ImplementUnits(pool, 2, []string{"a"}), "one free slot")
	assert.Empty(t, ImplementUnits(pool, 1, []string{"a"}), "no free slot")

	abandoned := running(batchTask("w", "w/"))
	withWIP := &model.TaskFile{Tasks: []model.Task{running(batchTask("a", "x/")), abandoned, batchTask("c", "y/")}}
	assert.Empty(t, ImplementUnits(withWIP, 3, []string{"a"}), "a wip task no pool runs waits for the drain")
	withWIP.Tasks[0].Status = model.StatusDone
	assert.Equal(t, []string{"w"}, ImplementUnits(withWIP, 3, nil), "then resumes alone")

	solo := batchTask("b", "s/")
	solo.Solo = true
	soloNext := &model.TaskFile{Tasks: []model.Task{running(batchTask("a", "x/")), solo, batchTask("c", "y/")}}
	assert.Empty(t, ImplementUnits(soloNext, 3, []string{"a"}), "a solo task next in line stops the refill")
	soloNext.Tasks[0].Status = model.StatusDone
	assert.Equal(t, []string{"b"}, ImplementUnits(soloNext, 3, nil), "and runs alone once the pool is empty")

	trailing := batchTask("z", "s/")
	trailing.Solo = true
	soloBehind := &model.TaskFile{Tasks: []model.Task{running(batchTask("a", "x/")), batchTask("c", "y/"), trailing}}
	assert.Equal(t, []string{"c"}, ImplementUnits(soloBehind, 3, []string{"a"}), "a solo task behind the next one is passed over, as before")
}
