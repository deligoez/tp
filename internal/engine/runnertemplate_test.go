package engine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// values returns a fully populated placeholder set, so a test that cares about
// one value can override just that one and still have the rest resolve.
func values(budget float64) engine.TemplateValues {
	return engine.TemplateValues{
		Prompt:       "run the brief",
		Kind:         engine.UnitImplement,
		ID:           "runner-templates",
		LogPath:      "/tmp/run/3-implement-runner-templates.jsonl",
		MaxBudgetUSD: budget,
	}
}

// claudeBaseArgs is §3.2.1's documented argv for the claude template, written
// out literally rather than derived, so a change to the shipped list has to be
// made twice on purpose.
var claudeBaseArgs = []string{
	"-p", "{prompt}",
	"--output-format", "stream-json",
	"--verbose",
	"--permission-mode", "auto",
}

// The claude template ships the documented argv, including --permission-mode
// auto: an unattended child that stops to ask for permission is the failure the
// whole version exists to prevent.
// The claude template ships the documented argv, including --permission-mode
// auto: an unattended child that stops to ask for permission is the failure the
// whole version exists to prevent.
//
// It is measured on a kind §6.3 leaves alone, so this test keeps saying what
// §3.2.1 says and nothing about the per-kind --agent pair layered on top of it;
// that pair has its own test, over all eight kinds.
func TestBuiltinRunner_ClaudeArgv(t *testing.T) {
	runner, err := engine.BuiltinRunner(engine.TemplateClaude, kindValues(agentlessKind, 0))
	require.NoError(t, err)

	assert.Equal(t, "claude", runner.Cmd)
	assert.Equal(t, claudeBaseArgs, runner.Args, "the template's argv is §3.2.1's, byte for byte")
	assert.Equal(t, "total_cost_usd", runner.SpendKey,
		"§3.2.2 reads total_cost_usd for claude, so the template declares it rather than being special-cased downstream")
}

// The budget flag is appended only when the resolved run_max_unit_budget_usd is
// non-zero. Both directions are asserted, because a test with only a non-zero
// value passes whether or not the condition exists at all.
// The budget flag is appended only when the resolved run_max_unit_budget_usd is
// non-zero. Both directions are asserted, because a test with only a non-zero
// value passes whether or not the condition exists at all. Like the argv test
// above it runs on an agentless kind, so the budget condition is measured on
// its own.
func TestBuiltinRunner_ClaudeAppendsBudgetOnlyWhenNonZero(t *testing.T) {
	zero, err := engine.BuiltinRunner(engine.TemplateClaude, kindValues(agentlessKind, 0))
	require.NoError(t, err)
	assert.Equal(t, claudeBaseArgs, zero.Args, "0 omits the flag entirely rather than passing a literal 0")
	assert.NotContains(t, zero.Args, "--max-budget-usd")
	assert.NotContains(t, zero.Args, "{max_budget_usd}")

	for _, tc := range []struct {
		budget float64
		want   string
	}{
		{0.01, "0.01"},
		{0.5, "0.5"},
		{5, "5"},
		{1000, "1000"},
	} {
		runner, budgetErr := engine.BuiltinRunner(engine.TemplateClaude, kindValues(agentlessKind, tc.budget))
		require.NoError(t, budgetErr)
		assert.Equal(t, append(append([]string{}, claudeBaseArgs...), "--max-budget-usd", "{max_budget_usd}"),
			runner.Args, "the pair is appended after the base argv, which is otherwise unchanged")

		argv, expandErr := engine.ExpandArgs(runner.Args, kindValues(agentlessKind, tc.budget))
		require.NoError(t, expandErr)
		assert.Equal(t, tc.want, argv[len(argv)-1], "the placeholder expands to the resolved dollars")
	}
}

// The opencode template is `run` with the prompt: no budget flag at any budget,
// and no spend_key, which is what makes cap-budget inert for it.
func TestBuiltinRunner_Opencode(t *testing.T) {
	for _, budget := range []float64{0, 7.5} {
		runner, err := engine.BuiltinRunner(engine.TemplateOpencode, values(budget))
		require.NoError(t, err)

		assert.Equal(t, "opencode", runner.Cmd)
		assert.Equal(t, []string{"run", "{prompt}"}, runner.Args)
		assert.Empty(t, runner.SpendKey, "no spend_key, so cap-budget is inert for opencode")

		argv, expandErr := engine.ExpandArgs(runner.Args, values(budget))
		require.NoError(t, expandErr)
		assert.Equal(t, []string{"run", "run the brief"}, argv)
	}
}

// A template name that is not one of the two tp ships is a usage error: the
// shape resolver decides the value is a name, and this layer decides whether it
// is a name tp knows (§3.2.1).
func TestBuiltinRunner_UnknownTemplateIsUsageError(t *testing.T) {
	for _, name := range []string{"cladue", "Claude", "", "gpt"} {
		runner, err := engine.BuiltinRunner(name, values(0))
		require.Error(t, err, "%q is not a built-in template", name)
		assert.Nil(t, runner)

		var tmplErr *engine.RunnerTemplateError
		require.ErrorAs(t, err, &tmplErr)
		assert.Contains(t, tmplErr.Hint(), "claude", "the hint names the templates tp ships")
		assert.Contains(t, tmplErr.Hint(), "opencode")
	}
}

// Every placeholder §3.2 documents resolves, and none is left in the argv.
func TestExpandArgs_EveryPlaceholderResolves(t *testing.T) {
	v := values(2.5)
	argv, err := engine.ExpandArgs([]string{
		"{unit_kind}", "{unit_id}", "{log_path}", "{max_budget_usd}", "{prompt}",
		"--log={log_path}", "before-{unit_id}-after",
	}, v)
	require.NoError(t, err)

	assert.Equal(t, []string{
		"implement", "runner-templates", v.LogPath, "2.5", "run the brief",
		"--log=" + v.LogPath, "before-runner-templates-after",
	}, argv)
	for _, arg := range argv {
		assert.NotContains(t, arg, "{", "no placeholder survives expansion: %q", arg)
	}
}

// {max_budget_usd} resolves at 0 too — a positional template receives the
// number, and only the claude template's flag pair is dropped (§3.2.1).
func TestExpandArgs_ZeroBudgetStillResolves(t *testing.T) {
	argv, err := engine.ExpandArgs([]string{"{unit_kind}", "{max_budget_usd}"}, values(0))
	require.NoError(t, err)
	assert.Equal(t, []string{"implement", "0"}, argv, "a literal 0, not an empty string and not an error")
}

// A placeholder the driver cannot resolve is a usage error naming the argument
// and the token, so an operator with a long args template is not left to find
// the typo by eye.
func TestExpandArgs_UnresolvedPlaceholderIsUsageError(t *testing.T) {
	for _, arg := range []string{"{model}", "--model={model}", "{Prompt}", "{unit-id}"} {
		argv, err := engine.ExpandArgs([]string{"-p", "{prompt}", arg}, values(0))
		require.Error(t, err, "%q is not a placeholder tp resolves", arg)
		assert.Nil(t, argv, "no argv is returned beside the error")

		var tmplErr *engine.RunnerTemplateError
		require.ErrorAs(t, err, &tmplErr)
		assert.Equal(t, "runner.args[2]", tmplErr.Field, "the field names the offending argument")
		assert.Contains(t, tmplErr.Error(), arg[strings.IndexByte(arg, '{'):],
			"the message quotes the token that could not be resolved")
		assert.Contains(t, tmplErr.Hint(), "{max_budget_usd}", "the hint lists the placeholders that do resolve")
	}
}

// Braces that are not placeholder-shaped are ordinary characters: a JSON
// literal in an args template is passed through, not reported.
func TestExpandArgs_LiteralBracesSurvive(t *testing.T) {
	args := []string{`{"role":"tester"}`, "{}", "{ unit_id }", "unterminated {prompt"}
	argv, err := engine.ExpandArgs(args, values(0))
	require.NoError(t, err)
	assert.Equal(t, args, argv, "nothing placeholder-shaped, so nothing is touched")
}

// A value that itself contains a brace token is not rescanned: the prompt is
// agent-facing prose the driver composes, and a `{model}` inside it must not be
// read back as an unresolved placeholder.
func TestExpandArgs_SubstitutedValueIsNotRescanned(t *testing.T) {
	v := values(0)
	v.Prompt = "report {model} and {unit_id} verbatim"

	argv, err := engine.ExpandArgs([]string{"-p", "{prompt}"}, v)
	require.NoError(t, err)
	assert.Equal(t, []string{"-p", "report {model} and {unit_id} verbatim"}, argv)
}

// End to end over §3.2's three shapes: the field resolves to a concrete argv
// with every placeholder expanded, whichever shape named the runner.
func TestResolveUnitRunner_ResolvesEachShapeToConcreteArgv(t *testing.T) {
	v := values(0)

	fromName, err := engine.ResolveUnitRunner(json.RawMessage(`"opencode"`), v)
	require.NoError(t, err)
	assert.Equal(t, "opencode", fromName.Cmd)
	assert.Equal(t, []string{"run", "run the brief"}, fromName.Args)

	fromObject, err := engine.ResolveUnitRunner(
		json.RawMessage(`{"cmd":"/bin/echo","args":["{unit_kind}","{unit_id}"],"spend_key":"cost"}`), v)
	require.NoError(t, err)
	assert.Equal(t, "/bin/echo", fromObject.Cmd)
	assert.Equal(t, []string{"implement", "runner-templates"}, fromObject.Args)
	assert.Equal(t, "cost", fromObject.SpendKey)

	fromMap, err := engine.ResolveUnitRunner(
		json.RawMessage(`{"implement":"claude","default":"opencode"}`), v)
	require.NoError(t, err)
	assert.Equal(t, "claude", fromMap.Cmd)
	assert.Equal(t, []string{"-p", "run the brief", "--output-format", "stream-json",
		"--verbose", "--permission-mode", "auto"}, fromMap.Args)

	auditValues := v
	auditValues.Kind = engine.UnitAuditRole
	fromDefault, err := engine.ResolveUnitRunner(
		json.RawMessage(`{"implement":"claude","default":"opencode"}`), auditValues)
	require.NoError(t, err)
	assert.Equal(t, "opencode", fromDefault.Cmd, "an unlisted kind takes the default branch's template")
}

// An absent runner field resolves to the built-in default template, so the
// whole chain works with no configuration at all.
func TestResolveUnitRunner_AbsentFieldTakesTheDefaultTemplate(t *testing.T) {
	runner, err := engine.ResolveUnitRunner(engine.DefaultRunner(), values(0))
	require.NoError(t, err)
	assert.Equal(t, "claude", runner.Cmd)

	fromNil, err := engine.ResolveUnitRunner(nil, values(0))
	require.NoError(t, err)
	assert.Equal(t, runner.Cmd, fromNil.Cmd)
}

// A shape error still classifies as a shape error through this layer: the
// template layer resolves names, it does not re-report §3.2's shapes.
func TestResolveUnitRunner_ShapeErrorsPropagate(t *testing.T) {
	runner, err := engine.ResolveUnitRunner(json.RawMessage(`{"audit-role":"claude"}`), values(0))
	require.Error(t, err, "a per-kind map without default is a usage error")
	assert.Nil(t, runner)

	var shapeErr *engine.RunnerShapeError
	assert.ErrorAs(t, err, &shapeErr, "the shape resolver's error reaches the caller unchanged")
}

// §3.2.1's ordering promise: an unresolved placeholder is raised BEFORE any
// child is spawned. Asserted by pointing the runner's cmd at a script that
// would leave a mark if it ever ran, and confirming the mark is absent after
// resolution failed.
func TestResolveUnitRunner_UnresolvedPlaceholderSpawnsNothing(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "spawned")
	script := filepath.Join(dir, "runner.sh")
	require.NoError(t, os.WriteFile(script,
		[]byte("#!/bin/sh\ntouch "+sentinel+"\n"), 0o755))

	raw, err := json.Marshal(map[string]any{
		"cmd":  script,
		"args": []string{"{prompt}", "{model}"},
	})
	require.NoError(t, err)

	runner, resolveErr := engine.ResolveUnitRunner(raw, values(0))
	require.Error(t, resolveErr, "{model} is not a placeholder tp resolves")
	assert.Nil(t, runner, "nothing to spawn is returned")

	var tmplErr *engine.RunnerTemplateError
	require.ErrorAs(t, resolveErr, &tmplErr)

	_, statErr := os.Stat(sentinel)
	assert.True(t, os.IsNotExist(statErr), "resolution spawned the runner: %s exists", sentinel)
}

// {prompt} expands to the fixed per-kind instruction: run this unit's brief
// command, do that one unit, stop.
func TestUnitPrompt_NamesTheBriefCommandAndStops(t *testing.T) {
	target := engine.UnitTarget{TaskFile: "spec/0.35.0.tasks.json", Spec: "spec/0.35.0.md", ID: "x", Round: 2}

	for _, kind := range engine.UnitKinds() {
		prompt := engine.UnitPrompt(kind, target)
		assert.Contains(t, prompt, kind.BriefCommand(target), "%s names its own brief command", kind)
		assert.Contains(t, prompt, "stop", "%s is told to stop after one unit", kind)
		assert.NotContains(t, prompt, "{", "the prompt is prose, not another template: %q", prompt)
	}

	assert.Empty(t, engine.UnitPrompt(engine.UnitKind("nonesuch"), target),
		"a kind with no brief command has no prompt, the same defensive direction BriefCommand takes")
}
