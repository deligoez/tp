package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

func TestParseLineRanges(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []LineRange
		wantErr bool
	}{
		{"single range", "4-10", []LineRange{{4, 10}}, false},
		{"multi range", "4-10,15-20,25-30", []LineRange{{4, 10}, {15, 20}, {25, 30}}, false},
		{"spaces", " 4 - 10 , 15 - 20 ", []LineRange{{4, 10}, {15, 20}}, false},
		{"empty", "", nil, false},
		{"whitespace only", "   ", nil, false},
		{"invalid format", "abc", nil, true},
		{"start > end", "10-4", nil, true},
		{"single number no dash", "4", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLineRanges(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestLineSet(t *testing.T) {
	set := LineSet([]LineRange{{1, 3}, {7, 9}})
	assert.True(t, set[1])
	assert.True(t, set[2])
	assert.True(t, set[3])
	assert.False(t, set[4])
	assert.True(t, set[7])
	assert.True(t, set[9])
	assert.False(t, set[10])
}

func TestCollapseToRanges(t *testing.T) {
	tests := []struct {
		name  string
		lines []int
		want  []LineRange
	}{
		{"contiguous", []int{1, 2, 3, 4, 5}, []LineRange{{1, 5}}},
		{"two gaps", []int{1, 2, 5, 6, 10}, []LineRange{{1, 2}, {5, 6}, {10, 10}}},
		{"single", []int{5}, []LineRange{{5, 5}}},
		{"empty", nil, nil},
		{"unsorted", []int{5, 1, 3, 2, 4}, []LineRange{{1, 5}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collapseToRanges(tt.lines)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestValidateLineCoverage_FullCoverage(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Title\nLine 2\nLine 3\nLine 4\nLine 5\n"), 0o600))

	tf := &model.TaskFile{
		Tasks: []model.Task{
			{ID: "t1", SourceLines: "1-5"},
		},
	}

	findings := ValidateLineCoverage(tf, specPath)
	// Should pass with no warnings about gaps
	for _, f := range findings {
		assert.NotContains(t, f.Message, "uncovered")
	}
}

func TestValidateLineCoverage_GapDetection(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Title\nLine 2\nLine 3\nLine 4\nLine 5\nLine 6\nLine 7\n"), 0o600))

	tf := &model.TaskFile{
		Tasks: []model.Task{
			{ID: "t1", SourceLines: "1-3"},
			{ID: "t2", SourceLines: "6-7"},
		},
	}

	findings := ValidateLineCoverage(tf, specPath)
	hasGap := false
	for _, f := range findings {
		if f.Rule == "line-coverage" && f.Line == 4 {
			hasGap = true
			assert.Contains(t, f.Message, "uncovered lines 4-5")
		}
	}
	assert.True(t, hasGap, "should detect gap at lines 4-5")
}

func TestValidateLineCoverage_MultiRange(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("L1\nL2\nL3\nL4\nL5\nL6\nL7\nL8\nL9\nL10\n"), 0o600))

	tf := &model.TaskFile{
		Tasks: []model.Task{
			{ID: "t1", SourceLines: "1-3,8-10"},
			{ID: "t2", SourceLines: "4-7"},
		},
	}

	findings := ValidateLineCoverage(tf, specPath)
	for _, f := range findings {
		assert.NotContains(t, f.Message, "uncovered", "multi-range should cover all lines")
	}
}

func TestValidateLineCoverage_NoSourceLines(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Title\nContent\n"), 0o600))

	tf := &model.TaskFile{
		Tasks: []model.Task{
			{ID: "t1"},
		},
	}

	findings := ValidateLineCoverage(tf, specPath)
	hasInfo := false
	for _, f := range findings {
		if f.Rule == "line-coverage" && f.Severity == "info" {
			hasInfo = true
			assert.Contains(t, f.Message, "no tasks have source_lines")
		}
	}
	assert.True(t, hasInfo)
}

func TestValidateLineCoverage_SkipsEmptyLines(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	// Lines: 1=Title, 2=empty, 3=Content, 4=empty, 5=More
	require.NoError(t, os.WriteFile(specPath, []byte("# Title\n\nContent\n\nMore\n"), 0o600))

	tf := &model.TaskFile{
		Tasks: []model.Task{
			{ID: "t1", SourceLines: "1-1,3-3,5-5"},
		},
	}

	findings := ValidateLineCoverage(tf, specPath)
	for _, f := range findings {
		assert.NotContains(t, f.Message, "uncovered", "empty lines should not count as uncovered")
	}
}

func TestValidateLineCoverage_OverlappingRanges(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("L1\nL2\nL3\nL4\nL5\n"), 0o600))

	tf := &model.TaskFile{
		Tasks: []model.Task{
			{ID: "t1", SourceLines: "1-3"},
			{ID: "t2", SourceLines: "2-5"}, // overlaps with t1
		},
	}

	findings := ValidateLineCoverage(tf, specPath)
	for _, f := range findings {
		assert.NotContains(t, f.Message, "uncovered", "overlapping ranges should still cover everything")
	}
}
