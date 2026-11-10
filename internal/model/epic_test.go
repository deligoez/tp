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

// TestEpicDep_TheMemberSetIsAGroupsIdentity: two groups naming the same
// members in another order, or with a repeat, are one group; a different
// set, or a plain edge, is not.
func TestEpicDep_TheMemberSetIsAGroupsIdentity(t *testing.T) {
	g := EpicDep{AnyOf: []string{"b", "a"}, Why: "w"}
	assert.Equal(t, []string{"a", "b"}, g.MemberSet())
	assert.True(t, g.SameGroup(EpicDep{AnyOf: []string{"a", "b", "a"}, Why: "other"}))
	assert.False(t, g.SameGroup(EpicDep{AnyOf: []string{"a", "c"}, Why: "w"}))
	assert.False(t, g.SameGroup(EpicDep{ID: "a", Why: "w"}))
	assert.False(t, EpicDep{ID: "a"}.SameGroup(EpicDep{ID: "a"}), "plain edges are no groups")
}

// TestEpicDep_NamesListsEveryIDTheEntryNames: a plain edge names its id, a
// group its distinct members, and an entry that wrongly carries both names
// the id and then each member once.
func TestEpicDep_NamesListsEveryIDTheEntryNames(t *testing.T) {
	assert.Equal(t, []string{"a"}, EpicDep{ID: "a"}.Names())
	assert.Equal(t, []string{"b", "c"}, EpicDep{AnyOf: []string{"b", "c", "b"}}.Names())
	assert.Equal(t, []string{"a", "b"}, EpicDep{ID: "a", AnyOf: []string{"a", "b"}}.Names())
	assert.Empty(t, EpicDep{}.Names())
}
