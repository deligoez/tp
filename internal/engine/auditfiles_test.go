package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditFileCaps_Literals pins both caps so a silent change fails a test.
func TestAuditFileCaps_Literals(t *testing.T) {
	assert.Equal(t, 10, CodeFileCap, "the shared code-file list cap")
	assert.Equal(t, 20, AuditFileCap, "the per-prompt affected-files cap")
}

func TestAuditCategory_EnumValid(t *testing.T) {
	for _, c := range []string{"security", "concurrency", "error-handling", "correctness", "contract"} {
		assert.True(t, IsValidCategory(c), c)
	}
	for _, c := range []string{"other", "misc", "", "Security"} {
		assert.False(t, IsValidCategory(c), c)
	}
}

func TestAuditCategories_IsPrecedenceOrderAndACopy(t *testing.T) {
	got := AuditCategories()
	assert.Equal(t, []string{"security", "concurrency", "error-handling", "correctness", "contract"}, got,
		"the order is the precedence RenderAuditCategoryText prints, not alphabetical")

	got[0] = "mutated"
	assert.Equal(t, "security", AuditCategories()[0], "a caller must not be able to reorder the precedence")
}

func TestRenderAuditCategoryText_ContainsEnumAndPrecedence(t *testing.T) {
	text := RenderAuditCategoryText()
	for _, c := range []string{"security", "concurrency", "error-handling", "correctness", "contract"} {
		assert.Contains(t, text, `"`+c+`"`)
	}
	assert.Contains(t, text, "security > concurrency > error-handling > correctness > contract")
	assert.Contains(t, text, "PASS -> category: null")
	assert.Contains(t, text, "PARTIAL or FAIL")
}

func TestFilterFiles_SpecCoverage_RanksByTaskCount(t *testing.T) {
	in := &AuditFileInputs{
		Universe: []string{"b.go", "a.go", "c.go"},
		TaskFiles: map[string][]string{
			"a.go": {"t1"},
			"b.go": {"t1", "t2"},
			"c.go": {"t3"},
		},
		DiffStats: map[string][2]int{"b.go": {13, 48}},
	}
	sel := SelectAuditFiles(in)
	require.Len(t, sel.SpecCoverage, 3)
	assert.Equal(t, "b.go", sel.SpecCoverage[0].Path, "highest task count first")
	assert.Equal(t, []string{"t1", "t2"}, sel.SpecCoverage[0].Tasks)
	assert.Equal(t, "+13/-48", sel.SpecCoverage[0].DiffSummary)
	assert.Equal(t, "a.go", sel.SpecCoverage[1].Path, "tie broken alphabetically")
	assert.Equal(t, "c.go", sel.SpecCoverage[2].Path)
	assert.Equal(t, "+0/-0", sel.SpecCoverage[2].DiffSummary, "outside-diff files fall back to +0/-0")
}

func TestFilterFiles_Cap20(t *testing.T) {
	in := &AuditFileInputs{TaskFiles: map[string][]string{}}
	for i := range 30 {
		p := fmt.Sprintf("f%02d.go", i)
		in.Universe = append(in.Universe, p)
		in.TaskFiles[p] = []string{"t1"}
	}
	sel := SelectAuditFiles(in)
	assert.Len(t, sel.SpecCoverage, AuditFileCap, "spec-coverage capped at 20")
}

func TestFilterFiles_CodeFiles(t *testing.T) {
	in := &AuditFileInputs{}
	for i := 14; i >= 0; i-- {
		in.Universe = append(in.Universe, fmt.Sprintf("m%02d.go", i))
	}
	sel := SelectAuditFiles(in)
	require.Len(t, sel.CodeFiles, CodeFileCap, "no path matches a priority keyword, so the first CodeFileCap files are taken")
	assert.Equal(t, "m00.go", sel.CodeFiles[0].Path, "alphabetical regardless of input order")
	assert.Equal(t, "m09.go", sel.CodeFiles[9].Path)
}

func TestFilterFiles_CodeFilesRanksPriorityPathsFirst(t *testing.T) {
	in := &AuditFileInputs{Universe: []string{"b.go", "a.go", "z_auth.go"}}
	sel := SelectAuditFiles(in)
	require.Len(t, sel.CodeFiles, 3)
	assert.Equal(t, "z_auth.go", sel.CodeFiles[0].Path, "keyword match ranks first even though it sorts last")
	assert.Equal(t, "a.go", sel.CodeFiles[1].Path, "the rest stay alphabetical")
	assert.Equal(t, "b.go", sel.CodeFiles[2].Path)
}

func TestFilterFiles_DropsBinaryFixturesDeleted(t *testing.T) {
	in := &AuditFileInputs{
		Universe: []string{
			"keep.go",
			"logo.png",
			"testdata/fixture.json",
			"pkg/testdata/inner.txt",
			"snapshot.golden",
			"removed.go",
		},
		Deleted: map[string]bool{"removed.go": true},
	}
	sel := SelectAuditFiles(in)
	require.Len(t, sel.CodeFiles, 1)
	assert.Equal(t, "keep.go", sel.CodeFiles[0].Path)
	require.Len(t, sel.SpecCoverage, 1, "fallback list is also filtered")
	assert.Equal(t, "keep.go", sel.SpecCoverage[0].Path)
}

func TestFilterFiles_DropFirstBackfill(t *testing.T) {
	// 13-file diff whose first 10 alphabetical entries include 3 fixtures:
	// drops filter before caps, so the 10-entry code-file list backfills
	// from the raw 11th-13th files and contains no fixture path.
	universe := []string{
		"a01.go", "a02.golden", "a03.go", "a04.go", "testdata/a05.json",
		"a06.go", "a07.go", "a08.golden", "a09.go", "a10.go",
		"z11.go", "z12.go", "z13.go",
	}
	sel := SelectAuditFiles(&AuditFileInputs{Universe: universe})
	require.Len(t, sel.CodeFiles, CodeFileCap, "13 - 3 = 10 eligible")
	for _, e := range sel.CodeFiles {
		assert.NotContains(t, e.Path, "testdata/")
		assert.NotContains(t, e.Path, ".golden")
	}
	assert.Equal(t, "z13.go", sel.CodeFiles[9].Path, "tail backfilled from the raw 11th-13th files")
}

func TestFilterFiles_PreCapTotals(t *testing.T) {
	// A universe holding a binary, a fixture, and a deleted file: only the three
	// remaining .go files are candidates, so both totals count those alone.
	universe := []string{"a.go", "b.go", "c.go", "logo.png", "testdata/x.json", "gone.go"}
	deleted := map[string]bool{"gone.go": true}

	mapped := SelectAuditFiles(&AuditFileInputs{
		Universe:  universe,
		Deleted:   deleted,
		TaskFiles: map[string][]string{"a.go": {"t1"}, "b.go": {"t2"}},
	})
	assert.Equal(t, 3, mapped.CodeFilesTotal, "filtered-universe size, dropped files excluded")
	assert.Equal(t, 2, mapped.SpecCoverageTotal, "task-mapped set size in the normal branch")

	fallback := SelectAuditFiles(&AuditFileInputs{Universe: universe, Deleted: deleted})
	assert.Equal(t, 3, fallback.CodeFilesTotal, "filtered-universe size, dropped files excluded")
	assert.Equal(t, 3, fallback.SpecCoverageTotal, "filtered-universe size in the no-task fallback branch")
}

// TestFilterFiles_CodeFilesRankByChurn pins decision 1 of
// spec/backlog/checklist-covers-what-changed.md: within each priority group the
// code roles' list runs by lines added plus deleted, descending, ties by path,
// and a file with no measured diff comes last in path order. The fixture's
// path order disagrees with its churn order at every position, so the shipped
// alphabetical order cannot pass it.
func TestFilterFiles_CodeFilesRankByChurn(t *testing.T) {
	in := &AuditFileInputs{
		Universe: []string{"a.go", "b.go", "c.go", "d.go", "e.go", "y_lock.go", "z_auth.go"},
		DiffStats: map[string][2]int{
			"a.go":      {1, 0},
			"b.go":      {3, 3},
			"d.go":      {40, 2},
			"e.go":      {3, 3},
			"y_lock.go": {1, 1},
			"z_auth.go": {9, 0},
		},
	}
	sel := SelectAuditFiles(in)
	got := make([]string, 0, len(sel.CodeFiles))
	for _, e := range sel.CodeFiles {
		got = append(got, e.Path)
	}
	assert.Equal(t, []string{"z_auth.go", "y_lock.go", "d.go", "b.go", "e.go", "a.go", "c.go"}, got,
		"priority group by churn, then the rest by churn with ties by path, unmeasured c.go last")
}

// TestFilterFiles_CodeFilesChurnDecidesTheCut: the cap keeps the most-changed
// files, not the first ones in path order.
func TestFilterFiles_CodeFilesChurnDecidesTheCut(t *testing.T) {
	in := &AuditFileInputs{DiffStats: map[string][2]int{}}
	for i := range 15 {
		p := fmt.Sprintf("m%02d.go", i)
		in.Universe = append(in.Universe, p)
		in.DiffStats[p] = [2]int{i, 0}
	}
	sel := SelectAuditFiles(in)
	require.Len(t, sel.CodeFiles, CodeFileCap)
	assert.Equal(t, "m14.go", sel.CodeFiles[0].Path, "the largest churn leads")
	assert.Equal(t, "m05.go", sel.CodeFiles[9].Path, "the five smallest fall outside the cap")
}

// TestFilterFiles_NamedListIsNotCut pins decision 2: a universe the caller
// named goes whole to the code roles, and the selection says it was named.
// DiffUnmeasured alone does not decide this; Named does.
func TestFilterFiles_NamedListIsNotCut(t *testing.T) {
	in := &AuditFileInputs{Named: true, DiffUnmeasured: true}
	for i := range 25 {
		in.Universe = append(in.Universe, fmt.Sprintf("n%02d.go", i))
	}
	sel := SelectAuditFiles(in)
	assert.Len(t, sel.CodeFiles, 25, "no cut on a named list")
	assert.Equal(t, 25, sel.CodeFilesTotal)
	assert.True(t, sel.CodeFilesNamed)

	unnamed := SelectAuditFiles(&AuditFileInputs{Universe: in.Universe, DiffUnmeasured: true})
	assert.Len(t, unnamed.CodeFiles, CodeFileCap, "an unmeasured but auto-detected universe is still cut")
	assert.False(t, unnamed.CodeFilesNamed)
}

func TestPriorityPathSubstrings_ExactValues(t *testing.T) {
	assert.Equal(t, []string{"lock", "validate", "auth", "secret", "perm"}, priorityPathSubstrings)
}
