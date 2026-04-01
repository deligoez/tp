package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckStructuredElements_Tables(t *testing.T) {
	lines := []string{
		"# Deactivation Rules",
		"",
		"| Condition | Action |",
		"|-----------|--------|",
		"| Final state | Auto-deactivate |",
		"| No slug | Deactivate |",
		"| Different slug | Replace |",
		"| Interactive state | Keep active |",
		"",
		"Some text after.",
	}

	findings, elems := CheckStructuredElements(lines, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 1, len(elems.Tables))
	assert.Equal(t, 4, elems.TotalRows, "should count 4 data rows (excluding header and separator)")
	assert.Equal(t, 3, elems.Tables[0].Line, "table starts at line 3")

	hasInfo := false
	for _, f := range findings {
		if f.Rule == "structured-elements" && f.Severity == "info" {
			hasInfo = true
		}
	}
	assert.True(t, hasInfo)
}

func TestCheckStructuredElements_NumberedList(t *testing.T) {
	lines := []string{
		"# Implementation Order",
		"",
		"1. Add continuation() method",
		"2. Add useContinuation param",
		"3. Create executeContinuation()",
		"4. Modify maybeRegisterScenarioOverrides()",
		"5. Write unit tests",
	}

	_, elems := CheckStructuredElements(lines, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 1, len(elems.NumberedLists))
	assert.Equal(t, 5, elems.TotalItems)
	assert.Equal(t, 5, elems.NumberedLists[0].LastNum)
	assert.Equal(t, 3, elems.NumberedLists[0].Line)
}

func TestCheckStructuredElements_CodeBlocks(t *testing.T) {
	lines := []string{
		"# API",
		"```php",
		"class Foo {",
		"}",
		"```",
		"",
		"```json",
		"{ \"key\": \"value\" }",
		"```",
	}

	_, elems := CheckStructuredElements(lines, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 2, elems.CodeBlocks)
}

func TestCheckStructuredElements_IgnoresTablesInCodeBlocks(t *testing.T) {
	lines := []string{
		"# Example",
		"```",
		"| Not | A | Table |",
		"|-----|---|-------|",
		"| Data | 1 | 2 |",
		"```",
	}

	_, elems := CheckStructuredElements(lines, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 0, elems.TotalRows, "tables inside code blocks should be ignored")
}

func TestCheckStructuredElements_Empty(t *testing.T) {
	_, elems := CheckStructuredElements([]string{}, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 0, elems.TotalRows)
	assert.Equal(t, 0, elems.TotalItems)
	assert.Equal(t, 0, elems.CodeBlocks)
}

func TestCheckStructuredElements_MultipleTables(t *testing.T) {
	lines := []string{
		"# Section 1",
		"| A | B |",
		"|---|---|",
		"| 1 | 2 |",
		"| 3 | 4 |",
		"",
		"# Section 2",
		"| X | Y |",
		"|---|---|",
		"| 5 | 6 |",
	}

	_, elems := CheckStructuredElements(lines, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 2, len(elems.Tables))
	assert.Equal(t, 3, elems.TotalRows, "2 rows in first table + 1 in second")
}

func TestCheckStructuredElements_NumberedTestList(t *testing.T) {
	// Simulates the spec feedback case: numbered tests #1-#7
	lines := []string{
		"### QA Tests",
		"",
		"1. Full lifecycle: activate → target → continuation → final",
		"2. Continuation with multiple interactive pauses (3 requests)",
		"3. Continuation with delegation chain (job→job→job)",
		"4. Explicit deactivation during continuation",
		"5. New scenario replaces active continuation",
		"6. Response format: activeScenario present during continuation",
		"7. Response format: availableScenarios alongside activeScenario",
	}

	findings, elems := CheckStructuredElements(lines, nil)
	require.NotNil(t, elems)
	assert.Equal(t, 1, len(elems.NumberedLists))
	assert.Equal(t, 7, elems.TotalItems)
	assert.Equal(t, 7, elems.NumberedLists[0].LastNum)

	// Should have info findings about the numbered list
	hasListInfo := false
	for _, f := range findings {
		if f.Rule == "structured-elements" && f.Line == 3 {
			hasListInfo = true
			assert.Contains(t, f.Message, "#1-#7")
		}
	}
	assert.True(t, hasListInfo)
}
