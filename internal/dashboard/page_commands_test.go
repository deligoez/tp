package dashboard_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// tpCommand matches a tp command written out: tp, a space, a subcommand.
var tpCommand = regexp.MustCompile(`\btp (epic|export|next|resume|status|claim|done|review|audit|init|add)\b`)

// TestPage_NoTpCommandShowsInPlainView is the operator's simplify-pass
// rule: agents run tp, so the page a person reads says what happens in
// plain words and never shows a command in plain view. A command still
// useful to a person sits behind a Copy command button (the command in its
// tooltip and clipboard) or in an (i) note. These keys are the text the
// page shows without a click, in both languages.
func TestPage_NoTpCommandShowsInPlainView(t *testing.T) {
	t.Parallel()
	shown := []string{
		"epics.none.body_html", "slices.none_html", "drawer.conditionNote_html", "task.epicWaits_html",
		"spec.next_html", "spec.readFailed", "spec.reading", "source.epicExport", "source.exportList",
		"hist.noKeyHint", "src.sliceHistory", "src.epicHistory", "src.spec",
	}
	for _, lang := range []string{"en", "tr"} {
		dict := dictionary(t, lang)
		for _, key := range shown {
			raw, ok := dict[key]
			if assert.True(t, ok, "%s has %s", lang, key) {
				assert.NotRegexp(t, tpCommand, string(raw), "%s %s", lang, key)
			}
		}
	}
	_, script := pageText(t)
	assert.Contains(t, script, `const copyCmd = (cmd) => '<button type="button" class="key copy" data-copy="' + esc(cmd) + '" title="' + esc(cmd) + '">' + esc(t("copy.command")) + "</button>";`)
	for _, s := range []string{`"<code>" + esc(cmd)`, `'<code class="snext">'`, `what: "tp `} {
		assert.NotContains(t, script, s, "no command is drawn as visible text")
	}
}

// TestPage_TheCommandsMovedOutOfViewAreOneClickAway: each command the
// rule took out of plain view is still reachable: the next action, a
// task's claim or close and a condition's settling command behind Copy
// command; starting a graph and reviewing a slice in an (i) note.
func TestPage_TheCommandsMovedOutOfViewAreOneClickAway(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{
		`(cmd ? copyCmd(cmd) : "")`,
		`copyCmd("tp epic done " + id + ' --evidence "<' + t("op.evidence") + '>"')`,
		`nograph: tk("info.nograph"),`,
	} {
		assert.Contains(t, script, s)
	}
	for _, lang := range []string{"en", "tr"} {
		dict := dictionary(t, lang)
		assert.Contains(t, string(dict["info.nograph"]), "tp epic set milestones=", lang)
		assert.Contains(t, string(dict["info.slices"]), "tp epic review --milestone", lang)
		assert.NotEmpty(t, dict["copy.command"], lang)
	}
}

// TestPage_EveryCopyCommandButtonIsHandled: a Copy command button can be
// drawn in any container (the drawer, the operator list, a task, the spec
// head), so one document-level delegate copies every [data-copy] and no
// container keeps a handler of its own that a new container would lack.
func TestPage_EveryCopyCommandButtonIsHandled(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	assert.Contains(t, script, "document.addEventListener(\"click\", (ev) => {\n    const b = ev.target.closest && ev.target.closest(\"[data-copy]\");\n    if (b) copyKey(b);\n  });")
	assert.Equal(t, 1, strings.Count(script, `closest("[data-copy]")`), "one delegate copies every button")
}

// TestPure_ASliceLabelIsPlainWordsAndOnlyItsCommandCarriesFlags: a review
// slice is named in words on the page ("milestone M1", "epics a, b"); the
// flag form tp epic review takes lives only in the Copy command a slice
// that can take another round offers.
func TestPure_ASliceLabelIsPlainWordsAndOnlyItsCommandCarriesFlags(t *testing.T) {
	t.Parallel()
	got := runPure(t, `const tr = (k, p) => k + " " + JSON.stringify(p || {});
const ss = [
  { key: { kind: "milestone", value: "M1" }, dir: "m-M1" },
  { key: { kind: "epic", value: "billing" }, dir: "e-billing" },
  { key: { kind: "epics", ids: ["a", "b"], name: "x" }, dir: "s-x" },
  { key: { kind: "epics", ids: ["a", "b"] }, dir: "s-ab" },
  { dir: "legacy-dir" },
];
out({ labels: ss.map((s) => P.sliceLabel(s, tr)), args: ss.map((s) => P.sliceArgs(s)) });`)
	assert.JSONEq(t, `{
  "labels": ["slice.milestone {\"name\":\"M1\"}", "slice.epic {\"id\":\"billing\"}", "slice.set {\"name\":\"x\"}", "slice.epics {\"ids\":\"a, b\"}", "legacy-dir"],
  "args": ["--milestone M1", "--epic billing", "--epics a,b --name x", "--epics a,b", "legacy-dir"]
}`, got)
	_, script := pageText(t)
	assert.NotContains(t, script, "function sliceLabel(", "the page names slices through tpPure only")
	assert.Contains(t, script, `copyCmd("tp epic review " + tpPure.sliceArgs(s) + " --grant-round")`)
	for _, lang := range []string{"en", "tr"} {
		dict := dictionary(t, lang)
		for _, k := range []string{"slice.milestone", "slice.epic", "slice.epics"} {
			if assert.Contains(t, dict, k, lang) {
				assert.NotContains(t, string(dict[k]), "--", "%s %s", lang, k)
			}
		}
	}
}
