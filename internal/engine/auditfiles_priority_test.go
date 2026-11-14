package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSelectCodeFiles_ATestFileIsNotPromotedByItsName: the priority keywords
// name code where a defect costs most (locking, validation, auth). A test file
// carrying one in its name is not that code, and promoting it let ten one-line
// test files fill the whole list of a release while the change itself ranked
// past them (spec/1.21.0-measurements.md). A test file ranks by churn with the
// rest; a non-test file with a keyword still ranks first.
func TestSelectCodeFiles_ATestFileIsNotPromotedByItsName(t *testing.T) {
	t.Parallel()
	in := &AuditFileInputs{DiffStats: map[string][2]int{}}
	for i := range CodeFileCap {
		p := fmt.Sprintf("internal/x/lock_%02d_test.go", i)
		in.Universe = append(in.Universe, p)
		in.DiffStats[p] = [2]int{1, 0}
	}
	in.Universe = append(in.Universe, "internal/x/big.go", "internal/x/validate.go")
	in.DiffStats["internal/x/big.go"] = [2]int{400, 20}
	in.DiffStats["internal/x/validate.go"] = [2]int{2, 0}

	sel := SelectAuditFiles(in)
	require.Len(t, sel.CodeFiles, CodeFileCap)
	assert.Equal(t, "internal/x/validate.go", sel.CodeFiles[0].Path, "a non-test keyword file still ranks first")
	assert.Equal(t, "internal/x/big.go", sel.CodeFiles[1].Path, "the change's largest file is not pushed out by keyword-named tests")
}

func TestIsTestSourcePath(t *testing.T) {
	t.Parallel()
	for _, p := range []string{"a/lock_test.go", "tests/test_validate.py", "src/auth.test.ts", "src/auth.spec.js", "__tests__/perm.js", "test/secret_helper.rb"} {
		assert.True(t, isTestSourcePath(p), p)
	}
	for _, p := range []string{"a/lock.go", "internal/validate/v.go", "src/auth.ts", "latest/x.go", "contest.go"} {
		assert.False(t, isTestSourcePath(p), p)
	}
}
