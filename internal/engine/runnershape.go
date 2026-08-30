package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Runner is §3.2's runner object: the harness is configuration, not a hard
// dependency, so everything the driver needs to spawn a child is data.
//
//	cmd        the executable to spawn
//	args       the argument template, carrying §3.2.1's placeholders
//	env        extra environment for the child, merged over the parent's
//	spend_key  dot path to the cost field in the runner's final log line
//
// Only cmd is required — a runner with no executable is not a runner. Args and
// env are the templates task's business (§3.2.1) and spend_key the spend
// reader's (§3.2.2); this file only carries them.
type Runner struct {
	Cmd      string            `json:"cmd"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	SpendKey string            `json:"spend_key,omitempty"`
}

// RunnerSpec is what the runner field resolved to for one unit kind. Exactly
// one of the two fields is set: Template names a built-in template, whose argv
// the template layer expands, and Runner is a runner object given in full. The
// per-kind map is not a third case here — it is resolved away, since a map
// dispatches to one of these two for the kind being spawned.
type RunnerSpec struct {
	// Template is the built-in template's name. Whether that name is one tp
	// ships is the template layer's judgement (§3.2.1), not this resolver's:
	// the shape is what is decided here.
	Template string
	// Runner is the runner object, non-nil only when the shape gave one.
	Runner *Runner
}

// RunnerShapeError is a runner value that is none of §3.2's three shapes — a
// map missing default, a runner object missing cmd, a key that is not a unit
// kind, a value that is neither a name nor an object. It is a usage error, so
// the CLI maps it to exit 2: nothing about the run state is wrong, the
// configuration simply does not say what to spawn, and no retry or lock wait
// can change that.
type RunnerShapeError struct {
	// Field names the place in the runner value at fault: "runner" for the
	// value as a whole, "runner.<key>" for one entry of a per-kind map, so an
	// operator with a large map is not left to find the bad entry by eye.
	Field string
	// Msg says what is wrong with that place.
	Msg string
}

func (e *RunnerShapeError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Msg)
}

// Hint is the actionable hint surfaced to the agent alongside exit 2. It is one
// string for every shape error because the answer is always the same: the
// three shapes and the key that tells the two object ones apart.
func (e *RunnerShapeError) Hint() string {
	return `runner takes one of three shapes: a built-in template name ("claude" or "opencode"), ` +
		`a runner object carrying cmd, or a map from unit kind to either of those with a "default" key ` +
		`covering the kinds it does not list`
}

// runnerField is the name the errors report for the runner value as a whole.
const runnerField = "runner"

// runnerDefaultKey is the per-kind map's catch-all key: the entry every unit
// kind the map does not list resolves to. A map without it is a usage error,
// because the driver would otherwise have nothing to spawn the moment the
// cycle reached an unlisted kind — hours in, not at run start.
const runnerDefaultKey = "default"

// ResolveRunner resolves the runner workflow field to the runner one unit kind
// spawns with (§3.2).
//
// The field takes three shapes and they are told apart by their JSON, not by
// configuration: a string is a built-in template name; an object carrying cmd
// is a runner object; any other object is a per-kind map. That single key is
// the whole discriminator between the two object shapes, which is why an
// object carrying both cmd and default is a runner and not a map — cmd decides.
//
// An absent value (nil, empty, or a literal null) is the built-in default
// template, matching the workflow layer, which stores exactly that when no
// layer sets the field.
//
// A per-kind map is validated whole, not just at the entry being selected: an
// operator who mistyped the audit branch learns at run start rather than when
// the first audit unit is spawned three hours later.
func ResolveRunner(raw json.RawMessage, kind UnitKind) (RunnerSpec, error) {
	value := bytes.TrimSpace(raw)
	if len(value) == 0 || string(value) == "null" {
		return RunnerSpec{Template: RunnerDefault}, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(value, &obj); err != nil {
		// Not an object: the only other shape is the template name, so a
		// failure here is either that name or nothing tp can use.
		return parseRunnerLeaf(value, runnerField)
	}

	if _, hasCmd := obj["cmd"]; hasCmd {
		return parseRunnerObject(value, runnerField)
	}
	return resolveRunnerMap(obj, kind)
}

// resolveRunnerMap resolves a per-kind map: every key must be a unit kind or
// default, every value must be a template name or a runner object, and the
// selected kind's entry — or default, for a kind the map does not list — is
// what the unit spawns with.
func resolveRunnerMap(obj map[string]json.RawMessage, kind UnitKind) (RunnerSpec, error) {
	// Sorted, so a map with two bad entries always reports the same one.
	for _, key := range slices.Sorted(maps.Keys(obj)) {
		if key != runnerDefaultKey {
			if _, ok := ParseUnitKind(key); !ok {
				return RunnerSpec{}, &RunnerShapeError{
					Field: runnerField + "." + key,
					Msg:   "not a unit kind; a per-kind runner map keys on the eight unit kinds plus " + runnerDefaultKey,
				}
			}
		}
		if _, err := parseRunnerLeaf(obj[key], runnerField+"."+key); err != nil {
			return RunnerSpec{}, err
		}
	}

	// The missing key is a property of the map, not of the kind that happens to
	// be resolving, so it is reported whether or not that kind is listed. A
	// check made only on the way to the default entry would clear the map for
	// every listed kind and leave the run to fail at the first unlisted one —
	// exactly the lateness whole-map validation exists to prevent.
	if _, ok := obj[runnerDefaultKey]; !ok {
		return RunnerSpec{}, &RunnerShapeError{
			Field: runnerField,
			Msg: fmt.Sprintf("a per-kind runner map needs a %q key covering the kinds it does not list",
				runnerDefaultKey),
		}
	}

	selected, field := obj[string(kind)], runnerField+"."+string(kind)
	if selected == nil {
		selected, field = obj[runnerDefaultKey], runnerField+"."+runnerDefaultKey
	}
	return parseRunnerLeaf(selected, field)
}

// parseRunnerLeaf resolves one of the two leaf shapes — a template name or a
// runner object — from the value at field. A per-kind map's entries are leaves
// too, which is why a map cannot nest inside a map: an object there carries no
// cmd and is reported as the runner object it has to be.
func parseRunnerLeaf(value json.RawMessage, field string) (RunnerSpec, error) {
	var name string
	if err := json.Unmarshal(value, &name); err == nil {
		if strings.TrimSpace(name) == "" {
			return RunnerSpec{}, &RunnerShapeError{Field: field, Msg: "empty template name"}
		}
		return RunnerSpec{Template: name}, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(value, &obj); err != nil {
		return RunnerSpec{}, &RunnerShapeError{
			Field: field,
			Msg:   "expected a built-in template name or a runner object",
		}
	}
	return parseRunnerObject(value, field)
}

// parseRunnerObject reads a runner object and enforces its one requirement:
// something to spawn. An object that reaches here without a usable cmd is the
// "runner object missing cmd" §3.2 calls a usage error — including the object
// that never carried the key at all, which is how a map value that is neither
// a name nor a runner is reported.
func parseRunnerObject(value json.RawMessage, field string) (RunnerSpec, error) {
	var runner Runner
	if err := json.Unmarshal(value, &runner); err != nil {
		return RunnerSpec{}, &RunnerShapeError{
			Field: field,
			Msg:   "cannot be read as a runner object: " + err.Error(),
		}
	}
	if strings.TrimSpace(runner.Cmd) == "" {
		return RunnerSpec{}, &RunnerShapeError{
			Field: field,
			Msg:   "a runner object needs a cmd to spawn",
		}
	}
	return RunnerSpec{Runner: &runner}, nil
}
