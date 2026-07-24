package cli

import (
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
)

func TestSlugifySubject(t *testing.T) {
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
	long := ""
	for i := 0; i < 50; i++ {
		long += "a"
	}
	slug := slugifySubject(long)
	assert.Len(t, slug, 40)
}

func TestFileCheckItems_StableAcrossReorder(t *testing.T) {
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

func TestFileCheckItems_CollisionSuffix(t *testing.T) {
	files := []engine.AuditFileEntry{
		{Path: "a_b.go"},
		{Path: "a-b.go"},
	}
	items := fileCheckItems(files, "security")
	assert.Len(t, items, 2)
	ids := map[string]bool{items[0].ItemID: true, items[1].ItemID: true}
	slug := slugifySubject("a_b.go Apply the security role rules to a_b.go")
	base := "file-security-" + slug
	assert.Contains(t, ids, base, "first item keeps base slug")
	assert.Contains(t, ids, base+"-2", "collision gets -2 suffix")
}

func TestFileCheckItems_AlwaysContainsLetter(t *testing.T) {
	files := []engine.AuditFileEntry{{Path: "123.go"}}
	items := fileCheckItems(files, "security")
	assert.Regexp(t, "[a-z]", items[0].ItemID, "slug id always contains a letter")
}
