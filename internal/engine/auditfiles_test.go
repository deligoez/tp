package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditCategory_EnumValid(t *testing.T) {
	for _, c := range []string{"security", "concurrency", "error-handling", "correctness", "contract"} {
		assert.True(t, IsValidCategory(c), c)
	}
	for _, c := range []string{"other", "misc", "", "Security"} {
		assert.False(t, IsValidCategory(c), c)
	}
}

func TestAuditCategory_PrecedenceResolution(t *testing.T) {
	assert.Equal(t, "security", ResolveAuditCategory("correctness", "security"), "security wins over correctness")
	assert.Equal(t, "concurrency", ResolveAuditCategory("contract", "concurrency", "error-handling"))
	assert.Equal(t, "contract", ResolveAuditCategory("contract"))
	assert.Equal(t, "", ResolveAuditCategory("bogus"), "unknown values resolve to empty")
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
	for i := 0; i < 30; i++ {
		p := fmt.Sprintf("f%02d.go", i)
		in.Universe = append(in.Universe, p)
		in.TaskFiles[p] = []string{"t1"}
	}
	sel := SelectAuditFiles(in)
	assert.Len(t, sel.SpecCoverage, AuditFileCap, "spec-coverage capped at 20")

	// Security via path heuristic on all files
	in2 := &AuditFileInputs{}
	for i := 0; i < 30; i++ {
		in2.Universe = append(in2.Universe, fmt.Sprintf("auth_%02d.go", i))
	}
	sel2 := SelectAuditFiles(in2)
	assert.Len(t, sel2.Security, AuditFileCap, "security capped at 20")
}

func TestFilterFiles_Security(t *testing.T) {
	in := &AuditFileInputs{
		Universe: []string{"zz_plain.go", "app/locker.go", "notes.go", "gone.go"},
		HeadReader: func(path string) ([]byte, bool) {
			switch path {
			case "notes.go":
				return []byte("package notes\n// validate the input before use\n"), true
			case "gone.go":
				return nil, false // absent at HEAD: path heuristic only
			}
			return []byte("package plain\n"), true
		},
	}
	sel := SelectAuditFiles(in)
	paths := make([]string, 0, len(sel.Security))
	for _, e := range sel.Security {
		paths = append(paths, e.Path)
		assert.Empty(t, e.Tasks, "security entries carry no task list")
	}
	assert.Equal(t, []string{"app/locker.go", "notes.go"}, paths,
		"path match (lock) + content match (validate), alphabetical; absent-at-HEAD file judged by path alone")
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
	assert.Empty(t, sel.Security)
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
