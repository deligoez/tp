package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestIsMechanizedClass_ValidityIsPerEntry covers test 34's predicate half:
// validity is judged per entry, so an array holding one rejected entry and one
// valid entry mechanizes the valid entry's class and leaves the rejected entry's
// class unmechanized. An implementation validating the whole slice mechanizes
// neither and fails here.
func TestIsMechanizedClass_ValidityIsPerEntry(t *testing.T) {
	checks := []model.Check{
		{Class: "empty-cmd", Cmd: "   "},       // rejected: blank cmd, well-formed class
		{Class: "duplicate-line", Cmd: "true"}, // accepted on its own
	}

	require.Error(t, ValidateChecks(checks),
		"the slice-level call rejects this array outright — the state test 34 turns on")

	assert.True(t, IsMechanizedClass(checks, "duplicate-line"),
		"a valid entry mechanizes its class even beside a rejected one")
	assert.False(t, IsMechanizedClass(checks, "empty-cmd"),
		"an entry the validator rejects mechanizes nothing")
	assert.Equal(t, []string{"duplicate-line"}, ReviewerExclusionClasses(checks),
		"the exclusion list carries the valid entry's class alone")
}

// TestIsMechanizedClass_ExactMatch pins §3.1's byte-for-byte comparison: the
// varying side is the candidate class a reviewer wrote, which is unconstrained.
func TestIsMechanizedClass_ExactMatch(t *testing.T) {
	checks := []model.Check{{Class: "duplicate-line", Cmd: "true"}}

	assert.True(t, IsMechanizedClass(checks, "duplicate-line"))
	assert.False(t, IsMechanizedClass(checks, "Duplicate-Line"), "no case folding")
	assert.False(t, IsMechanizedClass(checks, " duplicate-line "), "no trimming")
	assert.False(t, IsMechanizedClass(checks, ""), "an empty candidate class matches nothing")
	assert.False(t, IsMechanizedClass(nil, "duplicate-line"), "no registered check mechanizes nothing")
}

// TestMechanized_ClassNamedByTwoEntries covers test 35's predicate half: the
// validator's duplicate-class rule is cross-entry and unreachable per entry, so
// a class named twice is simply registered, and the reviewer exclusion list names
// it once. The other list test 35 names, mechanized_classes, is pinned by
// TestReviewRecord_ClassNamedByTwoEntriesWithheldOnce in internal/cli.
func TestMechanized_ClassNamedByTwoEntries(t *testing.T) {
	checks := []model.Check{
		{Class: "duplicate-line", Cmd: "check-a"},
		{Class: "duplicate-line", Cmd: "check-b"},
	}

	require.Error(t, ValidateChecks(checks),
		"the duplicate-class rule fires only over the slice")

	assert.True(t, IsMechanizedClass(checks, "duplicate-line"),
		"a class named by two entries is registered, not rejected")
	assert.Equal(t, []string{"duplicate-line"}, ReviewerExclusionClasses(checks),
		"the exclusion list names it once")
}

// TestMechanized_OverSpecificationExemptionIsScoped covers test 36's predicate
// half: over-specification is mechanized for candidate suppression like any other
// class, and never joins the reviewer exclusion list. An implementation exempting
// it everywhere leaves the register-a-check hint firing forever; one exempting it
// nowhere ships a prompt that both demands and forbids the class.
func TestMechanized_OverSpecificationExemptionIsScoped(t *testing.T) {
	checks := []model.Check{
		{Class: OverSpecificationClass, Cmd: "true"},
		{Class: "naming", Cmd: "true"},
	}

	assert.True(t, IsMechanizedClass(checks, OverSpecificationClass),
		"mechanized for candidate suppression like any other class")
	assert.Equal(t, []string{"naming"}, ReviewerExclusionClasses(checks),
		"the exemption is scoped to the reviewer exclusion list")
}

// TestReviewerExclusionClasses_FilterOrderAndShape pins the §3.2 membership
// order and the emitted shape: invalid entries are dropped before duplicates
// collapse, registration order survives, and the result is never nil.
func TestReviewerExclusionClasses_FilterOrderAndShape(t *testing.T) {
	shadowed := []model.Check{
		{Class: "duplicate-line", Cmd: "   "},  // rejected
		{Class: "duplicate-line", Cmd: "true"}, // valid, same class
	}
	assert.Equal(t, []string{"duplicate-line"}, ReviewerExclusionClasses(shadowed),
		"collapsing before dropping invalid entries would keep the rejected one and lose the class")

	ordered := []model.Check{
		{Class: "zeta-check", Cmd: "true"},
		{Class: "alpha-check", Cmd: "true"},
	}
	assert.Equal(t, []string{"zeta-check", "alpha-check"}, ReviewerExclusionClasses(ordered),
		"registration order, not sorted")

	allInvalid := ReviewerExclusionClasses([]model.Check{{Class: "Bad_Slug", Cmd: "true"}})
	assert.Empty(t, allInvalid, "every entry invalid leaves nothing to list")
	assert.NotNil(t, allInvalid, "the list is a slice, never nil")
	assert.NotNil(t, ReviewerExclusionClasses(nil), "no registered checks still yields a slice")
}
