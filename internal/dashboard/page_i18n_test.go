package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPure_TheTranslatorFillsPluralsAndNumbersByLocale: t reads a key's
// text from its dictionary and fills its {name} slots, a number formatted
// for the locale; a key whose text has one and other forms takes one for
// exactly one in English, while Turkish, whose noun stays singular after a
// number, has one form; t.html escapes the values it fills and keeps the
// text's own markup; a key the dictionary lacks reads as the key, so a gap
// shows instead of a blank (1.17.0).
func TestPure_TheTranslatorFillsPluralsAndNumbersByLocale(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const en = P.translator({ hi: "hi {who}", n: { one: "{n} epic", other: "{n} epics" }, h: "<code>{x}</code>" }, "en");
const tr = P.translator({ n: "{n} epic" }, "tr");
out([en("hi", { who: "<b>" }), en("n", { n: 1 }), en("n", { n: 12345 }), en("missing"), en.html("h", { x: "<b>" }), tr("n", { n: 1 }), tr("n", { n: 12345 }), en.locale, tr.locale]);`)
	assert.JSONEq(t, `["hi <b>", "1 epic", "12,345 epics", "missing", "<code>&lt;b&gt;</code>", "1 epic", "12.345 epic", "en", "tr"]`, got)
}

