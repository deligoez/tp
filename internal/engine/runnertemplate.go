package engine

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// The two built-in runner templates §3.2.1 ships by name. They are the whole
// set: the shape resolver decides a runner value is a template name, and this
// layer decides whether it is a name tp knows.
const (
	TemplateClaude   = "claude"
	TemplateOpencode = "opencode"
)

// claudeSpendKey is the cost field the claude template's final log line
// carries. The template declares it rather than leaving the spend reader
// (§3.2.2) to special-case the name, so "a runner that declares a spend_key" is
// one rule covering the built-in and the operator's own alike. The opencode
// template deliberately declares none, which is what makes cap-budget inert
// for it.
const claudeSpendKey = "total_cost_usd"

// The placeholders §3.2's args template accepts, without their braces.
const (
	placeholderPrompt       = "prompt"
	placeholderMaxBudgetUSD = "max_budget_usd"
	placeholderUnitID       = "unit_id"
	placeholderUnitKind     = "unit_kind"
	placeholderLogPath      = "log_path"
)

// TemplateValues is what one unit attempt expands §3.2's placeholders to. It is
// the driver's side of the template contract: every field here is something the
// driver knows before it spawns anything, which is what lets a placeholder
// failure be raised before rather than during a spawn.
type TemplateValues struct {
	// Prompt is {prompt}: the fixed per-kind instruction UnitPrompt renders.
	Prompt string
	// Kind is {unit_kind}, the unit's kind.
	Kind UnitKind
	// ID is {unit_id}, the unit's durable subject.
	ID string
	// LogPath is {log_path}, the unit's log (§3.5). A template that uses it
	// owns the file; one that omits it gets the driver's redirect instead.
	LogPath string
	// MaxBudgetUSD is the resolved run_max_unit_budget_usd in dollars. It is
	// both {max_budget_usd} and the claude template's flag condition: 0
	// expands to a literal 0 for a positional template, and omits the flag
	// pair for claude.
	MaxBudgetUSD float64
}

// placeholders returns the value set keyed by placeholder name.
func (v TemplateValues) placeholders() map[string]string {
	return map[string]string{
		placeholderPrompt:       v.Prompt,
		placeholderMaxBudgetUSD: strconv.FormatFloat(v.MaxBudgetUSD, 'f', -1, 64),
		placeholderUnitID:       v.ID,
		placeholderUnitKind:     string(v.Kind),
		placeholderLogPath:      v.LogPath,
	}
}

// RunnerTemplateError is a runner tp cannot turn into a command line: a
// template name that is not one of the two built-ins, or a placeholder the
// driver cannot resolve. Like the shape error it is a usage error (exit 2) —
// the configuration does not say what to spawn, and no retry can change that —
// but it carries its own hint, because the answer is a list of names or of
// placeholders rather than §3.2's three shapes.
type RunnerTemplateError struct {
	// Field names the place at fault: "runner" for the template name,
	// "runner.args[N]" for the argument holding the offending placeholder, so
	// a long args template does not have to be scanned by eye.
	Field string
	// Msg says what is wrong with that place.
	Msg string
	// HintMsg is the actionable hint. It is a field rather than a fixed string
	// because the two failures have different answers.
	HintMsg string
}

func (e *RunnerTemplateError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}

// Hint is the actionable hint surfaced to the agent alongside exit 2.
func (e *RunnerTemplateError) Hint() string {
	return e.HintMsg
}

// templateNamesHint lists the templates tp ships, for the unknown-name error.
const templateNamesHint = `tp ships two built-in templates, "` + TemplateClaude + `" and "` + TemplateOpencode +
	`"; any other harness is configured as a runner object carrying cmd`

// placeholdersHint lists the placeholders that do resolve, for the
// unresolved-placeholder error. It is written out rather than joined from the
// constants so the hint reads in §3.2's table order and needs no allocation.
const placeholdersHint = "an args template resolves {prompt}, {max_budget_usd}, {unit_id}, {unit_kind} and {log_path}; " +
	"anything else in braces is either a typo or a value the runner has to carry in its own env"

// BuiltinRunner returns the runner one of §3.2.1's two built-in templates
// expands to for a unit.
//
// The claude template carries the documented argv — the prompt, the streaming
// JSON output an unattended driver can read, and --permission-mode auto,
// because a child that stops to ask for permission is exactly the silent hang
// this version exists to prevent — with the budget flag pair appended only when
// the resolved run_max_unit_budget_usd is non-zero. A budget of 0 omits the
// pair entirely rather than passing a literal 0, which a harness would read as
// "spend nothing" rather than as "no cap". The comparison is > 0 rather than
// != 0 for the same reason: §7 clamps the field to 0-1000 so nothing below zero
// can arrive, and were one to, omitting the flag is a better answer than
// handing a harness a negative cap.
//
// The opencode template is `run` with the prompt: no budget flag and no
// spend_key.
//
// v is taken whole rather than as a single budget, because the per-kind
// argument §6.3 adds to the claude template reads v.Kind, and a template that
// already has the unit's values needs no new parameter to grow one.
func BuiltinRunner(name string, v TemplateValues) (*Runner, error) {
	switch name {
	case TemplateClaude:
		args := []string{
			"-p", "{" + placeholderPrompt + "}",
			"--output-format", "stream-json",
			"--verbose",
			"--permission-mode", "auto",
		}
		if v.MaxBudgetUSD > 0 {
			args = append(args, "--max-budget-usd", "{"+placeholderMaxBudgetUSD+"}")
		}
		return &Runner{Cmd: TemplateClaude, Args: args, SpendKey: claudeSpendKey}, nil
	case TemplateOpencode:
		return &Runner{Cmd: TemplateOpencode, Args: []string{"run", "{" + placeholderPrompt + "}"}}, nil
	default:
		return nil, &RunnerTemplateError{
			Field:   runnerField,
			Msg:     fmt.Sprintf("%q is not a built-in template", name),
			HintMsg: templateNamesHint,
		}
	}
}

// ExpandArgs expands §3.2's placeholders through an argument template, reporting
// a placeholder it cannot resolve as a usage error (§3.2.1).
//
// Each argument is scanned once and substituted values are never rescanned: a
// prompt is agent-facing prose the driver composes, and a brace token inside it
// is text rather than a second round of templating. Braces that are not
// placeholder-shaped — a JSON literal, an empty pair, a spaced-out name — are
// ordinary characters, so only something that really looks like a placeholder
// can be reported as one.
func ExpandArgs(args []string, v TemplateValues) ([]string, error) {
	values := v.placeholders()
	out := make([]string, 0, len(args))
	for i, arg := range args {
		expanded, err := expandArg(arg, values, i)
		if err != nil {
			return nil, err
		}
		out = append(out, expanded)
	}
	return out, nil
}

// expandArg expands one argument. index names the argument in an error, so the
// operator is pointed at the entry rather than at the template as a whole.
func expandArg(arg string, values map[string]string, index int) (string, error) {
	var b strings.Builder
	for i := 0; i < len(arg); {
		if arg[i] != '{' {
			b.WriteByte(arg[i])
			i++
			continue
		}
		end := strings.IndexByte(arg[i:], '}')
		if end < 0 {
			// No closing brace at all: the rest is literal text.
			b.WriteString(arg[i:])
			break
		}
		token := arg[i : i+end+1]
		name := token[1 : len(token)-1]
		if !placeholderShaped(name) {
			b.WriteString(token)
			i += end + 1
			continue
		}
		value, ok := values[name]
		if !ok {
			return "", &RunnerTemplateError{
				Field:   fmt.Sprintf("%s.args[%d]", runnerField, index),
				Msg:     fmt.Sprintf("%q is not a placeholder tp resolves", token),
				HintMsg: placeholdersHint,
			}
		}
		b.WriteString(value)
		i += end + 1
	}
	return b.String(), nil
}

// placeholderShaped reports whether a braced name is shaped like a placeholder
// rather than like ordinary text that happens to sit between braces.
//
// The test is deliberately narrow on both sides. It admits the case and hyphen
// variants an operator mistypes — {Prompt}, {unit-id} — because reporting a
// near miss is the whole point of the error; and it rejects anything carrying a
// space, a quote or a colon, so a JSON literal in an args template passes
// through untouched.
func placeholderShaped(name string) bool {
	if name == "" || !isASCIILetter(name[0]) {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if isASCIILetter(c) || (c >= '0' && c <= '9') || c == '_' || c == '-' {
			continue
		}
		return false
	}
	return true
}

// isASCIILetter reports whether c is an ASCII letter of either case.
func isASCIILetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ResolveUnitRunner resolves the runner workflow field to the concrete command
// one unit is spawned with: §3.2's shape resolution, then §3.2.1's template
// expansion, then every placeholder in the argv.
//
// It is a pure function that returns argv and spawns nothing, which is how
// §3.2.1's "raised before any child is spawned" is kept structurally rather
// than by call ordering: the driver cannot reach an exec without first holding
// a fully resolved command line, so a bad template or an unresolvable
// placeholder stops the run at start rather than three hours in.
//
// The returned runner is a copy: its Args are the expanded ones, and the
// caller may not mutate the shared Env map.
func ResolveUnitRunner(raw json.RawMessage, v TemplateValues) (*Runner, error) {
	spec, err := ResolveRunner(raw, v.Kind)
	if err != nil {
		return nil, err
	}

	runner := spec.Runner
	if runner == nil {
		if runner, err = BuiltinRunner(spec.Template, v); err != nil {
			return nil, err
		}
	}

	args, err := ExpandArgs(runner.Args, v)
	if err != nil {
		return nil, err
	}

	resolved := *runner
	resolved.Args = args
	return &resolved, nil
}

// UnitPrompt returns the fixed per-kind instruction {prompt} expands to
// (§3.2.1): run this unit's brief command, do that one unit, stop.
//
// The driver owns this text and never executes the brief command itself, nor
// reads its output — the child runs it as its first act, and the brief tp
// already emits is where every instruction comes from. Keeping the prompt this
// short is the point: anything the driver says here is a second, drifting copy
// of the brief.
//
// A kind outside the eight has no brief command and so has no prompt, the same
// defensive direction BriefCommand and DurableWrite take.
func UnitPrompt(kind UnitKind, t UnitTarget) string {
	brief := kind.BriefCommand(t)
	if brief == "" {
		return ""
	}
	return fmt.Sprintf(
		"Run `%s` first and follow the brief it returns; it is where your instructions come from. "+
			"Do that one %s unit and then stop: do not claim another unit, and do no work the brief did not ask for.",
		brief, kind)
}
