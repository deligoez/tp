package dashboard_test

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// served is one page file as the handler serves it.
func served(t *testing.T, path string) []byte {
	t.Helper()
	resp := serve(t, http.MethodGet, "127.0.0.1:7377", path)
	require.Equal(t, http.StatusOK, resp.StatusCode, path)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return body
}

// dictionary is one language file, each key's text or plural forms.
func dictionary(t *testing.T, lang string) map[string]json.RawMessage {
	t.Helper()
	var d map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(served(t, "/i18n/"+lang+".json"), &d), lang)
	return d
}

// slotsOf is the {name} slots of a key's text, every plural form's together,
// and fails unless the text is a string or one and other forms.
func slotsOf(t *testing.T, lang, key string, raw json.RawMessage) []string {
	t.Helper()
	var texts []string
	var one string
	if json.Unmarshal(raw, &one) == nil {
		texts = []string{one}
	} else {
		var forms map[string]string
		require.NoError(t, json.Unmarshal(raw, &forms), "%s %s is a string or { one, other }", lang, key)
		require.Contains(t, forms, "other", "%s %s has an other form", lang, key)
		for form, text := range forms {
			require.Contains(t, []string{"one", "other"}, form, "%s %s", lang, key)
			texts = append(texts, text)
		}
	}
	var slots []string
	for _, text := range texts {
		assert.NotEmpty(t, text, "%s %s", lang, key)
		for _, m := range regexp.MustCompile(`\{(\w+)\}`).FindAllStringSubmatch(text, -1) {
			if !slices.Contains(slots, m[1]) {
				slots = append(slots, m[1])
			}
		}
	}
	slices.Sort(slots)
	return slots
}

// usedKeys is every key the page names: t("…"), t.html("…") and tk("…") in
// its scripts, data-i18n and data-i18n-<attr> in its markup.
func usedKeys(t *testing.T) map[string]bool {
	t.Helper()
	used := map[string]bool{}
	call := regexp.MustCompile(`\b(?:t|t\.html|tk)\("([^"]+)"`)
	for _, path := range []string{"/pure.js", "/app.js", "/graph.js"} {
		for _, m := range call.FindAllStringSubmatch(string(served(t, path)), -1) {
			used[m[1]] = true
		}
	}
	attr := regexp.MustCompile(`data-i18n(?:-[a-z-]+)?="([^"]+)"`)
	for _, m := range attr.FindAllStringSubmatch(string(served(t, "/")), -1) {
		used[m[1]] = true
	}
	return used
}

// TestPage_BothLanguagesHoldTheSameKeysAndThePageUsesEachOne: en.json and
// tr.json hold exactly the same keys with the same {name} slots, every key
// the page's scripts and markup name is in en.json, and every key en.json
// holds is named, so neither language can lose a word or keep a dead one
// (1.17.0).
func TestPage_BothLanguagesHoldTheSameKeysAndThePageUsesEachOne(t *testing.T) {
	t.Parallel()
	en := dictionary(t, "en")
	tr := dictionary(t, "tr")
	for key, raw := range en {
		other, ok := tr[key]
		if !assert.True(t, ok, "tr.json lacks %s", key) {
			continue
		}
		assert.Equal(t, slotsOf(t, "en", key, raw), slotsOf(t, "tr", key, other), "the slots of %s", key)
	}
	for key := range tr {
		assert.Contains(t, en, key, "en.json lacks %s, which tr.json holds", key)
	}
	used := usedKeys(t)
	for key := range used {
		assert.Contains(t, en, key, "the page names %s, which en.json lacks", key)
	}
	for key := range en {
		assert.True(t, used[key], "en.json holds %s, which the page never names", key)
	}
}

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

// TestPure_TheTranslatorFormatsPercentsAndDurationsByLocale: a share reads
// as the locale writes a percent (40% in English, %40 in Turkish), and a
// task's minutes as minutes under an hour, else hours to one decimal.
func TestPure_TheTranslatorFormatsPercentsAndDurationsByLocale(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const en = P.translator({}, "en");
const tr = P.translator({}, "tr");
out([en.num(12345), tr.num(12345), en.pct(0.4), tr.pct(0.4), en.dur(45), en.dur(90), tr.dur(45), tr.dur(90)]);`)
	assert.JSONEq(t, `["12,345", "12.345", "40%", "%40", "45 min", "1.5 hr", "45 dk.", "1,5 sa."]`, got)
}

// TestPure_TheLanguageIsTheStoredChoiceElseTheBrowsers: a stored choice of
// en or tr wins; without one, a browser language starting tr reads Turkish
// and any other English (1.17.0).
func TestPure_TheLanguageIsTheStoredChoiceElseTheBrowsers(t *testing.T) {
	t.Parallel()
	got := runPure(t, `out([P.pickLang("tr", "en-US"), P.pickLang("en", "tr-TR"), P.pickLang(null, "tr-TR"), P.pickLang(null, "tr"), P.pickLang("de", "TR"), P.pickLang(null, "en-GB"), P.pickLang(null, "trk"), P.pickLang(null, undefined)]);`)
	assert.JSONEq(t, `["tr", "en", "tr", "tr", "tr", "en", "en", "en"]`, got)
}
