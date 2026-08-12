package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
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
	offenders := make([]string, 0)

	for _, path := range hintGuardedFiles(t) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err, "parse %s", path)

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !isOutputError(call) || len(call.Args) != 2 {
				return true
			}
			if code, ok := call.Args[0].(*ast.Ident); !ok || code.Name != "ExitFile" {
				return true
			}
			pos := fset.Position(call.Pos())
			offenders = append(offenders, filepath.Base(path)+":"+itoa(pos.Line))
			return true
		})
	}

	sort.Strings(offenders)
	require.Empty(t, offenders,
		"these exit-3 sites pass no hint, so they inherit the code-3 default — task-file advice, "+
			"which repairs nothing in a command that takes no task file. Pass the hint that fits: "+
			"specFileMissingHint, findingsFileMissingHint, ndjsonInputFileHint, ndjsonReadHint(err), "+
			"affectedFilesHint or outputFileHint")
}

// TestNDJSONReadersShareTheCap is the mechanical form of the second class three
// audit rounds reported one reader at a time: ndjson-line-cap-asymmetry.
//
// bufio's default is 64KB, so a scanner built without Buffer silently stops at
// the first longer line. Round 5 raised three readers, round 6 found two more,
// round 7 found the last one — each time by reading. Every scanner in the
// review/audit family now either declares ndjsonLineCap or says on its own line
// what non-NDJSON input it reads, and this guard is what keeps the next one
// from shipping at 64KB.
//
// Scope matches TestFileErrorsCarryAHint. The tp add --bulk and tp set --bulk
// readers scan NDJSON at 64KB too, but each pins its own warn-and-continue
// contract in its own tests (add_validation_test.go); reversing those is a
// change to a different command family and belongs to its own version, not to
// this sweep.
func TestNDJSONReadersShareTheCap(t *testing.T) {
	// The exemption is per SITE, not per file: a scanner over non-NDJSON input
	// says so on its own line with a "line-cap:" marker naming what it scans.
	// A file-level allow-list would have excused every other scanner in
	// review.go and audit.go — including the NDJSON readers this guard exists
	// to hold, which is how a guard passes while the thing it guards regresses.
	const siteExemption = "line-cap:"

	uncapped := make([]string, 0)
	for _, path := range hintGuardedFiles(t) {
		name := filepath.Base(path)
		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)

		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if !strings.Contains(line, "bufio.NewScanner(") {
				continue
			}
			// The Buffer call is the next statement when the cap is declared.
			capped := i+1 < len(lines) && strings.Contains(lines[i+1], "scanner.Buffer(") &&
				strings.Contains(lines[i+1], "ndjsonLineCap")
			if capped || strings.Contains(line, siteExemption) {
				continue
			}
			uncapped = append(uncapped, name+":"+itoa(i+1))
		}
	}

	sort.Strings(uncapped)
	require.Empty(t, uncapped,
		"these scanners keep bufio's 64KB default: past it they stop early and the caller "+
			"sees a silently shorter set. Declare the shared cap on the next line "+
			"(scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)), or mark the site "+
			"with a line-cap: comment naming the non-NDJSON input it scans")
}

// hintGuardedFiles lists the non-test review/audit sources this guard covers.
func hintGuardedFiles(t *testing.T) []string {
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
		if !strings.HasPrefix(name, "review") && !strings.HasPrefix(name, "audit") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	require.NotEmpty(t, files, "the guard must find the review/audit sources it claims to cover")
	return files
}

// isOutputError reports whether the call is output.Error(...).
func isOutputError(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Error" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "output"
}

// itoa keeps the offender list readable without pulling strconv in for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
