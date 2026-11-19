package dashboard_test

import (
	"regexp"
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
