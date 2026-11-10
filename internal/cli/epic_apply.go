package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicApplySet is the action a set row reports.
const epicApplySet = "set"

// epicApplyUsageHint answers a malformed payload row.
const epicApplyUsageHint = `each line is one row: {"op":"add","epic":{...}}, {"op":"link","from":...,"to":...|"any_of":[...],"why":...}, {"op":"unlink","from":...,"to":...|"any_of":[...]}, {"op":"set","id":...,"field":...,"value":...}, {"op":"set","field":...,"value":...} or {"op":"remove","id":...,"force":bool}; records (done, claim, gate --met) and import are not rows`

// epicApplyFileHint answers an unreadable payload path.
const epicApplyFileHint = "check the tp epic apply path — it takes an NDJSON file, one row JSON object per line"

// epicApplyLineTooLongHint answers a payload line over ndjsonLineCap.
const epicApplyLineTooLongHint = "a tp epic apply line exceeded the 1MB NDJSON read cap — write one row per line and shorten the oversized one"

// epicApplyRefusedHint answers an apply whose after-state the lint refuses.
const epicApplyRefusedHint = "fix the payload rows each error names; only the state after the last row is linted, so a row may name an epic a later row adds"

// epicApplyRow is one payload row as written; Op says which fields it reads.
type epicApplyRow struct {
	Op    string          `json:"op"`
	Epic  json.RawMessage `json:"epic"`
	From  string          `json:"from"`
	To    *string         `json:"to"`
	AnyOf []string        `json:"any_of"`
	Why   string          `json:"why"`
	ID    *string         `json:"id"`
	Field string          `json:"field"`
	Value json.RawMessage `json:"value"`
	Force bool            `json:"force"`
}

// epicApplyRowAction is what one row did, as --dry-run reports it.
type epicApplyRowAction struct {
	Line   int    `json:"line"`
	Op     string `json:"op"`
	Action string `json:"action"`
}

// epicApplyStep is a parsed row: its mutation, the action the mutation
// reports, and the epic ids it names, which map a refusal back to its line.
type epicApplyStep struct {
	line   int
	op     string
	mutate func(*model.EpicGraph) error
	action *string
	ids    []string
}

// epicApplyRowError is a failure at one row: an absent subject or an absent
// unlinked edge (spec/1.7.0.md §6). It names the row's payload line.
type epicApplyRowError struct {
	Line int
	Err  error
}

func (e *epicApplyRowError) Error() string {
	return fmt.Sprintf("payload line %d: %v", e.Line, e.Err)
}

func (e *epicApplyRowError) Unwrap() error { return e.Err }

func newEpicApplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apply <ndjson>",
		Short: "Apply add, link, unlink, set and remove rows in order as one write, linting only the final state; any error writes nothing",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicApply,
	}
	cmd.Flags().Bool("dry-run", false, "print the payload and each row's action without writing")
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicApply implements tp epic apply (spec/1.7.0.md §6): the rows run in
// file order on one graph under one lock, and the after-state is linted once
// against the before-state.
func runEpicApply(cmd *cobra.Command, args []string) error {
	fenceEpicWrite("tp epic apply")
	steps := readEpicApplySteps(args[0])
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	rows := make([]epicApplyRowAction, 0, len(steps))
	opts := epicWriteOpts{ifRevision: epicIfRevision(cmd), apply: true, dryRun: dryRun, rows: &rows, bulkLines: map[string][]int{}}
	for i := range steps {
		for _, id := range steps[i].ids {
			opts.bulkLines[id] = append(opts.bulkLines[id], steps[i].line)
		}
	}
	mutate := withAncestryReport(func(g *model.EpicGraph) error {
		for i := range steps {
			s := &steps[i]
			if err := s.mutate(g); err != nil {
				return &epicApplyRowError{Line: s.line, Err: err}
			}
			rows = append(rows, epicApplyRowAction{Line: s.line, Op: s.op, Action: *s.action})
		}
		return nil
	}, &opts)
	writeEpicGraphWith(opts, mutate)
	return nil
}

// readEpicApplySteps parses every row of the payload at path; a malformed
// row, or an add of an id an earlier row removes, exits 2 naming its line
// before the graph is read.
func readEpicApplySteps(path string) []epicApplyStep {
	var steps []epicApplyStep
	removed := map[string]int{}
	scanNDJSONFile(path, epicApplyFileHint, epicApplyLineTooLongHint, func(lineNum int, line []byte) {
		s, err := parseEpicApplyRow(line)
		if err == nil {
			err = refuseEpicReAdd(&s, lineNum, removed)
		}
		if err != nil {
			output.Error(ExitUsage, fmt.Sprintf("payload line %d: %v", lineNum, err), epicApplyUsageHint)
			os.Exit(ExitUsage)
		}
		s.line = lineNum
		steps = append(steps, s)
	})
	return steps
}

// refuseEpicReAdd records a remove row's id in removed and refuses an add of
// an id an earlier row removed: add already replaces by id (§6).
func refuseEpicReAdd(s *epicApplyStep, lineNum int, removed map[string]int) error {
	switch s.op {
	case "remove":
		removed[s.ids[0]] = lineNum
	case "add":
		if at, ok := removed[s.ids[0]]; ok {
			return fmt.Errorf("adds epic %q, which line %d removes: an add already replaces an epic by id, so drop the remove", s.ids[0], at)
		}
	}
	return nil
}

// parseEpicApplyRow decodes one row, refusing keys outside the row schema
// and anything after the one JSON object, and builds its step.
func parseEpicApplyRow(line []byte) (epicApplyStep, error) {
	var r epicApplyRow
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return epicApplyStep{}, fmt.Errorf("invalid row JSON: %w", err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return epicApplyStep{}, errors.New("trailing data after the row: write exactly one JSON object per line")
	}
	switch r.Op {
	case "add":
		return epicApplyAddStep(&r)
	case "link", "unlink":
		return epicApplyLinkStep(&r)
	case "set":
		return epicApplySetStep(&r)
	case "remove":
		if r.ID == nil {
			return epicApplyStep{}, errors.New(`a remove row takes "id"`)
		}
		action := engine.EpicEdgeRemoved
		return epicApplyStep{op: r.Op, mutate: engine.RemoveEpicRow(*r.ID, r.Force), action: &action, ids: []string{*r.ID}}, nil
	}
	return epicApplyStep{}, fmt.Errorf("unknown op %q: one of add, link, unlink, set, remove", r.Op)
}

// epicApplyAddStep builds an add row: the epic replaces the one with its id,
// as a --bulk row does, and a claimed or done record is refused.
func epicApplyAddStep(r *epicApplyRow) (epicApplyStep, error) {
	if len(r.Epic) == 0 {
		return epicApplyStep{}, errors.New(`an add row takes "epic"`)
	}
	e, err := decodeEpic(r.Epic)
	if err != nil {
		return epicApplyStep{}, err
	}
	action := engine.EpicEdgeAdded
	mutate := func(g *model.EpicGraph) error {
		if slices.ContainsFunc(g.Epics, func(x model.Epic) bool { return x.ID == e.ID }) {
			action = engine.EpicEdgeReplaced
		}
		engine.AddEpics(g, []model.Epic{e}, true)
		return nil
	}
	return epicApplyStep{op: r.Op, mutate: mutate, action: &action, ids: []string{e.ID}}, nil
}

// epicApplyLinkStep builds a link or unlink row of a plain edge (to) or an
// any-of group (any_of), exactly one of which it names.
func epicApplyLinkStep(r *epicApplyRow) (epicApplyStep, error) {
	if r.From == "" {
		return epicApplyStep{}, fmt.Errorf(`a %s row takes "from"`, r.Op)
	}
	if (r.To == nil) == (r.AnyOf == nil) {
		return epicApplyStep{}, fmt.Errorf(`a %s row takes exactly one of "to" and "any_of"`, r.Op)
	}
	var action string
	s := epicApplyStep{op: r.Op, action: &action, ids: append([]string{r.From}, r.AnyOf...)}
	switch {
	case r.To != nil && r.Op == "link":
		s.mutate = engine.LinkEpic(r.From, *r.To, r.Why, &action)
	case r.To != nil:
		s.mutate = engine.UnlinkEpic(r.From, *r.To, &action)
	case r.Op == "link":
		s.mutate = engine.LinkEpicAnyOf(r.From, r.AnyOf, r.Why, &action)
	default:
		s.mutate = engine.UnlinkEpicAnyOf(r.From, r.AnyOf, &action)
	}
	if r.To != nil {
		s.ids = append(s.ids, *r.To)
	}
	return s, nil
}

// epicApplySetStep builds a set row: with id an epic field, as tp epic set
// <id> writes it, and without one a graph key. A JSON string value is taken
// as its text and any other JSON value as written.
func epicApplySetStep(r *epicApplyRow) (epicApplyStep, error) {
	if r.Field == "" || len(r.Value) == 0 {
		return epicApplyStep{}, errors.New(`a set row takes "field" and "value"`)
	}
	value := string(bytes.TrimSpace(r.Value))
	if value != "" && value[0] == '"' {
		if err := json.Unmarshal(r.Value, &value); err != nil {
			return epicApplyStep{}, err
		}
	}
	action := epicApplySet
	s := epicApplyStep{op: r.Op, action: &action}
	var err error
	if r.ID != nil {
		s.mutate, err = engine.ParseEpicFieldSet(*r.ID, r.Field, value)
		s.ids = []string{*r.ID}
	} else {
		s.mutate, err = engine.ParseEpicGraphSet(r.Field, value)
	}
	return s, err
}
