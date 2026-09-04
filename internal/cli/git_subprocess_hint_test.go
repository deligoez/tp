package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitSubprocessFailuresNameGit guards a hole the hint enumeration cannot
// see. TestFileErrorsCarryAHint exempts commit.go and done.go wholesale as
// task-file commands, which is right for most of what they report — but not for
// a failure that came out of the git subprocess. There the exit-3 default names
// the task file and suggests `tp use`/`tp init`, which is a different object
// entirely, and an agent that follows it loops.
//
// This is how it reached CI: a runner with no committer identity made
// `git commit` exit 128, and tp answered with advice about task-file discovery.
//
// The rule is narrow on purpose — only calls whose own message says the failure
// came from git or from staging must carry an explicit hint. Everything else in
// those files keeps the default the exemption exists for.
func TestGitSubprocessFailuresNameGit(t *testing.T) {
	t.Parallel()
	gitMessage := func(s string) bool {
		return strings.Contains(s, "git ") || strings.Contains(s, "stage ")
	}

	root := repoRoot(t)
	offenders := make([]string, 0)

	for _, name := range []string{"commit.go", "done.go"} {
		path := filepath.Join(root, "internal", "cli", name)
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		require.NoError(t, err, "parse %s", name)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Error" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "output" {
				return true
			}

			// The message is the second argument, usually fmt.Sprintf(format, …).
			// Read the format literal; a message built some other way is not a
			// shape this guard claims to cover.
			var format string
			if inner, isCall := call.Args[1].(*ast.CallExpr); isCall && len(inner.Args) > 0 {
				if lit, isLit := inner.Args[0].(*ast.BasicLit); isLit {
					format = lit.Value
				}
			}
			if format == "" || !gitMessage(format) {
				return true
			}

			// A hint is the third argument. Its absence is the defect.
			if len(call.Args) < 3 {
				pos := fset.Position(call.Pos())
				offenders = append(offenders, filepath.Base(pos.Filename)+":"+
					strings.TrimPrefix(format, `"`))
				_ = pos
			}
			return true
		})
	}

	assert.Empty(t, offenders,
		"a failure that came from the git subprocess must not inherit the task-file hint — pass gitSubprocessHint")
}
