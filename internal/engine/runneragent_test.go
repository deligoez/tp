package engine_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// agentForKind is §6.3's table, written out literally rather than derived from
// the production mapping: a test that asked the code what it maps would agree
// with any mapping at all. The empty string is "this kind takes no agent",
// which is five of the eight — and it is the half of test 44 a three-case test
// would pass without ever exercising.
var agentForKind = map[engine.UnitKind]string{
	engine.UnitImplement:     "tp-implementer",
	engine.UnitReviewRole:    "tp-reviewer",
	engine.UnitAuditRole:     "tp-auditor",
	engine.UnitReviewRecord:  "",
	engine.UnitReviewResolve: "",
	engine.UnitDecompose:     "",
	engine.UnitAuditRecord:   "",
	engine.UnitAuditFix:      "",
}

// agentlessKind is one of the five kinds §6.3 leaves with the ordinary tool
// set. §3.2.1's own tests measure the template on it, so the base argv and the
// budget condition stay separable from the per-kind pair layered on top.
const agentlessKind = engine.UnitReviewRecord

// kindValues is values() with the kind under test, so a per-kind assertion
// changes exactly one input.
func kindValues(kind engine.UnitKind, budget float64) engine.TemplateValues {
	v := values(budget)
	v.Kind = kind
	return v
}

// agentArg returns the value the argv passes to --agent, and whether the flag
// is present at all. It scans rather than indexing so a change of argv order
// cannot turn a missing flag into a passing test.
func agentArg(args []string) (string, bool) {
	for i, arg := range args {
		if arg != "--agent" {
			continue
		}
		if i+1 < len(args) {
			return args[i+1], true
		}
		return "", true // present but with nothing after it: a bug, reported as such
	}
	return "", false
}

// TestBuiltinRunner_ClaudeAppendsTheAgentPerKind is test 44. Every one of the
// eight kinds is asserted, in both directions: the three that take an agent get
// exactly that agent's name, and the five that take none carry no --agent token
// anywhere in the argv. The five matter more than the three — §6.3 gives the
// record, resolve, decompose and fix kinds "the ordinary tool set", and a test
// covering only the three named agents passes whether or not the "none" branch
// was ever written.
func TestBuiltinRunner_ClaudeAppendsTheAgentPerKind(t *testing.T) {
	require.Len(t, agentForKind, len(engine.UnitKinds()),
		"every kind must be decided here, so a ninth kind cannot silently inherit an agent")

	for _, kind := range engine.UnitKinds() {
		want, listed := agentForKind[kind]
		require.True(t, listed, "%s is unlisted", kind)

		runner, err := engine.BuiltinRunner(engine.TemplateClaude, kindValues(kind, 0))
		require.NoError(t, err)

		got, present := agentArg(runner.Args)
		if want == "" {
			assert.False(t, present,
				"%s takes the ordinary tool set, so the claude template appends no --agent (§6.3); argv=%v",
				kind, runner.Args)
			assert.NotContains(t, runner.Args, "tp-implementer")
			assert.NotContains(t, runner.Args, "tp-reviewer")
			assert.NotContains(t, runner.Args, "tp-auditor")
			continue
		}

		require.True(t, present, "%s runs under an agent (§6.3); argv=%v", kind, runner.Args)
		assert.Equal(t, want, got, "%s runs under %s", kind, want)
	}
}

// The agent pair is part of the template's argv, so it survives placeholder
// expansion untouched and reaches the child as two ordinary arguments. Without
// this the flag could be added in a form that expansion mangles and every
// assertion above would still pass.
func TestBuiltinRunner_ClaudeAgentSurvivesExpansion(t *testing.T) {
	for kind, want := range agentForKind {
		if want == "" {
			continue
		}
		runner, err := engine.BuiltinRunner(engine.TemplateClaude, kindValues(kind, 0))
		require.NoError(t, err)

		argv, expandErr := engine.ExpandArgs(runner.Args, kindValues(kind, 0))
		require.NoError(t, expandErr)

		got, present := agentArg(argv)
		require.True(t, present, "argv=%v", argv)
		assert.Equal(t, want, got)
	}
}

// The budget pair and the agent pair are independent: a non-zero budget adds
// the flag §3.2.1 documents and changes nothing about the agent, and neither
// pair displaces the other. The last argument stays the budget placeholder, so
// §3.2.1's own assertion about it keeps its meaning.
func TestBuiltinRunner_ClaudeAgentAndBudgetCoexist(t *testing.T) {
	runner, err := engine.BuiltinRunner(engine.TemplateClaude, kindValues(engine.UnitAuditRole, 2.5))
	require.NoError(t, err)

	assert.Equal(t, append(append([]string{}, claudeBaseArgs...),
		"--agent", "tp-auditor",
		"--max-budget-usd", "{max_budget_usd}"), runner.Args)
}

// The agent flag belongs to the claude template alone. opencode has no such
// flag, and inventing one for it would hand an unknown argument to a harness
// that would refuse to start (§3.2.1 gives opencode `run` and the prompt).
func TestBuiltinRunner_OpencodeTakesNoAgent(t *testing.T) {
	for _, kind := range engine.UnitKinds() {
		runner, err := engine.BuiltinRunner(engine.TemplateOpencode, kindValues(kind, 0))
		require.NoError(t, err)

		_, present := agentArg(runner.Args)
		assert.False(t, present, "opencode takes no --agent for %s; argv=%v", kind, runner.Args)
		assert.Equal(t, []string{"run", "{prompt}"}, runner.Args)
	}
}
