package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The drawer is wide only for a stored "wide"; nothing stored, a cleared
// store or any other value keeps it narrow.
func TestPure_TheDrawerIsWideOnlyForAStoredWide(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.drawerWide("wide"), P.drawerWide("narrow"), P.drawerWide(null), P.drawerWide("1")]);`)
	assert.JSONEq(t, `[true, false, false, false]`, got)
}
