package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// selectorRows exercises the shapes auditRowIndex has to tell apart: a plain
// role, a role AuditRowRole trims, and an item_id that itself carries a colon.
var selectorRows = []map[string]any{
	{"role": "spec-coverage", "item_id": "item-1", "status": "PASS"},
	{"role": " ax-contract ", "item_id": "item-9", "status": "FAIL"},
	{"role": "go-safety", "item_id": "3.4:step-2", "status": "PARTIAL"},
	{"item_id": "orphan", "status": "FAIL"},
}

// TestAuditRowIndex_SelectorForms covers §3.3's two selector forms and every way
// a selector can fail to name a row. The boundary rows are the point: index 0
// and the last index resolve, one past the end does not, and a key is told from
// an index by the colon alone.
func TestAuditRowIndex_SelectorForms(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		selector string
		want     int
		wantErr  string
	}{
		{"first index", "0", 0, ""},
		{"last index", "3", 3, ""},
		{"one past the end", "4", -1, "out of range"},
		{"negative index", "-1", -1, "out of range"},
		{"key", "go-safety:3.4:step-2", 2, ""},
		{"key splits on the first colon only", "go-safety:3.4", -1, "no row matches"},
		{"role is trimmed the way AuditRowRole trims it", "ax-contract:item-9", 1, ""},
		{"an untrimmed role does not match", " ax-contract :item-9", -1, "no row matches"},
		{"a row with no role has the empty role", ":orphan", 3, ""},
		{"unknown role", "nobody:item-1", -1, "no row matches"},
		{"unknown item", "spec-coverage:item-99", -1, "no row matches"},
		{"neither form", "abc", -1, "invalid selector"},
		{"empty selector", "", -1, "invalid selector"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			index, usageErr := auditRowIndex(selectorRows, tc.selector)
			assert.Equal(t, tc.want, index)
			if tc.wantErr == "" {
				assert.Empty(t, usageErr)
				return
			}
			assert.Contains(t, usageErr, tc.wantErr)
		})
	}
}

// TestAuditRowIndex_EmptyResultsFile: every selector fails against a results
// file holding no rows, and the index message stays readable rather than
// pointing at a row that cannot exist.
func TestAuditRowIndex_EmptyResultsFile(t *testing.T) {
	t.Parallel()
	empty := make([]map[string]any, 0)

	index, usageErr := auditRowIndex(empty, "0")
	assert.Equal(t, -1, index)
	assert.Contains(t, usageErr, "out of range")

	index, usageErr = auditRowIndex(empty, "role:item")
	assert.Equal(t, -1, index)
	assert.Contains(t, usageErr, "no row matches")
}

// TestDispositionStatusOf covers the already-resolved refusal's reading of an
// existing disposition, including the shapes it cannot read.
func TestDispositionStatusOf(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "wontfix", dispositionStatusOf(map[string]any{"status": "wontfix"}))
	assert.Equal(t, "unknown", dispositionStatusOf(map[string]any{"status": 7}))
	assert.Equal(t, "unknown", dispositionStatusOf(map[string]any{}))
	assert.Equal(t, "unknown", dispositionStatusOf("fixed"))
	assert.Equal(t, "unknown", dispositionStatusOf(nil))
}

// TestDisposition_ShapeMatchesReview: the `resolved` object the audit side
// writes is the one tp review --resolve writes, so a durable-write predicate
// reads one form whichever side produced it.
func TestDisposition_ShapeMatchesReview(t *testing.T) {
	t.Parallel()
	d := disposition("duplicate", "same as item-4")
	assert.Equal(t, "duplicate", d["status"])
	assert.Equal(t, "same as item-4", d["evidence"])
	assert.NotEmpty(t, d["resolved_at"])
	assert.Len(t, d, 3, "no field the review counterpart does not write")
}
