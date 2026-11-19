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
