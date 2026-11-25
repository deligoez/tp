package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// foundDecideHint says what tp found decide takes.
const foundDecideHint = "name the undecided requests to decide, comma-separated as a decision unit's TP_UNIT_ID carries them (F3,F4)"

// decisionIdentity is the decision brief's opening: what the unit is and
// what it must never do (spec/1.31.0.md §5).
const decisionIdentity = "This agent is a decision unit: it answers each request below once, with tp found answer, and stops." +
	" It never makes a rule-9 decision (an operator decision such as raising a cap, accepting a finding or" +
	" settling a queue entry): such a request is answered human. When in doubt, it answers human."

// decisionAnswers words the three answers (spec/1.31.0.md §5).
var decisionAnswers = map[string]string{
	engine.FoundAnswerTask: "tp found answer <request> task --expected <text> --observed <text> --cite <file> --quote <text>:" +
		" only after you ran the reproduction and saw it fail, and only when a sentence quoted verbatim from the spec," +
		" a CLAUDE.md, CONVENTIONS.md or REFERENCE.md settles the choice",
	engine.FoundAnswerConforms: "tp found answer <request> conforms --reproduction <command> --cite <file> --quote <text>:" +
		" the expected behaviour already holds (the reproduction passes on the current code), settled by a verbatim quote",
	engine.FoundAnswerHuman: "tp found answer <request> human: everything else, including any answer that rests on an inference," +
		" a combination of sections or the reading of an ambiguous word; when in doubt, this one",
}

func newFoundDecideCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "decide <requests>",
		Short: "Brief a decision unit on the undecided requests it answers",
		Long: `Print the brief of a decision unit (tp run spawns one when undecided requests
exist and no implement unit is ready or running): the named requests, at
most 5, and the three answers tp found answer records. A request already
answered is listed as answered; an unknown one exits 2.`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return runFoundDecide(engine.DecisionRequestIDs(strings.Join(args, ",")))
		},
	}
}

// runFoundDecide implements tp found decide (spec/1.31.0.md §5).
func runFoundDecide(ids []string) error {
	if len(ids) == 0 || len(ids) > engine.DecisionBatchMax {
		return foundRefusal(fmt.Sprintf("a decision unit takes 1 to %d requests, got %d", engine.DecisionBatchMax, len(ids)), foundDecideHint)
	}
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file resolved, so there is no request to decide.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	reqs, err := engine.ReadFoundRequests(engine.CanonicalProjectRoot(filepath.Dir(taskFilePath)), taskFilePath)
	if err != nil {
		output.Error(ExitFile, "cannot read the found requests: "+err.Error(), foundStoreWriteHint)
		os.Exit(ExitFile)
		return nil
	}
	undecided := make([]engine.FoundRequest, 0, len(ids))
	answered := []string{}
	for _, id := range ids {
		i := slices.IndexFunc(reqs, func(r engine.FoundRequest) bool { return r.ID == id })
		switch {
		case i < 0:
			return foundRefusal(fmt.Sprintf("no found request %s", id), foundDecideHint)
		case reqs[i].State == engine.FoundStateUndecided:
			undecided = append(undecided, reqs[i])
		default:
			answered = append(answered, id)
		}
	}
	return output.JSON(map[string]any{
		"identity": decisionIdentity,
		"requests": undecided,
		"answered": answered,
		"answers":  decisionAnswers,
	})
}
