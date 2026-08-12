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
// Scope defaults to COVERED. Selecting files by a review/audit name prefix left
// role_panel.go — squarely in the family — outside the guard, and a new file
// under any other name escaped it entirely with the whole gate green. So every
// file in the package is covered unless taskFileCommands excuses it by name,
// which means a file added tomorrow is guarded without anyone remembering to
// add it. A site inside a covered file can still opt out with a
// task-file-hint: comment saying why the default is right there.
func TestFileErrorsCarryAHint(t *testing.T) {
	const siteExemption = "task-file-hint:"

	exemptFiles := taskFileCommands(t)
	emptyConsts := emptyStringConsts(t)
	offenders := make([]string, 0)

	for _, path := range packageSources(t, func(name string) bool { return !exemptFiles[name] }) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		require.NoError(t, err, "parse %s", path)

		exempt := exemptLines(fset, file, siteExemption)

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
			if exempt[pos.Line] {
				return true
			}
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
			"stateWriteHint or internalEncodeHint — or, where the code-3 task-file default IS "+
			"the right advice, mark the site with a task-file-hint: comment saying so")
}

// taskFileCommands names the files the hint guard does NOT cover, each with the
// reason it is excused. These are the commands that read or write the task
// file, where the code-3 default ("run 'tp use <file>' … 'tp init <spec>'") is
// the right advice rather than the wrong object.
//
// Two entries are excused for a weaker reason, recorded rather than hidden:
// lint.go and init.go take a SPEC, so their exit-3 sites want
// specFileMissingHint. They sit outside the v0.33.0 audit surface, and sweeping
// them is v0.34.0 work — but the list says so instead of implying they are fine.
//
// The guard fails if an entry no longer exists, so the list cannot rot into a
// blanket exemption for files nobody has.
func taskFileCommands(t *testing.T) map[string]bool {
	t.Helper()
	reasons := map[string]string{
		"add.go":            "task-file command",
		"blocked.go":        "task-file command",
		"brief.go":          "task-file command",
		"claim.go":          "task-file command",
		"close.go":          "task-file command",
		"commit.go":         "task-file command",
		"config.go":         "task-file/project config",
		"config_extract.go": "task-file/project config",
		"done.go":           "task-file command",
		"graph.go":          "task-file command",
		"importcmd.go":      "task-file command",
		"init.go":           "spec-path sites; specFileMissingHint sweep is v0.34.0",
		"keep.go":           "task-file command",
		"lint.go":           "spec-path sites; specFileMissingHint sweep is v0.34.0",
		"listcmd.go":        "task-file command",
		"next.go":           "task-file command",
		"plan.go":           "task-file command",
		"ready.go":          "task-file command",
		"remove.go":         "task-file command",
		"reopen.go":         "task-file command",
		"report.go":         "task-file command",
		"resume.go":         "task-file command",
		"set.go":            "task-file command",
		"set_local.go":      "task-file command",
		"set_project.go":    "task-file command",
		"show.go":           "task-file command",
		"stats.go":          "task-file command",
		"status.go":         "task-file command",
		"use.go":            "task-file pointer",
		"validate.go":       "task-file command",
	}

	dir := filepath.Join(repoRoot(t), "internal", "cli")
	exempt := make(map[string]bool, len(reasons))
	for name, reason := range reasons {
		_, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, "%s is exempted as %q but no longer exists — drop the entry", name, reason)
		exempt[name] = true
	}
	return exempt
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

// scannerSites finds every bufio.NewScanner bound to a variable in fn, in both
// binding forms: scanner := bufio.NewScanner(f) and var scanner = ... . The
// assignment-only version missed the var form, which is a binding the guard's
// own "every scanner" claim covers.
func scannerSites(fn *ast.FuncDecl) []scannerSite {
	sites := make([]scannerSite, 0)
	add := func(lhs []ast.Expr, rhs []ast.Expr) {
		if len(lhs) != 1 || len(rhs) != 1 {
			return
		}
		call, ok := rhs[0].(*ast.CallExpr)
		if !ok || !isSelectorCall(call, "bufio", "NewScanner") {
			return
		}
		name, ok := lhs[0].(*ast.Ident)
		if !ok {
			return
		}
		sites = append(sites, scannerSite{name: name.Name, pos: call.Pos()})
	}
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.AssignStmt:
			add(decl.Lhs, decl.Rhs)
		case *ast.ValueSpec:
			names := make([]ast.Expr, 0, len(decl.Names))
			for _, name := range decl.Names {
				names = append(names, name)
			}
			add(names, decl.Values)
		}
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
