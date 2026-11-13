package engine

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForestLayout_GoldenTreeColumns pins the tree packing on lulus's
// sample: a diamond with a tail and a side branch in one group, two lone
// nodes in another. Leaves take the next column, a parent sits centred
// over its children, the diamond's second edge in is a cross edge, and the
// second group starts forestGroupGap columns after the first.
func TestForestLayout_GoldenTreeColumns(t *testing.T) {
	t.Parallel()
	ids := []string{"a1", "a2", "a3", "a4", "a5", "a6", "b1", "b2"}
	edges := []LayoutEdge{{"a1", "a2"}, {"a1", "a3"}, {"a2", "a4"}, {"a3", "a4"}, {"a4", "a5"}, {"a1", "a6"}}
	groupOf := map[string]string{"a1": "a", "a2": "a", "a3": "a", "a4": "a", "a5": "a", "a6": "a", "b1": "b", "b2": "b"}
	got := ForestLayout(ids, edges, nil, nil, groupOf, PackTree)
	assert.Equal(t, Forest{
		Layers:  4,
		Columns: 8.5,
		Roles:   []EdgeRole{RoleTree, RoleTree, RoleTree, RoleCross, RoleTree, RoleTree},
		Places: map[string]ForestPlace{
			"a1": {0, 1}, "a2": {1, 0}, "a3": {1, 1}, "a6": {1, 2}, "a4": {2, 0}, "a5": {3, 0},
			"b1": {0, 6}, "b2": {0, 7.5},
		},
	}, got)
}

