package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A choose-one option shows its title only when it says more than its id:
// a title that is the id, or the id's words again in other case,
// punctuation or Turkish letters, is dropped; an empty title too.
func TestPure_AnOptionTitleThatRepeatsItsIdIsDropped(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([
  P.optionTitle("l1-tetigi", "L1 Tetiği"),
  P.optionTitle("iys", "İYS"),
  P.optionTitle("ab", "ab"),
  P.optionTitle("x", ""),
  P.optionTitle("cloud-a", "Bulut sağlayıcısı A"),
]);`)
	assert.JSONEq(t, `["", "", "", "", "Bulut sağlayıcısı A"]`, got)
}
