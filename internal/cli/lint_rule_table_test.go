package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §8.2: README.md's rule table must list every rule identifier tp can emit.
//
// BLIND SPOT OF THE COLLECTED LIST — read this before trusting a green run.
// collectEmittableRules reads the source, not the running program, and it sees
// exactly one shape: a string literal written directly as the Rule field of a
// composite literal whose type is engine.Finding (or an element of a
// []engine.Finding literal), in a non-test .go file under internal/. It is
// therefore blind to
//
//   - a rule identifier that reaches the field through a variable, constant,
//     parameter or a later assignment (f.Rule = x) rather than as a literal;
//   - a rule emitted from outside internal/, or only from a _test.go file;
//   - a rule carried by some other type. engine.EntryFinding has a Rule field
//     with five identifiers (id-blank, title-blank, acceptance-blank,
//     source-anchor, unknown-dependency), and they are excluded on purpose:
//     nothing reads that field, so those identifiers never reach output today.
//     If one is ever serialized, this census will not notice.
//
// So the test proves "every collected rule is documented", not "every emittable
// rule is documented" — absence from the collected list does not prove
// non-emission. §8.2 states this is the achievable form: rules are emitted from
// more than one package, no single enumeration makes absence imply
// non-emission, and observing emissions over this repository's own specs
// surfaces only a subset. The reverse direction (a documented rule the census
// does not find) is deliberately NOT asserted — a rule may be emitted in a
// shape the census cannot see, and failing on it would push the table toward
// under-documenting.
func TestLintRuleTableDocumentsEveryEmittableRule(t *testing.T) {
	collected := collectEmittableRules(t)
	documented := documentedLintRules(t)

	// An empty or single-package census looks exactly like a clean one, so
	// check the instrument before reading its result: §8.2's premise is that
	// rules come from more than one package, and a census that lost the walk
	// (wrong root, parse error swallowed) would report zero undocumented rules
	// while measuring nothing.
	require.NotEmpty(t, collected, "the rule census found no Finding literals at all — the walk is broken, not the tree")
	require.NotEmpty(t, documented, "README.md's rule table parsed to zero rows — the table moved or its header changed")
	pkgs := make(map[string]bool)
	for _, site := range collected {
		pkgs[filepath.Dir(site)] = true
	}
	require.GreaterOrEqual(t, len(pkgs), 2,
		"the census reached only %v — §8.2's premise is that rules are emitted from more than one package", pkgs)

	rules := make([]string, 0, len(collected))
	for rule := range collected {
		rules = append(rules, rule)
	}
	sort.Strings(rules)

	for _, rule := range rules {
		assert.Contains(t, documented, rule,
			"rule %q is emitted at %s but has no line in README.md's rule table", rule, collected[rule])
	}
}

// collectEmittableRules returns rule identifier -> "dir/file.go:line" of the
// first site that emits it. See the blind spot recorded on the test above.
func collectEmittableRules(t *testing.T) map[string]string {
	t.Helper()
	root := repoRoot(t)
	fset := token.NewFileSet()
	rules := make(map[string]string)

	err := filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, parseErr, "parsing %s", path)

		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || findingTypeName(lit.Type) != "Finding" {
				return true
			}
			for _, rule := range ruleLiterals(lit) {
				if _, seen := rules[rule.name]; !seen {
					rel, relErr := filepath.Rel(root, fset.Position(rule.pos).Filename)
					require.NoError(t, relErr)
					rules[rule.name] = rel + ":" + strconv.Itoa(fset.Position(rule.pos).Line)
				}
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return rules
}

type ruleLiteral struct {
	name string
	pos  token.Pos
}

// ruleLiterals reads the Rule string literals out of one Finding composite
// literal, descending into elided element literals so []Finding{{Rule: ...}}
// is read as well as Finding{Rule: ...}.
func ruleLiterals(lit *ast.CompositeLit) []ruleLiteral {
	var out []ruleLiteral
	for _, elt := range lit.Elts {
		if nested, ok := elt.(*ast.CompositeLit); ok {
			out = append(out, ruleLiterals(nested)...)
			continue
		}
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "Rule" {
			continue
		}
		basic, ok := kv.Value.(*ast.BasicLit)
		if !ok || basic.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(basic.Value)
		if err != nil {
			continue
		}
		out = append(out, ruleLiteral{name: value, pos: basic.Pos()})
	}
	return out
}

// findingTypeName reduces a composite literal's type to its base type name, so
// Finding, engine.Finding and []Finding all answer "Finding". An elided
// element literal has no type of its own and answers "" — ruleLiterals reaches
// those through its parent instead.
func findingTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return t.Sel.Name
	case *ast.ArrayType:
		return findingTypeName(t.Elt)
	case *ast.StarExpr:
		return findingTypeName(t.X)
	}
	return ""
}

// documentedLintRules parses the rule identifiers out of README.md's rule
// table: the first cell of every row under the header whose first column is
// "Rule", with the surrounding backticks stripped.
func documentedLintRules(t *testing.T) map[string]bool {
	t.Helper()
	lines := strings.Split(readRepoDoc(t, "README.md"), "\n")

	header := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "| Rule |") {
			header = i
			break
		}
	}
	require.NotEqual(t, -1, header, "README.md has no rule table (no row starting with \"| Rule |\")")

	rules := make(map[string]bool)
	for _, line := range lines[header+2:] { // +2 skips the header and its separator row
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			break
		}
		cells := strings.Split(strings.Trim(trimmed, "|"), "|")
		rules[strings.Trim(strings.TrimSpace(cells[0]), "`")] = true
	}
	return rules
}
