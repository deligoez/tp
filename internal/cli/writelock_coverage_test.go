package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isSelectorCall reports whether call is `pkg.name(...)`.
func isSelectorCall(call *ast.CallExpr, pkg, name string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

func isTaskFileWrite(call *ast.CallExpr) bool {
	return isSelectorCall(call, "model", "WriteTaskFile")
}

func isLockCall(call *ast.CallExpr) bool {
	return isSelectorCall(call, "engine", "WithFileLock") ||
		isSelectorCall(call, "engine", "WithFileLockTimeout")
}

// funcFacts is what one declared function contributes to the reachability
// question: does it write the task file outside a lock, and which same-package
// functions does it call outside a lock (so their writes are its writes too)?
type funcFacts struct {
	writesUnlocked bool
	callsUnlocked  map[string]bool
}

// collectFuncFacts parses every non-test .go file in dir and returns per-
// function facts, the names bound to a cobra RunE field, and the total number
// of model.WriteTaskFile call sites seen (the instrument's own floor: a parser
// that matches nothing must not read as "nothing is unlocked").
func collectFuncFacts(t *testing.T, dir string) (facts map[string]*funcFacts, runE []string, totalWrites int) {
	t.Helper()
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	facts = make(map[string]*funcFacts)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		require.NoError(t, parseErr)

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			f := &funcFacts{callsUnlocked: make(map[string]bool)}
			facts[fn.Name.Name] = f
			walkLockAware(fn.Body, false, func(call *ast.CallExpr, locked bool) {
				switch {
				case isTaskFileWrite(call):
					totalWrites++
					if !locked {
						f.writesUnlocked = true
					}
				case !locked:
					if id, isIdent := call.Fun.(*ast.Ident); isIdent {
						f.callsUnlocked[id.Name] = true
					}
				}
			})
		}
		ast.Inspect(file, func(n ast.Node) bool {
			kv, ok := n.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := kv.Key.(*ast.Ident)
			if !ok || key.Name != "RunE" {
				return true
			}
			if val, isIdent := kv.Value.(*ast.Ident); isIdent {
				runE = append(runE, val.Name)
			}
			return true
		})
	}
	return facts, runE, totalWrites
}

// walkLockAware visits every call expression under n, reporting whether it sits
// lexically inside an engine.WithFileLock/WithFileLockTimeout call — including
// inside the closure such a call is handed, which is where every locked write
// in this package lives.
func walkLockAware(n ast.Node, locked bool, visit func(call *ast.CallExpr, locked bool)) {
	childLocked := locked
	if call, ok := n.(*ast.CallExpr); ok {
		visit(call, locked)
		if isLockCall(call) {
			childLocked = true
		}
	}
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil {
			return false
		}
		if c == n {
			return true
		}
		walkLockAware(c, childLocked, visit)
		return false
	})
}

// TestNoUnlockedTaskFileWrite is §3's closing claim: after init and import moved
// inside the lock, no unlocked task-file write remains in the tool.
//
// It is proved rather than asserted, and proved through reachability rather
// than lexically — claim.go writes from claimSingle/claimBatch, helpers whose
// bodies sit outside the lock call but whose only callers are inside it, so a
// purely lexical scan would report false violations and teach nothing. The
// question that matters is whether any command can reach the write without
// passing through the lock: seed the unsafe set with functions that write
// outside a lock, propagate it along unlocked same-package calls to a fixpoint,
// then require that no cobra RunE entry point ended up in it.
func TestNoUnlockedTaskFileWrite(t *testing.T) {
	facts, runE, totalWrites := collectFuncFacts(t, ".")

	// Instrument floors: a scan that found no writes, or no commands, proves
	// nothing about either.
	require.GreaterOrEqual(t, totalWrites, 10, "the scan located the task-file write sites")
	require.GreaterOrEqual(t, len(runE), 20, "the scan located the command entry points")

	unsafe := make(map[string]bool)
	for name, f := range facts {
		if f.writesUnlocked {
			unsafe[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, f := range facts {
			if unsafe[name] {
				continue
			}
			for callee := range f.callsUnlocked {
				if unsafe[callee] {
					unsafe[name] = true
					changed = true
					break
				}
			}
		}
	}

	reachable := make([]string, 0)
	for _, name := range runE {
		if unsafe[name] {
			reachable = append(reachable, name)
		}
	}
	assert.Empty(t, reachable,
		"these commands can reach model.WriteTaskFile without passing through engine.WithFileLock")
}

// TestTaskFileWriteSinkIsCLIOnly backs the reachability scan's scope: it walks
// internal/cli only, which is exhaustive exactly as long as model.WriteTaskFile
// has no non-test caller elsewhere in the tool. A future caller in engine/ or
// cmd/ would slip past TestNoUnlockedTaskFileWrite unnoticed; this fails first.
func TestTaskFileWriteSinkIsCLIOnly(t *testing.T) {
	cliDir, err := filepath.Abs(".")
	require.NoError(t, err)
	root, err := filepath.Abs("../..")
	require.NoError(t, err)

	outside := make([]string, 0)
	fset := token.NewFileSet()
	require.NoError(t, filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			// Tool sources live under internal/ and cmd/ only; everything else
			// at the top level (spec/, skills/, .git/) holds no Go the claim
			// is about.
			if filepath.Dir(path) == root && d.Name() != "internal" && d.Name() != "cmd" {
				return fs.SkipDir
			}
			if d.Name() == "testdata" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || filepath.Dir(path) == cliDir {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		ast.Inspect(file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok && isTaskFileWrite(call) {
				outside = append(outside, path)
			}
			return true
		})
		return nil
	}))

	assert.Empty(t, outside,
		"model.WriteTaskFile is called only from internal/cli, so the lock-reachability scan there is exhaustive")
}
