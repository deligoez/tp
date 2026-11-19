package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPage_TheGraphToolbarCarriesNoDuplicateFitOrKeys is the simplify
// note's pass 2: the canvas's zoom group already holds Fit and the open
// legend already lists the keys, so the toolbar repeats neither. F still
// fits, from the canvas and from the page.
func TestPage_TheGraphToolbarCarriesNoDuplicateFitOrKeys(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	for _, s := range []string{`id="gc-fit"`, `data-info="keys"`, `gkeys`, `"graph.keysLabel"`, `"info.keys"`} {
		assert.NotContains(t, page+script, s)
	}
	assert.Contains(t, page, `id="gc-zfit"`)
	assert.Contains(t, script, `} else if (ev.key === "f" || ev.key === "F") fit();`)
}
