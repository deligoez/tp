package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A Copy command button is a 24 pixel icon with no visible text: its name
// and tooltip say what it does, "Copy command" or which option it copies,
// and the command itself is only what it copies.
func TestPage_ACopyCommandButtonIsAnIconWithANameAndNoText(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `const copyCmd = (cmd, label) => '<button type="button" class="copy" data-copy="' + esc(cmd) + '" title="' + esc(label || t("copy.command")) + '" aria-label="' + esc(label || t("copy.command")) + '"></button>';`)
	for _, s := range []string{
		`.copy { display: inline-grid; place-items: center; width: 24px; height: 24px; margin: -4px 0; padding: 0; border: 0; border-radius: var(--r-1); background: transparent; color: var(--muted); vertical-align: middle; }`,
		`.copy::before { content: ""; width: 16px; height: 16px; background: currentColor; -webkit-mask: var(--ic-copy) center / contain no-repeat; mask: var(--ic-copy) center / contain no-repeat; }`,
		`.copy.done::before { -webkit-mask-image: var(--ic-check); mask-image: var(--ic-check); }`,
		`--ic-copy: url(`,
		`--ic-check: url(`,
	} {
		assert.Contains(t, page, s)
	}
}

// A copied command turns its icon into a check for a moment and says so
// through a polite live region, so a screen reader hears it too.
func TestPage_ACopiedCommandShowsACheckAndIsAnnounced(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, page, `<p id="announce" class="vh" aria-live="polite"></p>`)
	assert.Contains(t, page, `.vh { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }`)
	for _, s := range []string{
		`$("announce").textContent = ok ? t("copy.copied") : t("copy.failed");`,
		`b.classList.add("done");`,
		`setTimeout(() => b.classList.remove("done"), 1500);`,
	} {
		assert.Contains(t, script, s)
	}
}

// A Copy command sits at the end of its row's text line, never on a line of
// its own: a condition's, a gate's and a threshold's in the row's title, a
// choose-one option's after its title, a slice's after its label.
func TestPage_ACopyCommandSitsAtTheEndOfItsRowsLine(t *testing.T) {
	t.Parallel()
	_, script := pageText(t)
	for _, s := range []string{
		`const row = (id, title, meta, why, epic, act) => '<div class="row"><span class="id">' + (epic ? epicLink(id) : esc(id)) + '</span><span class="t">' + esc(title) + (act ? " " + act : "") +`,
		`row(id, titleOf(v, id), t("op.condition"), "", false, done(id))`,
		`row(gate.milestone, gate.threshold, t("op.gate"), "", false, copyCmd("tp epic gate " + gate.milestone + " --met" + evidence))`,
		`row(r.milestone, r.threshold, t("op.threshold"), "", false, copyCmd(r.command))`,
		`return '<li class="opt">' + epicLink(m) + (ot ? '<span class="ot" title="' + esc(ot) + '">' + esc(ot) + "</span>" : "") + done(m, t("op.copyFor", { id: m })) + "</li>";`,
		`const sliceRow = (label, meta, act) => '<div class="row rwide"><span class="t">' + esc(label) + " " + act + '</span><span class="m">' + esc(meta) + "</span></div>";`,
	} {
		assert.Contains(t, script, s)
	}
}
