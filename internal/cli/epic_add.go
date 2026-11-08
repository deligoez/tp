package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicWriteRefusedHint answers a write that would add a lint error.
const epicWriteRefusedHint = "fix the payload so it adds no lint error; tp epic lint shows the errors the file already holds"

// epicWriteResult is every epic writer's payload: the file it wrote and the
// epic ids the write touched.
type epicWriteResult struct {
	File  string   `json:"file"`
	Epics []string `json:"epics"`
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
	return cmd
}

// runEpicAdd implements tp epic add (spec/1.3.0.md §3).
func runEpicAdd(cmd *cobra.Command, args []string) error {
	bulk, _ := cmd.Flags().GetString("bulk")
	var epics []model.Epic
	if bulk != "" {
		epics = readBulkEpics(bulk)
	} else {
		epics = []model.Epic{decodeEpicArg(cmd, args)}
	}
	ids := make([]string, 0, len(epics))
	for i := range epics {
		ids = append(ids, epics[i].ID)
	}
	writeEpicGraph(ids, func(g *model.EpicGraph) error {
		engine.AddEpics(g, epics, bulk != "")
		return nil
	})
	return nil
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

// decodeEpic parses one epic, refusing keys outside the schema.
func decodeEpic(raw []byte) (model.Epic, error) {
	var e model.Epic
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	err := dec.Decode(&e)
	return e, err
}

// readBulkEpics reads one epic per non-blank NDJSON line; any bad line aborts
// the whole add, naming it.
func readBulkEpics(path string) []model.Epic {
	data, err := os.ReadFile(path)
	if err != nil {
		output.Error(ExitFile, err.Error(), ndjsonInputFileHint)
		os.Exit(ExitFile)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), ndjsonLineCap)
	var epics []model.Epic
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
	}
	if err := scanner.Err(); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read %s: %v", path, err), ndjsonReadHint(err))
		os.Exit(ExitFile)
	}
	return epics
}

// writeEpicGraph runs one locked, lint-checked write of .tp/epics.json and
// reports it: exit 1 naming the new errors, 4 naming an absent epic, 3 on a
// file error.
func writeEpicGraph(ids []string, mutate func(*model.EpicGraph) error) {
	path, err := engine.UpdateEpicGraph(engine.ProjectConfigDir("."), mutate)
	var refused *engine.EpicWriteRefusedError
	var notFound *engine.EpicNotFoundError
	var removeRefused *engine.EpicRemoveRefusedError
	var mce *engine.MalformedConfigError
	switch {
	case errors.As(err, &removeRefused):
		output.Error(ExitValidation, removeRefused.Error(), epicRemoveRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &notFound):
		output.Error(ExitState, notFound.Error(), epicNotFoundHint)
		os.Exit(ExitState)
	case errors.As(err, &refused):
		output.Error(ExitValidation, refused.Error(), epicWriteRefusedHint)
		os.Exit(ExitValidation)
	case errors.As(err, &mce):
		output.Error(ExitFile, mce.Error(), epicGraphFileHint)
		os.Exit(ExitFile)
	case err != nil:
		output.Error(ExitFile, err.Error(), stateWriteHint)
		os.Exit(ExitFile)
	}
	if err := output.JSON(epicWriteResult{File: path, Epics: ids}); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
}
