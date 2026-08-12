package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFileErrorsCarryAHint is the mechanical form of a finding class three
// audit rounds reported one site at a time: hintless-site-inherits-task-file-hint.
//
// output.Error's hint is variadic, so a two-argument call silently inherits the
// code-3 default, which is TASK-file advice ("run 'tp use <file>' … 'tp init
// <spec>'"). Across tp review and tp audit that advice is always wrong: none of
// these modes takes a task file, so every one of their exit-3 sites must say
// what to check instead. Rounds 5, 6 and 7 each found a fresh handful of these
// by reading; a class that survives three prose sweeps gets a check instead.
//
// Scope is deliberate. Elsewhere in the CLI (done, claim, import) the task-file
// default is often exactly right, so this guard covers only the command family
// that never reads a task file.
func TestFileErrorsCarryAHint(t *testing.T) {
	emptyConsts := emptyStringConsts(t)
	offenders := make([]string, 0)

	for _, path := range hintGuardedFiles(t) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err, "parse %s", path)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isOutputError(call) || len(call.Args) == 0 {
				return true
			}
			if code, ok := call.Args[0].(*ast.Ident); !ok || code.Name != "ExitFile" {
				return true
			}
			if !hintlessCall(call, emptyConsts) {
				return true
			}
			pos := fset.Position(call.Pos())
			offenders = append(offenders, filepath.Base(path)+":"+strconv.Itoa(pos.Line))
			return true
		})
	}

	sort.Strings(offenders)
	require.Empty(t, offenders,
		"these exit-3 sites pass no usable hint, so they inherit the code-3 default — task-file "+
			"advice, which repairs nothing in a command that takes no task file. Pass the hint that "+
			"fits: specFileMissingHint, findingsFileMissingHint, ndjsonInputFileHint, "+
			"ndjsonReadHint(err), affectedFilesHint, reviewDirFlagHint, outputFileHint, "+
			"stateWriteHint or internalEncodeHint")
}

// hintlessCall reports whether an output.Error call reaches resolveHint's
// fallback. Two arguments is the obvious form; the one a reader misses is a
// hint argument that EVALUATES to "", since resolveHint treats hint[0] == "" as
// no hint at all. Round 8 defeated the argument-count check with a literal ""
// and round 9 defeated the literal check with a named empty constant, so the
// test resolves identifiers against the package's own constants.
func hintlessCall(call *ast.CallExpr, emptyConsts map[string]bool) bool {
	if len(call.Args) < 3 {
		return true
	}
	switch hint := call.Args[2].(type) {
	case *ast.BasicLit:
		value, err := strconv.Unquote(hint.Value)
		return err == nil && value == ""
	case *ast.Ident:
		return emptyConsts[hint.Name]
	}
	// A call, a concatenation or a selector: not statically empty.
	return false
}

// emptyStringConsts collects package-level constants in internal/cli whose value
// is the empty string, so a hint named rather than written inline is judged by
// what it holds.
func emptyStringConsts(t *testing.T) map[string]bool {
	t.Helper()
	empty := make(map[string]bool)
	for _, path := range packageSources(t, func(string) bool { return true }) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err, "parse %s", path)

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok {
						continue
					}
					if value, err := strconv.Unquote(lit.Value); err == nil && value == "" {
						empty[name.Name] = true
					}
				}
			}
		}
	}
	return empty
}

// TestNDJSONReadersShareTheCap is the mechanical form of the second class three
// audit rounds reported one reader at a time: ndjson-line-cap-asymmetry.
//
// bufio's default is 64KB, so a scanner built without Buffer silently stops at
// the first longer line. Round 5 raised three readers, round 6 found two more,
// round 7 found the last one — each time by reading.
//
// The check is on the SCANNER, not on the text of the next line. Round 9 broke
// the text version three ways at once: min(4096, ndjsonLineCap) contained the
// constant and passed at 4KB, a correctly capped reader renamed from scanner to
// sc failed, and a reader in a new file matching neither name prefix escaped
// entirely. So this walks every non-test source in the package, binds each
// bufio.NewScanner to the variable it is assigned to, and requires a
// <var>.Buffer(_, ndjsonLineCap) — the identifier itself, no expression around
// it — somewhere in the same function.
//
// tp add --bulk and tp set --bulk read NDJSON at bufio's default and pin their
// own warn-and-continue contracts; they carry a line-cap: marker naming that,
// which is also what a genuinely non-NDJSON scanner (spec markdown, git output)
// carries. The marker states the exception at the site rather than in a list
// somewhere else that a new reader could quietly join.
func TestNDJSONReadersShareTheCap(t *testing.T) {
	const siteExemption = "line-cap:"

	uncapped := make([]string, 0)
	for _, path := range packageSources(t, func(string) bool { return true }) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		require.NoError(t, err, "parse %s", path)

		exempt := exemptLines(fset, file, siteExemption)

		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}
			for _, site := range scannerSites(fn) {
				line := fset.Position(site.pos).Line
				if exempt[line] || cappedInBody(fn.Body, site.name) {
					continue
				}
				uncapped = append(uncapped, filepath.Base(path)+":"+strconv.Itoa(line))
			}
			return true
		})
	}

	sort.Strings(uncapped)
	require.Empty(t, uncapped,
		"these scanners keep bufio's 64KB default: past it they stop early and the caller "+
			"sees a silently shorter set. Declare the shared cap on the scanner "+
			"(scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)), or mark the site "+
			"with a line-cap: comment naming the non-NDJSON input it scans")
}

// scannerSite is one bufio.NewScanner bound to a variable name.
type scannerSite struct {
	name string
	pos  token.Pos
}

// scannerSites finds every bufio.NewScanner assigned to a variable in fn.
func scannerSites(fn *ast.FuncDecl) []scannerSite {
	sites := make([]scannerSite, 0)
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || !isSelectorCall(call, "bufio", "NewScanner") {
			return true
		}
		name, ok := assign.Lhs[0].(*ast.Ident)
		if !ok {
			return true
		}
		sites = append(sites, scannerSite{name: name.Name, pos: call.Pos()})
		return true
	})
	return sites
}

// cappedInBody reports whether the body calls <name>.Buffer with ndjsonLineCap
// itself as the max — an expression built from the constant does not count,
// because that is how a 4KB cap once read as the shared one.
func cappedInBody(body *ast.BlockStmt, name string) bool {
	capped := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isSelectorCall(call, name, "Buffer") || len(call.Args) != 2 {
			return true
		}
		if maxArg, ok := call.Args[1].(*ast.Ident); ok && maxArg.Name == "ndjsonLineCap" {
			capped = true
		}
		return true
	})
	return capped
}

// isSelectorCall reports whether call is recv.method(...).
func isSelectorCall(call *ast.CallExpr, recv, method string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != method {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == recv
}

// exemptLines maps the lines a site-exemption comment covers: its own line,
// for a trailing comment, and the line after the comment group ends, for the
// block form written above the scanner.
func exemptLines(fset *token.FileSet, file *ast.File, marker string) map[int]bool {
	lines := make(map[int]bool)
	for _, group := range file.Comments {
		marked := false
		for _, c := range group.List {
			if strings.Contains(c.Text, marker) {
				marked = true
				lines[fset.Position(c.Pos()).Line] = true
			}
		}
		if marked {
			lines[fset.Position(group.End()).Line+1] = true
		}
	}
	return lines
}

// hintGuardedFiles lists the non-test review/audit sources the hint guard covers.
func hintGuardedFiles(t *testing.T) []string {
	t.Helper()
	files := packageSources(t, func(name string) bool {
		return strings.HasPrefix(name, "review") || strings.HasPrefix(name, "audit")
	})
	require.NotEmpty(t, files, "the guard must find the review/audit sources it claims to cover")
	return files
}

// packageSources lists the non-test .go files in internal/cli matching keep.
func packageSources(t *testing.T, keep func(name string) bool) []string {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "internal", "cli")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	files := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !keep(name) {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	require.NotEmpty(t, files, "no package sources matched")
	return files
}

// isOutputError reports whether the call is output.Error(...).
func isOutputError(call *ast.CallExpr) bool {
	return isSelectorCall(call, "output", "Error")
}
