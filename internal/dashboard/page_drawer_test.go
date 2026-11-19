package dashboard_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An epic's description reads first: the drawer draws it above its fields,
// Acceptance among them.
func TestPage_TheDrawerDrawsTheDescriptionAboveAcceptance(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	desc := strings.Index(script, `(String(ge.description || "").trim() ? descHTML(ge.description) : "")`)
	fields := strings.Index(script, `"<dl>" + fields.map(`)
	if assert.Positive(t, desc, "the drawer draws the description") && assert.Positive(t, fields, "the drawer draws its fields") {
		assert.Less(t, desc, fields, "the description comes before the fields, Acceptance among them")
	}
}
