package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFrontmatter_TPMapping(t *testing.T) {
	spec := `---
title: ignored top-level key
tp:
  domain: prose
  lens:
    all:
      - "Does any chapter summary leak a plot point?"
    implementer:
      - "Can each section be written without inventing facts?"
    architect: []
---
# Heading
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)
	assert.Equal(t, LineRange{Start: 1, End: 11}, fm.Lines)
	assert.Equal(t, "prose", fm.Domain)
	assert.Equal(t, []string{"Does any chapter summary leak a plot point?"}, fm.Lens["all"])
	assert.Equal(t, []string{"Can each section be written without inventing facts?"}, fm.Lens["implementer"])
	assert.Equal(t, []string{}, fm.Lens["architect"])
	assert.Empty(t, fm.Errors)
	assert.Empty(t, fm.Warnings)
}

func TestParseFrontmatter_StructuralFailures(t *testing.T) {
	t.Run("unterminated block treated as content with lint error", func(t *testing.T) {
		fm := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: prose\n# Heading\ncontent\n"))
		assert.False(t, fm.Present, "no frontmatter — all lines are content")
		assert.Equal(t, DomainSoftware, fm.Domain)
		require.Len(t, fm.Errors, 1)
		assert.Contains(t, fm.Errors[0].Message, "never closed")
	})

	t.Run("malformed YAML stays excluded with defaults", func(t *testing.T) {
		fm := ParseFrontmatterBytes([]byte("---\ntp: [unclosed\n---\n# Heading\n"))
		assert.True(t, fm.Present, "closed block stays excluded from parsers")
		assert.Equal(t, LineRange{Start: 1, End: 3}, fm.Lines)
		assert.Equal(t, DomainSoftware, fm.Domain)
		assert.Empty(t, fm.Lens)
		require.Len(t, fm.Errors, 1)
		assert.Contains(t, fm.Errors[0].Message, "YAML parse failed")
	})

	t.Run("no frontmatter at all", func(t *testing.T) {
		fm := ParseFrontmatterBytes([]byte("# Heading\ncontent\n"))
		assert.False(t, fm.Present)
		assert.Equal(t, DomainSoftware, fm.Domain)
		assert.Empty(t, fm.Errors)
	})
}

func TestParseFrontmatter_ShapeWarnings(t *testing.T) {
	spec := `---
tp:
  domain: 42
  lens:
    implementer: "not a list"
    tester:
      - "valid question"
      - 99
    mystery:
      - "unknown key"
---
content
`
	fm := ParseFrontmatterBytes([]byte(spec))
	require.True(t, fm.Present)
	assert.Equal(t, DomainSoftware, fm.Domain, "non-string domain falls back")
	assert.NotContains(t, fm.Lens, "implementer", "non-list value ignored")
	assert.Equal(t, []string{"valid question"}, fm.Lens["tester"], "non-string element ignored")
	assert.NotContains(t, fm.Lens, "mystery", "unknown key ignored")

	msgs := make([]string, 0, len(fm.Warnings))
	for _, w := range fm.Warnings {
		msgs = append(msgs, w.Message)
	}
	var joined strings.Builder
	for _, m := range msgs {
		joined.WriteString(m + "\n")
	}
	assert.Contains(t, joined.String(), "tp.domain is not a string")
	assert.Contains(t, joined.String(), "tp.lens.implementer is not a list")
	assert.Contains(t, joined.String(), "tp.lens.tester[1] is not a string")
	assert.Contains(t, joined.String(), `tp.lens key "mystery" is unknown`)
}

// TestParseFrontmatter_UnknownTPKey covers a mistyped key directly under tp:.
// It is warned about and ignored, and the warning names every key the parser
// accepts, so a typo'd review_roles cannot silently leave a spec unconfigured.
func TestParseFrontmatter_UnknownTPKey(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: prose\n  review_role:\n    implementer:\n      focus: [\"q\"]\n  lenz: {}\n---\n"))
	assert.Equal(t, "prose", fm.Domain, "the known sibling still parses")
	assert.Empty(t, fm.ReviewRoles, "the mistyped key configures nothing")
	msgs := make([]string, 0, len(fm.Warnings))
	for _, w := range fm.Warnings {
		msgs = append(msgs, w.Message)
	}
	assert.Equal(t, []string{
		`tp key "lenz" is unknown (known: domain, lens, review_roles, audit_roles); ignored`,
		`tp key "review_role" is unknown (known: domain, lens, review_roles, audit_roles); ignored`,
	}, msgs, "one warning per unknown key, sorted")

	all := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: prose\n  lens: {}\n  review_roles: {}\n  audit_roles: {}\n---\n"))
	assert.Empty(t, all.Warnings, "every accepted key is known")
}

func TestParseFrontmatter_NonMappingLensAndTP(t *testing.T) {
	fm := ParseFrontmatterBytes([]byte("---\ntp:\n  domain: prose\n  lens: \"nope\"\n---\n"))
	assert.Equal(t, "prose", fm.Domain)
	assert.Empty(t, fm.Lens)
	require.Len(t, fm.Warnings, 1)
	assert.Contains(t, fm.Warnings[0].Message, "tp.lens is not a mapping")

	fm = ParseFrontmatterBytes([]byte("---\ntp: just-a-string\n---\n"))
	assert.Equal(t, DomainSoftware, fm.Domain)
	require.Len(t, fm.Warnings, 1)
	assert.Contains(t, fm.Warnings[0].Message, "tp is not a mapping")
}
