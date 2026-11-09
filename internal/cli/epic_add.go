package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicWriteRefusedHint answers a write that would add a lint error.
const epicWriteRefusedHint = "fix the payload so it adds no lint error; tp epic lint shows the errors the file already holds"

// epicWriteResult is every epic writer's payload: the file it wrote, the
// number of epics the file holds after the write, the paths of the keys
// outside the schema it did not carry into the file, the file's revision
// after the write, and for link and unlink what the write did.
type epicWriteResult struct {
	File        string   `json:"file"`
	Epics       int      `json:"epics"`
	DroppedKeys []string `json:"dropped_keys"`
	Revision    string   `json:"revision"`
	Action      string   `json:"action,omitempty"`
}

func newEpicAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [epic-json]",
		Short: "Add an epic to .tp/epics.json; a write that adds a lint error exits 1 and writes nothing",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runEpicAdd,
	}
	cmd.Flags().Bool("stdin", false, "read the epic JSON from stdin")
	cmd.Flags().String("bulk", "", "add one epic per NDJSON line; a row whose id exists replaces that epic")
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicAdd implements tp epic add (spec/1.3.0.md §3).
func runEpicAdd(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic add")
	refuseTwoEpicInputs(cmd, args)
	bulk, _ := cmd.Flags().GetString("bulk")
	opts := epicWriteOpts{ifRevision: epicIfRevision(cmd)}
	var epics []model.Epic
	if bulk != "" {
		epics, opts.bulkLines = readBulkEpics(bulk)
	} else {
		epics = []model.Epic{decodeEpicArg(cmd, args)}
	}
	writeEpicGraphWith(opts, func(g *model.EpicGraph) error {
		engine.AddEpics(g, epics, bulk != "")
		return nil
	})
	return nil
}

// refuseTwoEpicInputs exits 2 when more than one of the argument, --stdin and
// --bulk is given: reading one would silently drop the other epic.
func refuseTwoEpicInputs(cmd *cobra.Command, args []string) {
	stdin, _ := cmd.Flags().GetBool("stdin")
	bulk, _ := cmd.Flags().GetString("bulk")
	sources := []struct {
		on   bool
		name string
	}{
		{len(args) > 0, "an epic JSON argument"},
		{stdin, "--stdin"},
		{bulk != "", "--bulk"},
	}
	for i := range sources {
		for j := i + 1; j < len(sources); j++ {
			if sources[i].on && sources[j].on {
				output.Error(ExitUsage, sources[i].name+" cannot be combined with "+sources[j].name,
					"pass the epic exactly one way: as an argument, via --stdin, or as --bulk <ndjson>")
				os.Exit(ExitUsage)
			}
		}
	}
}

// decodeEpicArg reads the single epic from the argument or stdin; invalid
// JSON is a usage error.
func decodeEpicArg(cmd *cobra.Command, args []string) model.Epic {
	stdin, _ := cmd.Flags().GetBool("stdin")
	var raw []byte
	switch {
	case len(args) > 0:
		raw = []byte(args[0])
	case stdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("read stdin: %v", err), runtimeFailureHint)
			os.Exit(ExitFile)
		}
		raw = data
	default:
		output.Error(ExitUsage, "epic JSON required as argument, via --stdin, or as --bulk <ndjson>")
		os.Exit(ExitUsage)
	}
	e, err := decodeEpic(raw)
	if err != nil {
		output.Error(ExitUsage, "invalid epic JSON", err.Error())
		os.Exit(ExitUsage)
	}
	return e
}

// decodeEpic parses one epic, refusing keys outside the schema and anything
// after the one JSON value.
func decodeEpic(raw []byte) (model.Epic, error) {
	var e model.Epic
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return e, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return e, errors.New("trailing data after the epic: pass exactly one JSON object")
	}
	return e, nil
}

// epicBulkFileHint answers an unreadable --bulk path. The shared
// ndjsonInputFileHint names the reviewers'/auditors' files, the wrong input
// for epic rows.
const epicBulkFileHint = "check the --bulk path — it takes an NDJSON file, one epic JSON object per line"

// epicBulkLineTooLongHint answers a --bulk line over ndjsonLineCap. The shared
// ndjsonLineTooLongHint asks for one finding per line and a shorter note,
// neither of which a file of epic rows holds.
const epicBulkLineTooLongHint = "a --bulk line exceeded the 1MB NDJSON read cap — write one epic JSON object per line and shorten the oversized one"

// readBulkEpics reads one epic per non-blank NDJSON line; any bad line aborts
// the whole add, naming it. lines maps each id to the 1-based physical lines
// that hold it, so a refused write can point back into the payload.
func readBulkEpics(path string) (epics []model.Epic, lines map[string][]int) {
	data, err := os.ReadFile(path)
	if err != nil {
		output.Error(ExitFile, err.Error(), epicBulkFileHint)
		os.Exit(ExitFile)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
	lines = map[string][]int{}
	for lineNum := 1; scanner.Scan(); lineNum++ {
		if strings.TrimSpace(scanner.Text()) == "" {
			continue
		}
		e, err := decodeEpic(scanner.Bytes())
		if err != nil {
			output.Error(ExitUsage, fmt.Sprintf("line %d: invalid epic JSON", lineNum), err.Error())
			os.Exit(ExitUsage)
		}
		epics = append(epics, e)
		lines[e.ID] = append(lines[e.ID], lineNum)
	}
	if err := scanner.Err(); err != nil {
		hint := epicBulkFileHint
		if errors.Is(err, bufio.ErrTooLong) {
			hint = epicBulkLineTooLongHint
		}
		output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", path, err), hint)
		os.Exit(ExitFile)
	}
	return epics, lines
}

// epicIfRevisionFlag registers --if-revision on a graph writer.
func epicIfRevisionFlag(cmd *cobra.Command) {
	cmd.Flags().String("if-revision", "", "write only if the graph file's revision (lowercase hex SHA-256 of its bytes; \"\" when absent) is this one")
}

// epicIfRevision returns the --if-revision value when the flag was given,
// the empty string included, and nil when it was not.
func epicIfRevision(cmd *cobra.Command) *string {
	if !cmd.Flags().Changed("if-revision") {
		return nil
	}
	v, _ := cmd.Flags().GetString("if-revision")
	return &v
}

// epicWriteOpts are what a writer adds to the shared write: the revision it
// read, the payload lines of a --bulk write, and the action a link or unlink
// reports, which mutate fills in.
type epicWriteOpts struct {
	ifRevision *string
	bulkLines  map[string][]int
	action     *string
}

// writeEpicGraph runs one locked, lint-checked write of .tp/epics.json with
// the command's --if-revision and reports it.
func writeEpicGraph(cmd *cobra.Command, mutate func(*model.EpicGraph) error) {
	writeEpicGraphWith(epicWriteOpts{ifRevision: epicIfRevision(cmd)}, mutate)
}

// writeEpicGraphWith runs one locked, lint-checked write of .tp/epics.json
// and reports it: exit 1 naming the new errors, 4 naming an absent epic or a
// stale revision, 3 on a file error.
func writeEpicGraphWith(opts epicWriteOpts, mutate func(*model.EpicGraph) error) {
	count := 0
	counted := func(g *model.EpicGraph) error {
		if err := mutate(g); err != nil {
			return err
		}
		count = len(g.Epics)
		return nil
	}
	out, err := engine.UpdateEpicGraph(engine.ProjectConfigDir("."), opts.ifRevision, counted)
	if err != nil {
		exitEpicWriteError(err, opts.bulkLines)
	}
	result := epicWriteResult{File: out.Path, Epics: count, DroppedKeys: out.Dropped, Revision: out.Revision}
	if opts.action != nil {
		result.Action = *opts.action
	}
	if err := output.JSON(result); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}

// exitEpicWriteError maps a refused or failed graph write to its exit code.
func exitEpicWriteError(err error, bulkLines map[string][]int) {
	var refused *engine.EpicWriteRefusedError
	var notFound *engine.EpicNotFoundError
	var removeRefused *engine.EpicRemoveRefusedError
	var mce *engine.MalformedConfigError
	var lockErr *engine.LockTimeoutError
	var stale *engine.EpicStaleRevisionError
	var edgeNotFound *engine.EpicEdgeNotFoundError
	switch {
	case errors.As(err, &lockErr):
		output.Error(ExitState, lockErr.Error(), lockErr.Hint())
		os.Exit(ExitState)
	case errors.As(err, &stale):
		output.Error(ExitState, stale.Error(), epicStaleRevisionHint)
		os.Exit(ExitState)
	case errors.As(err, &removeRefused):
		output.Error(ExitValidation, removeRefused.Error(), epicRemoveRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &notFound):
		output.Error(ExitState, notFound.Error(), epicNotFoundHint)
		os.Exit(ExitState)
	case errors.As(err, &edgeNotFound):
		output.Error(ExitState, edgeNotFound.Error(), epicEdgeNotFoundHint)
		os.Exit(ExitState)
	case errors.As(err, &refused) && bulkLines != nil:
		output.Error(ExitValidation, epicBulkRefusedMessage(refused, bulkLines), epicBulkRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &refused):
		output.Error(ExitValidation, refused.Error(), epicWriteRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &mce):
		output.Error(ExitFile, mce.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
	}
	output.Error(ExitFile, err.Error(), epicGraphWriteHint)
	os.Exit(ExitFile)
}

// epicStaleRevisionHint answers a write whose --if-revision is stale.
const epicStaleRevisionHint = "re-read .tp/epics.json, redo the edit on what it holds now, and pass the revision of those bytes (or the one the last writer printed)"

// epicBulkRefusedHint answers a refused --bulk write: the fault is in the
// payload rows the message names, not in the file.
const epicBulkRefusedHint = "fix the --bulk payload rows each error names; a row may depend on any other row of the same payload"

// epicBulkRefusedMessage is the refusal's message with, after each error, the
// payload lines of the epics it names.
func epicBulkRefusedMessage(refused *engine.EpicWriteRefusedError, bulkLines map[string][]int) string {
	parts := make([]string, 0, len(refused.New))
	for i := range refused.New {
		f := &refused.New[i]
		part := fmt.Sprintf("%s [%s] (%s)", f.Rule, strings.Join(f.Epics, ", "), f.Message)
		var nums []int
		for _, id := range f.Epics {
			nums = append(nums, bulkLines[id]...)
		}
		if len(nums) > 0 {
			sort.Ints(nums)
			label := "--bulk line "
			if len(nums) > 1 {
				label = "--bulk lines "
			}
			strs := make([]string, len(nums))
			for j, n := range nums {
				strs[j] = strconv.Itoa(n)
			}
			part += " (" + label + strings.Join(strs, ", ") + ")"
		}
		parts = append(parts, part)
	}
	return "write refused: it adds lint errors the file did not hold: " + strings.Join(parts, "; ")
}
