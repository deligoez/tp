package cli

import (
	"math"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

func newReportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "Per-task duration and estimation accuracy",
		Long: `Shows how long each completed task took and compares against estimates.
Only includes tasks with both started_at and closed_at timestamps.
Output: {tasks: [{id, estimate_minutes, actual_minutes, accuracy}], summary}`,
		RunE: runReport,
	}
}

type reportTask struct {
	ID                string   `json:"id"`
	EstimateMinutes   int      `json:"estimate_minutes"`
	ActualMinutes     float64  `json:"actual_minutes"`
	Accuracy          *float64 `json:"accuracy"`
	DurationSource    string   `json:"duration_source,omitempty"`
	Note              *string  `json:"note,omitempty"`
	GateSkippedReason *string  `json:"gate_skipped_reason,omitempty"`
	OutOfScope        *string  `json:"out_of_scope,omitempty"`
}

type reportSummary struct {
	TotalTasks            int      `json:"total_tasks"`
	Completed             int      `json:"completed"`
	Tracked               int      `json:"tracked"`
	Untracked             int      `json:"untracked"`
	TotalEstimatedMinutes int      `json:"total_estimated_minutes"`
	TotalActualMinutes    float64  `json:"total_actual_minutes"`
	EstimationAccuracy    *float64 `json:"estimation_accuracy"`
	ExcludedFromAccuracy  int      `json:"excluded_from_accuracy"`
	ImplicitDuration      int      `json:"implicit_duration"`
	AverageTaskMinutes    float64  `json:"average_task_minutes"`
	FastestTask           *idDur   `json:"fastest_task"`
	SlowestTask           *idDur   `json:"slowest_task"`
}

type idDur struct {
	ID      string  `json:"id"`
	Minutes float64 `json:"minutes"`
}

func runReport(_ *cobra.Command, _ []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	tasks, summary := computeReport(tf)
	if flagCompact {
		for i := range tasks {
			tasks[i].Note = nil
		}
	}
	return output.JSON(map[string]any{
		"tasks":   tasks,
		"summary": summary,
	})
}

// computeReport builds the full report from a task file.
func computeReport(tf *model.TaskFile) ([]reportTask, reportSummary) {
	var (
		tasks          []reportTask
		completedCount int
		untrackedCount int
		totalEstimated int
		totalActual    float64
		fastest        *idDur
		slowest        *idDur

		// Accuracy accumulators: null-accuracy tasks are excluded from
		// the summary's estimation_accuracy (§14.2).
		accEstimated int
		accActual    float64
		excluded     int

		// §11.2: implicit-duration tasks are excluded from estimation_accuracy
		// under a separate implicit_duration count, disjoint from and taking
		// precedence over excluded_from_accuracy (rounds-to-zero).
		implicitDuration int
	)

	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusDone {
			continue
		}
		completedCount++

		if t.StartedAt == nil || t.ClosedAt == nil {
			untrackedCount++
			continue
		}

		duration := t.ClosedAt.Sub(*t.StartedAt)
		actualMin := duration.Minutes()

		if actualMin < 0 {
			untrackedCount++
			continue
		}

		roundedActual := roundTo(actualMin, 1)

		rt := reportTask{
			ID:                t.ID,
			EstimateMinutes:   t.EstimateMinutes,
			ActualMinutes:     roundedActual,
			DurationSource:    t.DurationSource,
			GateSkippedReason: t.GateSkippedReason,
		}

		// §7.2: surface an out-of-fence finding the closing unit recorded as a
		// trailing "Out of scope:" line so it reaches a human. Visible even
		// under --compact: the line exists precisely to not die in a context
		// window.
		if t.ClosedReason != nil {
			if note := engine.ExtractOutOfScope(*t.ClosedReason); note != "" {
				n := note
				rt.OutOfScope = &n
			}
		}

		// §11.2: an implicit-duration task (started_at came from tp done's or
		// tp commit's implicit claim) is excluded from estimation_accuracy under
		// a separate implicit_duration count. The two buckets are disjoint —
		// implicit takes precedence over excluded_from_accuracy (rounds-to-zero).
		switch {
		case t.DurationSource == model.DurationSourceImplicit:
			note := "implicit claim (duration not measured)"
			rt.Note = &note
			implicitDuration++
		case roundedActual == 0:
			// §14.1: when actual_minutes rounds to 0.0 the accuracy is null
			// and the task carries an explanatory note.
			note := "duration below resolution"
			rt.Note = &note
			excluded++
		default:
			acc := roundTo(float64(t.EstimateMinutes)/actualMin, 2)
			rt.Accuracy = &acc
			accEstimated += t.EstimateMinutes
			accActual += actualMin
		}
		tasks = append(tasks, rt)

		totalEstimated += t.EstimateMinutes
		totalActual += actualMin

		if fastest == nil || actualMin < fastest.Minutes {
			fastest = &idDur{ID: t.ID, Minutes: roundedActual}
		}
		if slowest == nil || actualMin > slowest.Minutes {
			slowest = &idDur{ID: t.ID, Minutes: roundedActual}
		}
	}

	if tasks == nil {
		tasks = make([]reportTask, 0)
	}

	trackedCount := len(tasks)
	avgMin := 0.0
	if trackedCount > 0 {
		avgMin = roundTo(totalActual/float64(trackedCount), 1)
	}

	summary := reportSummary{
		TotalTasks:            len(tf.Tasks),
		Completed:             completedCount,
		Tracked:               trackedCount,
		Untracked:             untrackedCount,
		TotalEstimatedMinutes: totalEstimated,
		TotalActualMinutes:    roundTo(totalActual, 1),
		EstimationAccuracy:    nil,
		ExcludedFromAccuracy:  excluded,
		ImplicitDuration:      implicitDuration,
		AverageTaskMinutes:    avgMin,
		FastestTask:           fastest,
		SlowestTask:           slowest,
	}
	if accActual > 0 {
		estAccuracy := roundTo(float64(accEstimated)/accActual, 2)
		summary.EstimationAccuracy = &estAccuracy
	}

	return tasks, summary
}

func roundTo(val float64, places int) float64 {
	pow := math.Pow(10, float64(places))
	return math.Round(val*pow) / pow
}
