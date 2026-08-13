package cli_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
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

// declKey names one declaration in the fact map. A method is qualified by its
// receiver type: two declarations named Error (*reportPathError's and
// flagUsageError's) would otherwise share the key "Error" and the second would
// overwrite the first, silently dropping a writesUnlocked fact from the seed.
// Go forbids two package-level functions of one name and two methods of one
// name on one type, so the qualified key is unique by construction. Method
// calls reach these keys through receiverTypes and recordCallee below.
func declKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return types.ExprString(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

// baseTypeName returns the package-local named type expr denotes, pointer
// stripped, or "" when expr is not a bare local name: a qualified type from
// another package, an interface, a func type, a map or a slice all resolve to
// nothing rather than to a guess.
func baseTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// receiverTypes maps the identifiers inside fn that hold a package-local named
// type to that type's name -- the enclosing method's own receiver, and locals
// declared `var x T`, `x := T{...}` or `x := &T{...}`. Those are the receivers
// a method call can be resolved through without a type checker; every other
// receiver (an interface value such as err in err.Error(), a func value, a
// value produced by another package, a chained field or call expression) is
// left unresolved and contributes no edge. The map is function-wide rather
// than scope-aware, so a name declared twice in one function resolves to the
// declaration the walk saw last.
func receiverTypes(fn *ast.FuncDecl) map[string]string {
	locals := make(map[string]string)
	if fn.Recv != nil && len(fn.Recv.List) > 0 && len(fn.Recv.List[0].Names) > 0 {
		if name := baseTypeName(fn.Recv.List[0].Type); name != "" {
			locals[fn.Recv.List[0].Names[0].Name] = name
		}
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.DeclStmt:
			gen, ok := stmt.Decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				return true
			}
			for _, spec := range gen.Specs {
				vs, isValue := spec.(*ast.ValueSpec)
				if !isValue || vs.Type == nil {
					continue
				}
				if name := baseTypeName(vs.Type); name != "" {
					for _, id := range vs.Names {
						locals[id.Name] = name
					}
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range stmt.Lhs {
				id, isIdent := lhs.(*ast.Ident)
				if !isIdent || i >= len(stmt.Rhs) {
					continue
				}
				rhs := stmt.Rhs[i]
				if unary, isUnary := rhs.(*ast.UnaryExpr); isUnary && unary.Op == token.AND {
					rhs = unary.X
				}
				lit, isLit := rhs.(*ast.CompositeLit)
				if !isLit || lit.Type == nil {
					continue
				}
				if name := baseTypeName(lit.Type); name != "" {
					locals[id.Name] = name
				}
			}
		}
		return true
	})
	return locals
}

// recordCallee records the same-package declaration fun names, keyed as the
// declKey the fixpoint reads: a plain identifier for a function call, and for a
// method call the resolved receiver type qualified with the method name. Both
// receiver spellings are recorded because Go auto-addresses -- `var x T; x.M()`
// runs `func (T) M` or `func (*T) M`, and only one of the two can be declared.
func recordCallee(f *funcFacts, fun ast.Expr, locals map[string]string) {
	switch callee := fun.(type) {
	case *ast.Ident:
		f.callsUnlocked[callee.Name] = true
	case *ast.SelectorExpr:
		recv, isIdent := callee.X.(*ast.Ident)
		if !isIdent {
			return
		}
		typeName := locals[recv.Name]
		if typeName == "" {
			return
		}
		f.callsUnlocked[typeName+"."+callee.Sel.Name] = true
		f.callsUnlocked["*"+typeName+"."+callee.Sel.Name] = true
	}
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
			facts[declKey(fn)] = f
			locals := receiverTypes(fn)
			walkLockAware(fn.Body, false, func(call *ast.CallExpr, locked bool) {
				switch {
				case isTaskFileWrite(call):
					totalWrites++
					if !locked {
						f.writesUnlocked = true
					}
				case !locked:
					recordCallee(f, call.Fun, locals)
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
			// Only a RunE bound to a named function becomes an entry point. Two
			// commands bind a function literal instead -- tp audit and tp review --
			// so they sit outside the set this test checks. That is vacuous while
			// the unsafe set stays {claimSingle, claimBatch}, which neither reaches,
			// and the closure bodies would have to be walked as roots to close it.
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
