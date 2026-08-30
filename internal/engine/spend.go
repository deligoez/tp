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

