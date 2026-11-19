package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPage_TheSectionChipClearsWithTheXIcon: the section filter chip's clear
// button draws the --ic-x mask at the small icon size, like every other
// close, and carries no typed glyph; its accessible name stays.
func TestPage_TheSectionChipClearsWithTheXIcon(t *testing.T) {
	t.Parallel()
	page, script := pageText(t)
	assert.Contains(t, script, `'<button type="button" class="key secx" data-secclear aria-label="' + esc(t("tasks.showAll")) + '"></button></span>'`)
	assert.NotContains(t, script, `data-secclear aria-label="' + esc(t("tasks.showAll")) + '">×</button>`)
	assert.Contains(t, page, `.secx::before { content: ""; width: 16px; height: 16px; background: currentColor; -webkit-mask: var(--ic-x) center / contain no-repeat; mask: var(--ic-x) center / contain no-repeat; }`)
}
