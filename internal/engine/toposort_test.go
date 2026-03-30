package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

func TestTopoSort_EmptyInput(t *testing.T) {
	result := engine.TopoSort(nil)
	assert.Empty(t, result)
}

func TestTopoSort_SingleOpenTask(t *testing.T) {
	tasks := []model.Task{
		{ID: "a", Status: model.StatusOpen, DependsOn: []string{}},
	}

	result := engine.TopoSort(tasks)

	require.Len(t, result, 1)
	assert.Equal(t, "a", result[0].ID)
}

func TestTopoSort_DoneTasksExcluded(t *testing.T) {
	tasks := []model.Task{
		{ID: "a", Status: model.StatusDone, DependsOn: []string{}},
		{ID: "b", Status: model.StatusOpen, DependsOn: []string{}},
	}

	result := engine.TopoSort(tasks)

	require.Len(t, result, 1)
	assert.Equal(t, "b", result[0].ID)
}

func TestTopoSort_WIPTasksAppearFirst(t *testing.T) {
	tasks := []model.Task{
		{ID: "a", Status: model.StatusOpen, DependsOn: []string{}},
		{ID: "b", Status: model.StatusWIP, DependsOn: []string{}},
		{ID: "c", Status: model.StatusOpen, DependsOn: []string{}},
	}

	result := engine.TopoSort(tasks)

	require.Len(t, result, 3)
	assert.Equal(t, "b", result[0].ID, "WIP task should be first")
}

func TestTopoSort_LinearChain(t *testing.T) {
	tasks := []model.Task{
		{ID: "c", Status: model.StatusOpen, DependsOn: []string{"b"}},
		{ID: "a", Status: model.StatusOpen, DependsOn: []string{}},
		{ID: "b", Status: model.StatusOpen, DependsOn: []string{"a"}},
	}

	result := engine.TopoSort(tasks)

	require.Len(t, result, 3)

	// Extract IDs for order verification
	ids := make([]string, len(result))
	for i, task := range result {
		ids[i] = task.ID
	}

	// a must come before b, b must come before c
	posA, posB, posC := indexOf(ids, "a"), indexOf(ids, "b"), indexOf(ids, "c")
	assert.Less(t, posA, posB, "a should come before b")
	assert.Less(t, posB, posC, "b should come before c")
}

func TestTopoSort_Diamond(t *testing.T) {
	// A -> B -> D
	// A -> C -> D
	tasks := []model.Task{
		{ID: "d", Status: model.StatusOpen, DependsOn: []string{"b", "c"}},
		{ID: "b", Status: model.StatusOpen, DependsOn: []string{"a"}},
		{ID: "c", Status: model.StatusOpen, DependsOn: []string{"a"}},
		{ID: "a", Status: model.StatusOpen, DependsOn: []string{}},
	}

	result := engine.TopoSort(tasks)

	require.Len(t, result, 4)

	ids := make([]string, len(result))
	for i, task := range result {
		ids[i] = task.ID
	}

	posA := indexOf(ids, "a")
	posB := indexOf(ids, "b")
	posC := indexOf(ids, "c")
	posD := indexOf(ids, "d")

	assert.Less(t, posA, posB, "a should come before b")
	assert.Less(t, posA, posC, "a should come before c")
	assert.Less(t, posB, posD, "b should come before d")
	assert.Less(t, posC, posD, "c should come before d")
	assert.Equal(t, len(result)-1, posD, "d should be last")
}

func TestTopoSort_MixedStatuses(t *testing.T) {
	tasks := []model.Task{
		{ID: "done1", Status: model.StatusDone, DependsOn: []string{}},
		{ID: "wip1", Status: model.StatusWIP, DependsOn: []string{}},
		{ID: "open1", Status: model.StatusOpen, DependsOn: []string{"done1"}},
		{ID: "open2", Status: model.StatusOpen, DependsOn: []string{"open1"}},
	}

	result := engine.TopoSort(tasks)

	// done1 excluded
	ids := make([]string, len(result))
	for i, task := range result {
		ids[i] = task.ID
	}

	assert.NotContains(t, ids, "done1", "done tasks should be excluded")
	assert.Contains(t, ids, "wip1")
	assert.Contains(t, ids, "open1")
	assert.Contains(t, ids, "open2")

	// WIP first
	assert.Equal(t, "wip1", ids[0], "WIP task should be first")

	// open1 before open2 (dependency)
	posOpen1 := indexOf(ids, "open1")
	posOpen2 := indexOf(ids, "open2")
	assert.Less(t, posOpen1, posOpen2, "open1 should come before open2")
}

func indexOf(slice []string, item string) int {
	for i, v := range slice {
		if v == item {
			return i
		}
	}
	return -1
}
