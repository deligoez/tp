package engine

import "fmt"

// PositionalItem is one spec item an audit checklist names by its position:
// a table row (table-<t>-<r>) or a numbered list item (list-<l>-<n>), with
// the line, section and text it was read from.
type PositionalItem struct {
	ID      string
	Type    string
	Line    int
	Section string
	Text    string
}

// SpecPositionalItems derives the spec's positional checklist items from its
// lines, the one derivation the audit checklist and the audit carried set
// share: tables are numbered by section in order, rows from 0; numbered lists
// open a new list at an item 1 in a new section, items by their own number.
func SpecPositionalItems(specLines []string) []PositionalItem {
	items := make([]PositionalItem, 0)
	tableIdx, rowIndex := -1, 0
	var prevSection string
	for _, row := range ExtractTableRows(specLines) {
		if row.Section != prevSection || tableIdx < 0 {
			tableIdx++
			rowIndex = 0
			prevSection = row.Section
		}
		items = append(items, PositionalItem{
			ID: fmt.Sprintf("table-%d-%d", tableIdx, rowIndex), Type: "table_row",
			Line: row.Line, Section: row.Section, Text: row.Raw,
		})
		rowIndex++
	}
	listIdx := -1
	var prevListSection string
	for _, item := range ExtractNumberedItems(specLines) {
		if (item.Number == 1 && item.Section != prevListSection) || listIdx < 0 {
			listIdx++
			prevListSection = item.Section
		}
		items = append(items, PositionalItem{
			ID: fmt.Sprintf("list-%d-%d", listIdx, item.Number), Type: "list_item",
			Line: item.Line, Section: item.Section, Text: item.Text,
		})
	}
	return items
}
