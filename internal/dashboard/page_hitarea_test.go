package dashboard_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A link button, an epic id among them, takes a press over at least 24 by
// 24 pixels (WCAG 2.5.8) while its text keeps the line it sits on: the hit
// area is a centred pseudo-element, not padding.
func TestPage_ALinkButtonHasA24PixelHitArea(t *testing.T) {
	t.Parallel()
	page, _ := pageText(t)
	for _, s := range []string{
		`.link { position: relative; border: 0; background: none; padding: 0;`,
		`.link::after { content: ""; position: absolute; left: 50%; top: 50%; width: max(100%, 24px); height: max(100%, 24px); transform: translate(-50%, -50%); }`,
	} {
		assert.Contains(t, page, s)
	}
}
