package cli

import (
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugifySubject(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"lowercase", "HelloWorld", "helloworld"},
		{"non-alphanumeric to dash", "a/b_c.go", "a-b-c-go"},
		{"collapsed runs", "a---b", "a-b"},
		{"trim leading and trailing", "!!!ab!!!", "ab"},
		{"digits preserved", "file123.go", "file123-go"},
		{"cap at 40 then trim trailing dash",
			"aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-bbbbb",
			"aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa-aaaa"},
		{"all non-alphanumeric produces empty", "!!!", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, slugifySubject(tc.in))
		})
	}
}

func TestSlugifySubject_Cap40(t *testing.T) {
	t.Parallel()
	var long strings.Builder
	for range 50 {
		long.WriteString("a")
	}
	slug := slugifySubject(long.String())
	assert.Len(t, slug, 40)
}

func TestFileCheckItems_StableAcrossReorder(t *testing.T) {
	t.Parallel()
	files := []engine.AuditFileEntry{
		{Path: "internal/foo.go"},
		{Path: "internal/bar.go"},
	}
	reordered := []engine.AuditFileEntry{
		{Path: "internal/bar.go"},
		{Path: "internal/foo.go"},
	}
	a := fileCheckItems(files, "security")
	b := fileCheckItems(reordered, "security")
	setOf := func(items []ChecklistItem) map[string]bool {
		m := make(map[string]bool, len(items))
		for _, it := range items {
			m[it.ItemID] = true
		}
		return m
	}
	assert.Equal(t, setOf(a), setOf(b), "same subjects keep same ids regardless of order")
}

// TestFileCheckItems_IDIsAFunctionOfRoleAndPathAlone: a file's id is the same
// whether it is emitted alone, beside a file whose slug collides with it, or
// after a file that sorts before it. The old positional -2 suffix made the id
// depend on the neighbours, so two shards of one round could give one id to
// two different files, and --merge kept one row of each such pair.
func TestFileCheckItems_IDIsAFunctionOfRoleAndPathAlone(t *testing.T) {
	t.Parallel()
	alone := func(p string) string {
		return fileCheckItems([]engine.AuditFileEntry{{Path: p}}, "security")[0].ItemID
	}
	items := fileCheckItems([]engine.AuditFileEntry{{Path: "0.go"}, {Path: "a_b.go"}, {Path: "a-b.go"}}, "security")
	require.Len(t, items, 3)
	assert.NotEqual(t, items[1].ItemID, items[2].ItemID, "a slug collision is told apart by the path digest")
	for _, it := range items {
		assert.Equal(t, alone(it.Section), it.ItemID, "%s keeps its id whatever else is in the list", it.Section)
	}
}

// TestFileCheckItems_IDShape pins the derivation: the readable slug, then a
// "." (a character the slug never holds, so an id tells its derivation by
// shape), then 16 hex characters of the SHA-256 of the cleaned path.
func TestFileCheckItems_IDShape(t *testing.T) {
	t.Parallel()
	id := fileCheckItems([]engine.AuditFileEntry{{Path: "internal/cli/audit.go"}}, "go-safety")[0].ItemID
	assert.Regexp(t, `^file-go-safety-[a-z0-9-]{1,40}\.[0-9a-f]{16}$`, id)
	assert.True(t, isCurrentFileCheckID(id))
	assert.False(t, isCurrentFileCheckID("file-go-safety-internal-cli-audit-go-apply-the-go"),
		"an id derived before the digest carries no suffix")
	assert.False(t, isCurrentFileCheckID("file-go-safety-internal-cli-audit-go-apply-the-go-2"),
		"nor does one carrying the old positional suffix")
}

// TestFileCheckItems_OneFileOneIDAcrossSpellings: a path spelled with a
// leading ./ or a doubled slash is the same file, so it gets the same id.
func TestFileCheckItems_OneFileOneIDAcrossSpellings(t *testing.T) {
	t.Parallel()
	id := func(p string) string {
		return fileCheckItems([]engine.AuditFileEntry{{Path: p}}, "security")[0].ItemID
	}
	assert.Equal(t, id("app/a.go"), id("./app/a.go"))
	assert.Equal(t, id("app/a.go"), id("app//a.go"))
}

func TestFileCheckItems_AlwaysContainsLetter(t *testing.T) {
	t.Parallel()
	files := []engine.AuditFileEntry{{Path: "123.go"}}
	items := fileCheckItems(files, "security")
	assert.Regexp(t, "[a-z]", items[0].ItemID, "slug id always contains a letter")
}
