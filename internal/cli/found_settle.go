package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// foundSettleHint names the batch's shape, so a refused call can be rebuilt
// without reading the help.
const foundSettleHint = "settle open queue entries: --task <entry> makes a task (an entry without expected and observed behaviour also needs --expected <entry>=<text> and --observed <entry>=<text>), --reject <entry>=<reason> rejects one; each repeatable, every entry once"

// foundSettleFlags are tp found settle's inputs, each repeatable.
type foundSettleFlags struct {
	tasks, rejects, expected, observed []string
}

// errFoundSettleRefused marks a plan error that is the caller's to fix: tp
// found settle exits 2 on it and writes nothing.
var errFoundSettleRefused = errors.New("refused")

func newFoundSettleCmd() *cobra.Command {
	var f foundSettleFlags
	cmd := &cobra.Command{
		Use:   "settle",
		Short: "Settle open queue entries: make a task of each, or reject it with a reason",
		Long: `Settle a batch of the spec's open queue entries, the person's step.

--task makes a task in the spec's task file, shaped like a found task; it
does not count against found_max, and each finder waiting on the entry now
depends on it. --reject records the reason, which may say where the defect
went; its finders are ready again. A settled entry stays in the queue.

An entry that is unknown, not open or named twice, a rejection without a
reason, or a task without expected and observed behaviour refuses the whole
batch: exit 2, nothing written.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFoundSettle(&f)
		},
	}
	// StringArray, not StringSlice: a reason is prose.
	cmd.Flags().StringArrayVar(&f.tasks, "task", nil, "an open entry to make a task of (repeatable)")
	cmd.Flags().StringArrayVar(&f.rejects, "reject", nil, "<entry>=<reason>: an open entry to reject (repeatable)")
	cmd.Flags().StringArrayVar(&f.expected, "expected", nil, "<entry>=<text>: a --task entry's expected behaviour, where it has none")
	cmd.Flags().StringArrayVar(&f.observed, "observed", nil, "<entry>=<text>: a --task entry's observed behaviour, where it has none")
	return cmd
}

// runFoundSettle implements tp found settle (spec/1.31.0.md §6).
func runFoundSettle(f *foundSettleFlags) error {
	fenceFoundSettle()
	rows, msg := foundSettleRows(f)
	if msg != "" {
		return foundRefusal(msg, foundSettleHint)
	}
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file resolved, so there is no queue to settle.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	plan, err := settleFoundQueue(taskFilePath, rows)
	if errors.Is(err, errFoundSettleRefused) {
		return foundRefusal(strings.TrimSuffix(err.Error(), ": "+errFoundSettleRefused.Error()), foundSettleHint)
	}
	if err != nil {
		output.Error(ExitFile, "cannot settle the entries: "+err.Error(), foundStoreWriteHint)
		os.Exit(ExitFile)
		return nil
	}
	out := map[string]any{}
	if len(plan.Made) > 0 {
		out["made"] = plan.Made
	}
	if len(plan.Rejected) > 0 {
		out["rejected"] = plan.Rejected
	}
	return output.JSON(out)
}

// foundSettleRows turns the flags into the batch's rows, --task ones first,
// or says why the flags are refused.
func foundSettleRows(f *foundSettleFlags) (rows []engine.FoundSettleRow, refusal string) {
	rows = make([]engine.FoundSettleRow, 0, len(f.tasks)+len(f.rejects))
	for _, id := range f.tasks {
		rows = append(rows, engine.FoundSettleRow{Entry: strings.TrimSpace(id)})
	}
	for _, r := range f.rejects {
		id, reason, ok := strings.Cut(r, "=")
		if !ok || strings.TrimSpace(reason) == "" {
			return nil, fmt.Sprintf("--reject %s needs a reason: --reject <entry>=<reason>", r)
		}
		rows = append(rows, engine.FoundSettleRow{Entry: strings.TrimSpace(id), Reason: reason})
	}
	for _, field := range []struct {
		name   string
		values []string
		set    func(*engine.FoundSettleRow, string)
	}{
		{"--expected", f.expected, func(r *engine.FoundSettleRow, v string) { r.Expected = v }},
		{"--observed", f.observed, func(r *engine.FoundSettleRow, v string) { r.Observed = v }},
	} {
		for _, v := range field.values {
			id, text, ok := strings.Cut(v, "=")
			i := -1
			for j := range rows {
				if rows[j].Entry == strings.TrimSpace(id) {
					i = j
				}
			}
			if !ok || i < 0 {
				return nil, fmt.Sprintf("%s %s names no entry of this call: %s <entry>=<text>", field.name, v, field.name)
			}
			field.set(&rows[i], text)
		}
	}
	return rows, ""
}

// settleFoundQueue settles the rows under the task-file lock, writing the
// task file and the queue. A plan error is a refusal and writes nothing.
func settleFoundQueue(taskFilePath string, rows []engine.FoundSettleRow) (engine.FoundSettlePlan, error) {
	var plan engine.FoundSettlePlan
	err := engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			return err
		}
		queue, err := engine.ReadFoundQueue(taskFilePath)
		if err != nil {
			return err
		}
		plan, err = engine.PlanFoundSettle(tf.Tasks, queue, rows)
		if err != nil {
			return fmt.Errorf("%w: %w", err, errFoundSettleRefused)
		}
		if len(plan.Tasks) > 0 {
			addFoundEdges(tf, plan.Edges)
			tf.Tasks = append(tf.Tasks, plan.Tasks...)
			if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
				return err
			}
		}
		return engine.WriteFoundQueue(taskFilePath, plan.Queue)
	})
	return plan, err
}
