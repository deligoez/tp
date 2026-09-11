package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/deligoez/tp/internal/output"
)

// mergeInputCounts is one input file's share of a merge (§8a.4): how many of its
// content lines became a usable row, and how many were skipped as malformed or
// incomplete. Blank and whitespace-only lines are neither parsed nor skipped, so
// a file padded with newlines never reads as a dropped role. So is a line
// holding an empty JSON array (isEmptyJSONArray), at both merges.
//
// Why and where each line was skipped is kept beside the counts and never
// reaches the payload: the per-line warnings and the dropped-input error carry
// it, which is where an operator acts on it, and `inputs` stays the
// {path, parsed, skipped} it has been since v0.35.0.
type mergeInputCounts struct {
	Path    string `json:"path"`
	Parsed  int    `json:"parsed"`
	Skipped int    `json:"skipped"`
	skips   []mergeSkip
}

// mergeSkip is one reason an input's lines were skipped and the 1-based file
// lines it covers, ascending. Line numbers count every line of the file, blank
// ones included, so they are the ones an editor shows.
type mergeSkip struct {
	reason string
	lines  []int
}

// skipInvalidJSON is the reason a line that does not parse is skipped. Every
// other reason names the required fields the row lacks.
const skipInvalidJSON = "invalid JSON"

// skipLine counts line n as skipped for reason, warns on stderr naming the file
// and the line, and files the line under its reason for the error a dropped
// input gets.
func (c *mergeInputCounts) skipLine(n int, reason string) {
	kind := "incomplete"
	if reason == skipInvalidJSON {
		kind = "malformed"
	}
	fmt.Fprintf(os.Stderr, "warning: skipping %s line (%s) in %s:%d\n", kind, reason, c.Path, n)
	c.Skipped++
	for i := range c.skips {
		if c.skips[i].reason == reason {
			c.skips[i].lines = append(c.skips[i].lines, n)
			return
		}
	}
	c.skips = append(c.skips, mergeSkip{reason: reason, lines: []int{n}})
}

// mergeRowRule is what one merge requires of a row: the noun and the fields its
// refusal hint names, and the predicate returning the fields a row lacks.
type mergeRowRule struct {
	noun     string
	required []string
	missing  func(row map[string]any) []string
}

// loadMergeRows reads and validates the rows of every input file, skipping
// blank, malformed (invalid JSON), and incomplete lines — the last by the
// merge's own rule — with a stderr warning that names which and where. It
// aborts only on a missing/unreadable file (exit 3), and returns the §8a.4
// per-input accounting beside the rows: blank lines and any empty JSON array
// count as neither, so an all-empty set of inputs is a valid clean result and
// yields zero rows without failing, and the merge→record chain works on a
// clean round. An input whose content lines all fail is a dropped role, which
// the merge turns into exit 1.
func loadMergeRows(args []string, rule mergeRowRule) ([]map[string]any, []mergeInputCounts) {
	for _, path := range args {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			output.Error(ExitFile, fmt.Sprintf("file not found: %s", path), ndjsonInputFileHint)
			os.Exit(ExitFile)
		}
	}

	all := make([]map[string]any, 0)
	inputs := make([]mergeInputCounts, 0, len(args))
	for _, path := range args {
		f, err := os.Open(path)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("cannot open file: %s", path), ndjsonInputFileHint)
			os.Exit(ExitFile)
		}
		rows, counts, scanErr := scanMergeInput(f, path, rule.missing)
		f.Close()
		if scanErr != nil {
			// Aborting, not warning: a read that failed produces zero rows,
			// and zero rows is also what a clean round looks like — so a
			// swallowed read error lets an input tp never read record one.
			// The old audit warning also named one cause (an over-long line)
			// for every failure, including reading a directory.
			output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", path, scanErr), ndjsonReadHint(scanErr))
			os.Exit(ExitFile)
		}
		all = append(all, rows...)
		inputs = append(inputs, counts)
	}

	return all, inputs
}

// scanMergeInput reads one already-opened input and returns its usable rows
// beside that input's §8a.4 accounting. It warns on stderr about each line it
// skips and returns the scanner's read error, if any, for the caller to act on.
func scanMergeInput(f *os.File, path string, missingFields func(map[string]any) []string) ([]map[string]any, mergeInputCounts, error) {
	counts := mergeInputCounts{Path: path}
	rows := make([]map[string]any, 0)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap) // audit notes can be long
	n := 0
	for scanner.Scan() {
		n++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			// A line holding an empty JSON array counts as neither parsed
			// nor skipped, like a blank one: it is a role saying it found
			// nothing. tp's own code-audit prompt asked for exactly that,
			// and counting it as a skip made the file a dropped role — which
			// fails the whole merge and, since §5 row 10, writes no `-o`, so
			// one clean role took the panel down. The prompt no longer asks
			// for it; this stays for the rounds run from prompts an older
			// binary emitted.
			//
			// The test is a parse rather than a comparison against `[]`
			// because TrimSpace removes only the outer padding: `[ ]`
			// survives it unchanged and was still dropping the role. A
			// NON-empty array is left where it was — that is a role whose
			// findings would be lost silently, so it stays a skip.
			if !isEmptyJSONArray(line) {
				counts.skipLine(n, skipInvalidJSON)
			}
			continue
		}
		if missing := missingFields(row); len(missing) > 0 {
			counts.skipLine(n, "missing "+strings.Join(missing, ", "))
			continue
		}
		counts.Parsed++
		rows = append(rows, row)
	}
	return rows, counts, scanner.Err()
}

// invalidJSONHint is the repair for lines that did not parse: the file is where
// the operator said it was and tp read all of it, so the repair is in whatever
// wrote the file.
const invalidJSONHint = "re-emit the invalid JSON lines as one JSON object per line (a trailing comma or a wrapping array breaks every line at once)"

// maxLineGroups caps how many groups of lines one reason lists in the
// dropped-input error, so a file of hundreds of skipped lines still gets a
// one-line diagnosis.
const maxLineGroups = 8

// droppedInputs returns the inputs that had at least one content line and
// parsed none of them — the shape a whole role's file takes when its emitter
// got the format wrong, which used to merge clean and freeze an undercounted
// round (§8a.4). A zero-byte or blank-only file has nothing to drop and is
// never returned: that stays the documented way a role reports nothing found.
func droppedInputs(inputs []mergeInputCounts) []mergeInputCounts {
	dropped := make([]mergeInputCounts, 0, len(inputs))
	for _, in := range inputs {
		if in.Parsed == 0 && in.Skipped > 0 {
			dropped = append(dropped, in)
		}
	}
	return dropped
}

// droppedInputsMessage names every dropped input with each reason its lines
// were skipped and the lines, in one message, so one exit diagnoses them all.
func droppedInputsMessage(dropped []mergeInputCounts) string {
	named := make([]string, 0, len(dropped))
	for _, in := range dropped {
		reasons := make([]string, 0, len(in.skips))
		for _, s := range in.skips {
			reasons = append(reasons, s.reason+": "+lineList(s.lines))
		}
		named = append(named, fmt.Sprintf("%s (%s)", in.Path, strings.Join(reasons, "; ")))
	}
	who := "that input"
	if len(dropped) > 1 {
		who = "those inputs"
	}
	return fmt.Sprintf("no line parsed in %s, so %s contributed nothing to the merge", strings.Join(named, ", "), who)
}

// droppedInputsHint gives the repair for the reasons the dropped inputs'
// lines were skipped, and only those. The format advice used to be the whole
// hint: a file of valid rows that each lacked `evidence` was told to fix a
// trailing comma it did not have.
func droppedInputsHint(dropped []mergeInputCounts, rule mergeRowRule) string {
	var badJSON, lacking bool
	for _, in := range dropped {
		for _, s := range in.skips {
			if s.reason == skipInvalidJSON {
				badJSON = true
			} else {
				lacking = true
			}
		}
	}
	parts := make([]string, 0, 2)
	if lacking {
		parts = append(parts, fmt.Sprintf("each %s needs a non-empty %s: add what the named lines lack", rule.noun, joinAnd(rule.required)))
	}
	if badJSON {
		parts = append(parts, invalidJSONHint)
	}
	return strings.Join(parts, "; ") + ", then merge again"
}

// lineList renders ascending line numbers compactly — "line 4", "lines 1-3,7"
// — listing at most maxLineGroups groups of consecutive lines and counting the
// rest.
func lineList(lines []int) string {
	groups := make([]string, 0, maxLineGroups)
	listed := 0
	for i := 0; i < len(lines) && len(groups) < maxLineGroups; {
		j := i
		for j+1 < len(lines) && lines[j+1] == lines[j]+1 {
			j++
		}
		if j == i {
			groups = append(groups, strconv.Itoa(lines[i]))
		} else {
			groups = append(groups, fmt.Sprintf("%d-%d", lines[i], lines[j]))
		}
		listed += j - i + 1
		i = j + 1
	}
	s := "lines " + strings.Join(groups, ",")
	if len(lines) == 1 {
		s = "line " + groups[0]
	}
	if rest := len(lines) - listed; rest > 0 {
		s += fmt.Sprintf(" and %d more", rest)
	}
	return s
}

// joinAnd joins items as prose: "a", "a and b", "a, b and c".
func joinAnd(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// writeMergeOutput writes `tp review --merge`'s NDJSON to `-o`, or refuses to
// touch that path at all when the merge is already going to exit non-zero (§5
// row 10). It reports whether it wrote, so the caller names `output_path` in
// the summary only when a file is actually there. `tp audit --merge` does not
// call it: §4 fences this release out of the audit phase, so that merge still
// writes before it refuses.
//
// The refusal is keyed on `dropped`, never on how many rows survived: a
// converged round's inputs hold no content line, drop nothing, and still get
// the zero-byte `-o` the review loop reads as "nothing found".
//
// The write itself goes to a temporary file in the destination's own directory
// and is renamed on success, so a failed write leaves no truncated `-o`. It
// does not always leave no temporary, and there are two cases it cannot clean:
// a removal that fails for the same reason the rename did (measured in the
// field as EPERM on both, with the temporary locked mid-write), and a SIGKILL
// between create and rename. The first now names the temporary in the error;
// nothing can report the second.
//
// Further consequences of writing this way, measured against v1.0.1 —
// accepted rather than repaired:
//
//   - The usable `-o` basename is 235, not the filesystem's 255: tp appends
//     `.tp-merge-` plus os.CreateTemp's random suffix, which is 9 or 10 digits.
//     235 succeeded on 20 runs of 20; 236 failed on 16 of 20 — the boundary is
//     intermittent, not sharp, because the suffix length varies.
//   - A writable `-o` inside a NON-writable directory merged at v1.0.1 and now
//     exits 3: the temporary has to be created in that directory.
//   - A read-only (0444) `-o` was refused at v1.0.1 and is now replaced: rename
//     needs permission on the directory, not on the file it overwrites.
//   - An existing `-o` comes out at 0600 whatever mode it had. The renamed file
//     carries os.CreateTemp's mode, not the destination's; the v1.0.1
//     os.WriteFile(path, data, 0o600) passed its mode only on create and left
//     an existing file's own. Measured both ways: 0644 → 0600 and 0666 → 0600
//     here, against 0644 → 0644 under the WriteFile equivalent.
func writeMergeOutput(outputPath, ndjson string, dropped []mergeInputCounts) bool {
	if len(dropped) > 0 {
		return false
	}
	// The single os.WriteFile this replaced refused an `-o` naming an existing
	// directory with "open <path>: is a directory". Written through a
	// temporary and a rename, the same path gets as far as the rename and
	// reports "rename <path>.tp-merge-NNN <path>: file exists" — which names
	// tp's own temporary and diagnoses nothing, beside a standing hint whose
	// every word is true of that path. Checked up front, so the message is the
	// old one and no temporary is created to leave behind.
	if fi, statErr := os.Stat(outputPath); statErr == nil && fi.IsDir() {
		failMergeOutput(fmt.Errorf("open %s: is a directory", outputPath))
		return false
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), filepath.Base(outputPath)+".tp-merge-*")
	if err != nil {
		failMergeOutput(err)
		return false
	}
	// One failure path, so the temporary file is removed however the write
	// ends: a partial temporary left beside `-o` is the thing this shape is
	// here to prevent.
	name := tmp.Name()
	_, err = tmp.WriteString(ndjson)
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(name, outputPath)
	}
	if err != nil {
		removeErr := os.Remove(name)
		failMergeOutput(mergeWriteFailure(err, removeErr, name))
		return false
	}
	return true
}

// mergeWriteFailure composes the error a failed `-o` write reports, naming the
// temporary when the cleanup failed too.
//
// The removal error itself is of no use to the operator — it is the same EPERM
// the rename already reported — but the PATH is: the temporary carries a name
// nobody chose and nothing else in tp mentions, so a message that omits it
// leaves a file that cannot be found except by looking. There is no retry here
// on purpose: whatever refused the removal will refuse it again.
func mergeWriteFailure(writeErr, removeErr error, tmpPath string) error {
	if removeErr == nil {
		return writeErr
	}
	return fmt.Errorf("%w (the temporary %s could not be removed either, and nothing else will remove it)", writeErr, tmpPath)
}

// failMergeOutput reports an unwritable `-o` at exit 3, the code and hint the
// single os.WriteFile call this replaced already used.
func failMergeOutput(err error) {
	output.Error(ExitFile, fmt.Sprintf("cannot write output file: %s", err), outputFileHint)
	os.Exit(ExitFile)
}

// finishMerge applies §8a.4's exit rule once the merge has emitted its payload,
// and both merges still reach it the same way. What changed under it is what a
// caller has already written: for `tp review --merge`, §5 row 10 makes the
// payload the summary alone, because `-o` is the file the next command in a
// review loop reads — so a refused review merge leaves that path exactly as it
// found it, writeMergeOutput having declined to write it.
//
// That supersedes §8a.4's stated rationale for the review side, which kept the
// write because "the accounting an operator reads is in that payload": the
// accounting reaches stdout either way, and it was the file, not the
// accounting, that let a refused merge be chained into `--record` as a clean
// round. The rationale still stands for `tp audit --merge`, which §4 fences
// out of this release and which therefore still writes `-o` before refusing.
//
// encodeErr is the payload's own write error and is reported first: a merge
// that could not emit has nothing to say about its inputs. The hint is always
// the merge's own, never the exit-1 default ("run 'tp validate' to audit the
// task file"), which names a file this mode never touches.
func finishMerge(encodeErr error, dropped []mergeInputCounts, rule mergeRowRule) error {
	if encodeErr != nil {
		return encodeErr
	}
	if len(dropped) == 0 {
		return nil
	}
	output.Error(ExitValidation, droppedInputsMessage(dropped), droppedInputsHint(dropped, rule))
	os.Exit(ExitValidation)
	return nil
}
