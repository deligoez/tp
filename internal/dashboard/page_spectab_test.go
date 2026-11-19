package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The Spec tab with nothing open opens the project's spec when it has
// exactly one; with none or several there is nothing to pick for the
// person.
func TestPure_TheSpecTabOpensTheOnlySpec(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.onlySpec([{ path: "spec/a.tasks.json" }]), P.onlySpec([{ path: "a" }, { path: "b" }]), P.onlySpec([]), P.onlySpec(undefined)]);`)
	assert.JSONEq(t, `["spec/a.tasks.json", null, null, null]`, got)
}
