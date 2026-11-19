package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A done card keeps its tint under hover and press: the hover layer is
// drawn over the done tint, not in place of it.
func TestPage_ADoneCardKeepsItsTintOnHover(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	assert.Contains(t, page, `.tcard[data-s="done"]:hover { background-image: linear-gradient(var(--fill-hover), var(--fill-hover)), linear-gradient(var(--fill-weak), var(--fill-weak)); }`)
	assert.Contains(t, page, `.tcard[data-s="done"]:active { background-image: linear-gradient(var(--fill-press), var(--fill-press)), linear-gradient(var(--fill-weak), var(--fill-weak)); }`)
}
