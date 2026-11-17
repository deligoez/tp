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
