package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// TestGateMembers_DefaultSkipsConditionsAndDeferrable is spec/1.5.0.md §10
// row 9: the chain plus a condition and a deferrable epic in A. With a gates
// entry that names no members, A's members are its work epics: the condition
// is not one, and the deferrable epic does not hold the gate. A default that
// counted conditions would list "go".
func TestGateMembers_DefaultSkipsConditionsAndDeferrable(t *testing.T) {
	g := epicChainGraph()
	g.Epics = append(g.Epics,
		model.Epic{ID: "go", Title: "Go decision", Milestone: "A", Condition: true},
		model.Epic{ID: "later", Title: "Later step", Milestone: "A"},
		model.Epic{ID: "elsewhere", Title: "Other step", Milestone: "B"},
	)
	g.Gates = map[string]model.EpicGate{"A": {Deferrable: []string{"later"}}}

	members, deferrable := GateMembers(g, "A")

	assert.Equal(t, []string{"a", "b", "c", "d"}, members)
	assert.Equal(t, []string{"later"}, deferrable)

	delete(g.Gates, "A")
	members, deferrable = GateMembers(g, "A")
	assert.Equal(t, []string{"a", "b", "c", "d", "later"}, members, "with no gates entry, only conditions are left out")
	assert.Empty(t, deferrable)
}

