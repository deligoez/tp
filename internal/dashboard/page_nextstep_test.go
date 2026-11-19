package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The next-step sentence sits under a label that already says Next or Next
// step (the Spec view's row, the Specs table's column; Sıradaki adım in
// Turkish), so it names the step alone: neither "next" nor "step" twice.
func TestPage_TheNextStepSentenceDoesNotRepeatItsLabel(t *testing.T) {
	t.Parallel()
	for lang, want := range map[string]string{"en": `"{phase}"`, "tr": `"{phase}"`} {
		assert.Equal(t, want, string(dictionary(t, lang)["specs.nextIn"]), lang)
	}
}
