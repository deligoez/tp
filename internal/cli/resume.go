package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume [spec]",
		Short: "Report the lifecycle phase and the single next action (read-only)",
		Long: `The phase-agnostic resume oracle. From durable state alone (the task file,
the spec, .tp-review/, .tp/local.json, and git) it reports which lifecycle phase
the project is in and the concrete next action — the note a finishing agent
leaves for the next one. Read-only: it writes no file.

Output: {phase, spec, changes, kept, next_action, blockers}`,
		Args: cobra.MaximumNArgs(1),
		RunE: runResume,
	}
}

func runResume(_ *cobra.Command, args []string) error {
	var taskFilePath, specPath string
	var tf *model.TaskFile

	if len(args) == 1 {
		// A spec argument is authoritative: use its adjacent <base>.tasks.json,
		// treating an absent adjacent file as an empty task set (§4.1).
		specPath = args[0]
		taskFilePath = engine.SpecAdjacentTaskFile(specPath)
		if loaded, err := model.ReadTaskFile(taskFilePath); err == nil {
			tf = loaded
		} else {
			tf = &model.TaskFile{Spec: specPath, Tasks: []model.Task{}}
		}
	} else {
		// No spec argument: discover the active task file and derive the spec.
		discovered, err := engine.DiscoverTaskFile(".", flagFile)
		if err != nil {
			output.Error(ExitFile, "no task file found", "run tp init <spec> or pass a spec path")
			os.Exit(ExitFile)
			return nil
		}
		taskFilePath = discovered
		loaded, readErr := model.ReadTaskFile(discovered)
		if readErr != nil {
			output.Error(ExitFile, readErr.Error())
			os.Exit(ExitFile)
			return nil
		}
		tf = loaded
		specPath, _ = engine.ResolveSpecPath(discovered, loaded.Spec)
	}

	result, err := engine.AssembleResume(".", taskFilePath, specPath, tf)
	if err != nil {
		var sce *engine.StateCorruptError
		if errors.As(err, &sce) {
			output.Error(ExitFile, sce.Error(), sce.Hint())
		} else {
			output.Error(ExitFile, err.Error())
		}
		os.Exit(ExitFile)
		return nil
	}

	if output.IsJSON() {
		return output.JSON(resumeJSON(&result, flagCompact))
	}
	printResumeSummary(&result)
	return nil
}

// resumeJSON renders the resume result as JSON, stripping the human-facing
// fields under --compact: next_action.summary, each kept[].reason, and each
// blockers[].message, while keeping every machine-actionable field including
// blockers[].data (§4.2).
func resumeJSON(r *engine.ResumeResult, compact bool) map[string]any {
	nextAction := map[string]any{
		"command":       r.NextAction.Command,
		"brief_command": r.NextAction.BriefCommand,
		"payload":       r.NextAction.Payload,
	}
	if !compact {
		nextAction["summary"] = r.NextAction.Summary
	}

	kept := make([]map[string]any, 0, len(r.Kept))
	for _, k := range r.Kept {
		entry := map[string]any{"path": k.Path}
		if !compact {
			entry["reason"] = k.Reason
		}
		kept = append(kept, entry)
	}

	blockers := make([]map[string]any, 0, len(r.Blockers))
	for _, b := range r.Blockers {
		entry := map[string]any{"code": b.Code, "class": b.Class, "data": b.Data}
		if !compact {
			entry["message"] = b.Message
		}
		blockers = append(blockers, entry)
	}

	// §8.4: bookkeeping and the guidance note survive --compact — both are
	// decision-critical (the agent must commit bookkeeping, and the guidance
	// shapes how it drives the implement phase).
	bookkeeping := make([]map[string]any, 0, len(r.Bookkeeping))
	for _, b := range r.Bookkeeping {
		bookkeeping = append(bookkeeping, map[string]any{
			"path": b.Path,
			"kind": b.Kind,
			"ref":  b.Ref,
		})
	}

	out := map[string]any{
		"phase":       r.Phase,
		"spec":        r.Spec,
		"changes":     r.Changes,
		"kept":        kept,
		"bookkeeping": bookkeeping,
		"next_action": nextAction,
		"blockers":    blockers,
	}
	if r.Guidance != "" {
		out["guidance"] = r.Guidance
	}
	return out
}

// printResumeSummary writes the short human summary for a TTY (§4.1).
func printResumeSummary(r *engine.ResumeResult) {
	next := "—"
	if r.NextAction.Command != nil {
		next = *r.NextAction.Command
	}
	fmt.Printf("phase: %s\n", r.Phase)
	fmt.Printf("next:  %s — %s\n", next, r.NextAction.Summary)
	if r.NextAction.BriefCommand != nil {
		fmt.Printf("brief: %s\n", *r.NextAction.BriefCommand)
	}
	if r.Guidance != "" {
		fmt.Printf("guidance: %s\n", r.Guidance)
	}
	if len(r.Bookkeeping) > 0 {
		fmt.Printf("bookkeeping: %d tp-owned file(s) to commit\n", len(r.Bookkeeping))
	}
	if len(r.Changes) > 0 {
		fmt.Printf("unexplained changes: %d\n", len(r.Changes))
	}
	for _, b := range r.Blockers {
		fmt.Printf("blocker [%s] %s: %s\n", b.Class, b.Code, b.Message)
	}
}
