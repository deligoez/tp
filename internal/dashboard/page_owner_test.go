package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ownersJS is a declared owners set for the owner tests: a named owner, an
// unnamed one, and one whose handle is the word "unowned", which must stay
// an owner like any other (spec/backlog/epic-owner.md).
const ownersJS = `const owners = [{ handle: "ana", name: "Ana A." }, { handle: "ben" }, { handle: "unowned", name: "U" }];`

// TestPure_AnOwnerIsNamedSlottedAndSelectedByItsPlaceInTheSet: a label is
// the display name or the handle; a slot is the position in the declared
// set; a selection is a declared index or null for the unowned, so a handle
// spelled "unowned" is selected as itself and never as the unowned.
func TestPure_AnOwnerIsNamedSlottedAndSelectedByItsPlaceInTheSet(t *testing.T) {
	t.Parallel()
	got := runPure(t, ownersJS+`
out([
  P.ownerLabel(owners, "ana"), P.ownerLabel(owners, "ben"), P.ownerLabel(owners, "zed"),
  P.ownerSlot(owners, "ana"), P.ownerSlot(owners, "unowned"), P.ownerSlot(owners, null), P.ownerSlot(owners, "zed"),
  P.ownerSelected(["unowned"], owners, 2), P.ownerSelected(["unowned"], owners, null), P.ownerSelected([], owners, null),
  P.ownerSelected(["ben", "ana"], owners, 0), P.ownerSelected(["ben"], owners, 0), P.ownerSelected(["ben"], owners, undefined),
]);`)
	assert.JSONEq(t, `["Ana A.", "ben", "zed", 0, 2, null, null, true, false, true, true, false, true]`, got)
}

// TestPure_TheOwnerLegendCountsByMembershipNeverByPaint: an epic with two
// owners counts under each though the map paints it in the first one's
// colour; owners past the eighth share one counted "other" row, counting
// each epic naming any of them once; the unowned are counted on their own.
func TestPure_TheOwnerLegendCountsByMembershipNeverByPaint(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const owners = Array.from({ length: 10 }, (_, i) => ({ handle: "h" + i }));
owners[0].name = "Zero";
const epics = [{ owner: ["h0"] }, { owner: ["h1", "h0"] }, { owner: ["h8"] }, { owner: ["h9", "h8"] }, {}, { owner: [] }];
out(P.ownerLegend(owners, epics));`)
	assert.JSONEq(t, `{"owners": [{"slot": 0, "label": "Zero", "n": 2}, {"slot": 1, "label": "h1", "n": 1}],
		"other": {"owners": 2, "n": 2}, "unowned": 2}`, got)
}

// TestPure_ConditionsGroupUnderEachOwnerTheUnownedLast: a condition with two
// owners is listed under each, so the groups can sum past the conditions and
// total counts them once; the unowned group comes last, keyed by null, and
// holds a condition naming only handles the set does not declare, so none
// is hidden; a handle spelled "unowned" is an owner's group.
func TestPure_ConditionsGroupUnderEachOwnerTheUnownedLast(t *testing.T) {
	t.Parallel()
	got := runPure(t, ownersJS+`
const own = { c1: ["ben", "ana"], c2: ["ana"], c3: [], c4: ["zed"], c5: ["unowned"] };
out(P.ownerGroups(owners, ["c1", "c2", "c3", "c4", "c5"], (id) => own[id]));`)
	assert.JSONEq(t, `{"total": 5, "groups": [
		{"owner": 0, "label": "Ana A.", "ids": ["c1", "c2"]},
		{"owner": 1, "label": "ben", "ids": ["c1"]},
		{"owner": 2, "label": "U", "ids": ["c5"]},
		{"owner": null, "label": null, "ids": ["c3", "c4"]}]}`, got)
}

// TestPure_UnownedMeansNoDeclaredOwnerEverywhere: an epic naming only a
// handle the set does not declare (a lint error the page still draws) is
// unowned to the legend and to the map's selection, as it is to the
// operator groups, so it is counted and lit once and never hidden.
func TestPure_UnownedMeansNoDeclaredOwnerEverywhere(t *testing.T) {
	t.Parallel()
	got := runPure(t, ownersJS+`
out([P.ownerLegend(owners, [{ owner: ["zed"] }, { owner: [] }]).unowned,
  P.ownerSelected(["zed"], owners, null), P.ownerSelected(["zed", "ben"], owners, null)]);`)
	assert.JSONEq(t, `[2, true, false]`, got)
}
