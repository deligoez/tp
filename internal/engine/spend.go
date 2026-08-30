package engine

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
)

// spendTailBytes bounds the read of a unit's log. The number the driver wants
// is on the last line, and a log is an agent's whole transcript — reading it
// whole to find one key would be paying for the output §3.5 keeps on disk
// precisely so nothing has to hold it.
//
// A final line longer than this is not guessed at: the tail then starts inside
// a line rather than at one, and a fragment is not a measurement.
const spendTailBytes = 1 << 20

// readSpend returns the number a unit's runner reported for that unit (§3.2.2):
// the key its runner declares — total_cost_usd for claude — read from the final
// line of the unit's log.
//
// Every way of not having a number returns nil rather than 0: a runner that
// declares no spend_key, a log that is absent or empty, a final line that is
// not JSON, a key the line does not carry, a value that is not a number. A row
// reporting 0 for any of them would be a cost nobody measured, and §3.5 keeps
// the field nullable for exactly that distinction.
//
// Reading one named key out of one line is not prose parsing (Non-Goal 6): the
// driver never reads a log into any context, and nothing here looks at a line
// the runner did not put its own number on.
func readSpend(logPath, spendKey string) *float64 {
	key := strings.TrimSpace(spendKey)
	if key == "" || logPath == "" {
		return nil
	}
	line := finalLogLine(logPath)
	if len(line) == 0 {
		return nil
	}
	return spendFromLine(line, key)
}

// finalLogLine returns the last non-blank line of a log, or nil when the file
// has none the reader can be sure of.
//
// Only the tail is read, and a tail that begins inside a line is refused: the
// bytes are then a suffix of a line rather than a line, and every JSON object
// has its opening brace at the front.
func finalLogLine(path string) []byte {
	file, err := os.Open(path) //nolint:gosec // the path is the driver's own, built from the run directory
	if err != nil {
		return nil
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil || info.Size() == 0 {
		return nil
	}
	from := int64(0)
	if info.Size() > spendTailBytes {
		from = info.Size() - spendTailBytes
	}
	buf := make([]byte, info.Size()-from)
	if _, err := file.ReadAt(buf, from); err != nil {
		return nil
	}

	tail := bytes.TrimRight(buf, " \t\r\n")
	if i := bytes.LastIndexByte(tail, '\n'); i >= 0 {
		return bytes.TrimSpace(tail[i+1:])
	}
	if from > 0 {
		// No newline anywhere in the tail: the final line is longer than
		// the bounded read, so what is held here is a fragment of it.
		return nil
	}
	return bytes.TrimSpace(tail)
}

// spendFromLine reads the dot path key out of one JSON line (§3.2's runner
// table), or nil when the line does not carry a number there.
//
// The path is walked rather than flattened, so a key naming a nested field
// resolves and a key naming an object, an array or a string resolves to
// nothing — the field is documented as a cost, and anything that is not a
// number is not one.
func spendFromLine(line []byte, key string) *float64 {
	var value any
	if err := json.Unmarshal(line, &value); err != nil {
		return nil
	}
	for _, part := range strings.Split(key, ".") {
		obj, isObject := value.(map[string]any)
		if !isObject {
			return nil
		}
		found, present := obj[part]
		if !present {
			return nil
		}
		value = found
	}
	amount, isNumber := value.(float64)
	if !isNumber {
		return nil
	}
	return &amount
}

// unmeteredKinds returns the unit kinds whose runner declares no spend_key
// (§3.2.2), in §3.3's table order.
//
// It asks the question of every kind rather than of the runner value's shape,
// because a per-kind map is exactly the case where the answer differs between
// kinds: the cap then accrues only the reporting ones, and naming the others is
// what keeps a partial total from reading as a complete one.
//
// The seam is read first, for the same reason ResolveUnitRunner reads it first:
// it replaces the field entirely, and it declares the claude template's key, so
// a run under it meters everything whatever the configuration says.
//
// A runner value the resolver rejects contributes no kind. It is a usage error
// that stops the run at its first spawn with its own message (§3.2), and a
// second, vaguer report of it here would be noise before that one.
func unmeteredKinds(raw json.RawMessage) []UnitKind {
	if _, seam := SeamRunner(); seam {
		return nil
	}
	var unmetered []UnitKind
	for _, kind := range unitKindOrder {
		spec, err := ResolveRunner(raw, kind)
		if err != nil {
			continue
		}
		runner := spec.Runner
		if runner == nil {
			if runner, err = BuiltinRunner(spec.Template, TemplateValues{Kind: kind}); err != nil {
				continue
			}
		}
		if strings.TrimSpace(runner.SpendKey) == "" {
			unmetered = append(unmetered, kind)
		}
	}
	return unmetered
}

// spendNotice is §3.2.2's run-start report, or "" for a run whose every kind is
// metered.
//
// A run that meters nothing names no kind: there is no partial total to
// explain, and listing all eight would spend the operator's attention on the
// less informative half of the message. A run that meters some of them names
// the rest, because that is the case where a total is real and incomplete.
func spendNotice(unmetered []UnitKind) string {
	switch {
	case len(unmetered) == 0:
		return ""
	case len(unmetered) == len(unitKindOrder):
		return "spend is not metered: the runner declares no spend_key, " +
			"so every unit reports spend null and run_max_budget_usd is inert"
	}
	names := make([]string, 0, len(unmetered))
	for _, kind := range unmetered {
		names = append(names, string(kind))
	}
	return "spend is not metered for " + strings.Join(names, ", ") +
		": their runner declares no spend_key, so run_max_budget_usd accrues only the other kinds"
}
