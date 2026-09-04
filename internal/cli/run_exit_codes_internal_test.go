package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// runExitCodeContract is section 3.4's exit-code sentence as a table: `tp run`
// exits 0 on converged and 4 on every other stop_reason. (2 belongs to a usage
// error raised before the loop starts, which never reaches this mapping — it is
// dispatchError's, and run_exit_codes_test.go drives it through the binary.)
//
// The table is the claim; TestRunExitCode_CoversEveryDeclaredStopReason is what
// makes it exhaustive, by deriving the vocabulary from the engine package's own
// declarations rather than from this list. A tenth reason therefore fails this
// package's tests until someone decides which side of the contract it falls on.
var runExitCodeContract = map[engine.StopReason]int{
	engine.StopConverged:    ExitSuccess,
	engine.StopCapUnits:     ExitState,
	engine.StopCapWallClock: ExitState,
	engine.StopCapBudget:    ExitState,
	engine.StopEscalation:   ExitState,
	engine.StopUnitFailure:  ExitState,
	engine.StopNoUnits:      ExitState,
	engine.StopInterrupted:  ExitState,
	engine.StopDriverError:  ExitState,
}

// declaredStopReasons reads every constant declared with type StopReason out of
// the engine package's sources.
//
// The engine exports no enumerator over the nine — Known() answers membership
// one value at a time and cannot be walked — so the vocabulary is recovered
// from the declarations themselves. That is the whole point of doing it this
// way: a table listing the reasons this file happens to remember would let a
// tenth reason ship unmapped and silently take the default branch, while a
// table checked against the declarations cannot.
//
// Every non-test file in the package is parsed, not just stopreason.go, so
// moving a constant to a neighbouring file does not quietly shrink the
// vocabulary this test measures against.
func declaredStopReasons(t *testing.T) []engine.StopReason {
	t.Helper()

	sources, err := filepath.Glob(filepath.Join("..", "engine", "*.go"))
	require.NoError(t, err)
	require.NotEmpty(t, sources, "the engine package's sources are where the vocabulary is declared")

	fset := token.NewFileSet()
	var reasons []engine.StopReason
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, source, nil, 0)
		require.NoError(t, err, "parsing %s", source)

		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				named, ok := value.Type.(*ast.Ident)
				if !ok || named.Name != "StopReason" {
					continue
				}
				for _, expr := range value.Values {
					lit, ok := expr.(*ast.BasicLit)
					require.True(t, ok, "a StopReason constant in %s is not a literal", source)
					require.Equal(t, token.STRING, lit.Kind)
					unquoted, err := strconv.Unquote(lit.Value)
					require.NoError(t, err)
					reasons = append(reasons, engine.StopReason(unquoted))
				}
			}
		}
	}
	require.NotEmpty(t, reasons, "the parse recovered engine's StopReason constants")
	return reasons
}

// The table above and the vocabulary the engine declares are the same set: no
// declared reason is missing a mapping, and no mapping names a reason the
// engine does not declare.
func TestRunExitCode_CoversEveryDeclaredStopReason(t *testing.T) {
	t.Parallel()
	declared := declaredStopReasons(t)

	seen := make(map[engine.StopReason]bool, len(declared))
	for _, reason := range declared {
		assert.False(t, seen[reason], "%q is declared once", reason)
		seen[reason] = true

		assert.True(t, reason.Known(), "%q is inside the closed vocabulary", reason)
		_, mapped := runExitCodeContract[reason]
		assert.True(t, mapped,
			"%q has no exit code: decide whether it is a convergence or a report to a human, then add it to runExitCodeContract", reason)
	}

	for reason := range runExitCodeContract {
		assert.True(t, seen[reason], "%q is mapped but the engine declares no such stop reason", reason)
	}

	// Last, so that a tenth reason reports as an unmapped reason first and a
	// miscount second — the count alone would not say what to do about it.
	assert.Len(t, declared, 9, "section 3.4 names nine stop reasons")
}

// The mapping itself, over the whole vocabulary: exactly one reason exits 0.
func TestRunExitCode_ZeroOnConvergedAndFourOnEveryOther(t *testing.T) {
	t.Parallel()
	// The two codes are section 3.4's literals, pinned here so a renumbered
	// constant is caught rather than carried along by both sides at once.
	require.Equal(t, 0, ExitSuccess)
	require.Equal(t, 4, ExitState)

	var zeros []engine.StopReason
	for reason, want := range runExitCodeContract {
		got := runExitCode(reason)
		assert.Equal(t, want, got, "%q exits %d, not %d", reason, want, got)
		if got == ExitSuccess {
			zeros = append(zeros, reason)
		}
	}
	assert.Equal(t, []engine.StopReason{engine.StopConverged}, zeros,
		"converged is the only stop reason that exits 0")
}

// The default falls towards "a human is needed": a value outside the closed
// vocabulary — an empty reason from a run that ended without naming one, a
// near-miss spelling — is reported, never silently accepted as convergence.
//
// This is the asymmetry that makes the mapping a whitelist of the one reason
// rather than a list of the eight, and it is what distinguishes the rule from
// code that returns 4 for the eight it happens to know by name.
func TestRunExitCode_AnUnnamedReasonIsNotConvergence(t *testing.T) {
	t.Parallel()
	for _, unnamed := range []engine.StopReason{"", "Converged", "converged ", "CONVERGED", "cap_units", "done", "released"} {
		require.False(t, unnamed.Known(), "%q is outside the vocabulary", unnamed)
		assert.Equal(t, ExitState, runExitCode(unnamed),
			"%q is not convergence, so it exits 4", unnamed)
	}
}
