package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEpicDep_GroupMembersAreDistinctInStoredOrder: a plain edge is no group
// and has no members; a group, an empty any_of included, is one, and its
// members drop a repeat but keep the order they were stored in.
func TestEpicDep_GroupMembersAreDistinctInStoredOrder(t *testing.T) {
	plain := EpicDep{ID: "a", Why: "w"}
	assert.False(t, plain.IsGroup())
	assert.Nil(t, plain.Members())

	assert.True(t, EpicDep{AnyOf: []string{}, Why: "w"}.IsGroup(), "an empty any_of is a malformed group, not a plain edge")
	assert.Equal(t, []string{"c", "a", "b"}, EpicDep{AnyOf: []string{"c", "a", "c", "b"}, Why: "w"}.Members())
}
