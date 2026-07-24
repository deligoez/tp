package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScopeFenceText_StatesEveryProhibition(t *testing.T) {
	got := ScopeFenceText()

	// §7.1: each prohibition the fence forbids, plus the out-of-scope escape
	// hatch the brief must name so a unit knows where to put a finding.
	for _, want := range []string{
		"acceptance criteria",
		"refactor",
		"rename",
		"reformat",
		`"clean up"`,
		"task file",
		".tp-review/",
		`"Out of scope:"`,
	} {
		assert.Contains(t, got, want)
	}
	assert.NotContains(t, got, "fixing it outside", "the fence forbids fixing, not only fixing outside")
}

func TestExtractOutOfScope(t *testing.T) {
	t.Run("present returns note without prefix", func(t *testing.T) {
		reason := "- model at internal/model/task.go:18\n- tests green\nOut of scope: typo in README.md"
		assert.Equal(t, "typo in README.md", ExtractOutOfScope(reason))
	})

	t.Run("absent returns empty", func(t *testing.T) {
		reason := "- model added\n- tests green"
		assert.Equal(t, "", ExtractOutOfScope(reason))
	})

	t.Run("empty reason returns empty", func(t *testing.T) {
		assert.Equal(t, "", ExtractOutOfScope(""))
	})

	t.Run("prefix only returns empty", func(t *testing.T) {
		assert.Equal(t, "", ExtractOutOfScope("- done\nOut of scope:"))
	})

	t.Run("leading spaces after prefix trimmed", func(t *testing.T) {
		assert.Equal(t, "note here", ExtractOutOfScope("Out of scope:   note here"))
	})

	t.Run("not matched when embedded mid line", func(t *testing.T) {
		// The line must BEGIN with the prefix; a mention inside another line
		// is not the out-of-scope escape hatch.
		reason := "- evidence line\n- saw Out of scope: inline in docs"
		assert.Equal(t, "", ExtractOutOfScope(reason))
	})
}
