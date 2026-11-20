package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	claimSpawn    string
	claimSession  string
	claimTakeover bool
)

// refuseBadSpawnFlags is the usage check of tp claim's spawn flags, made
// before any file is read: --session and --takeover need --spawn, --spawn
// takes one id, and each value has the claim-token shape, checked here at the
// sink where it is written into the task file.
func refuseBadSpawnFlags(cmd *cobra.Command, args []string) {
	sessionSet := cmd.Flags().Changed("session")
	usage := func(msg string) {
		output.Error(ExitUsage, msg)
		os.Exit(ExitUsage)
	}
	if claimSpawn == "" {
		if sessionSet || claimTakeover {
			usage("--session and --takeover need --spawn")
		}
		return
	}
	if claimAllReady || len(args) != 1 {
		usage("--spawn claims exactly one task id")
	}
	if !engine.ValidClaimToken(claimSpawn) {
		usage(fmt.Sprintf("--spawn %q: want 1-128 of A-Z a-z 0-9 _ -", claimSpawn))
	}
	// An empty --session is refused like any bad value: no session is
	// spelled by leaving the flag out.
	if sessionSet && !engine.ValidClaimToken(claimSession) {
		usage(fmt.Sprintf("--session %q: want 1-128 of A-Z a-z 0-9 _ -", claimSession))
	}
}

// claimForSpawn is tp claim <id> --spawn: an open, ready task is claimed now;
// a wip one keeps its started_at and is bound to this spawn, unless another
// spawn holds it, which only --takeover overrides. A done task, a blocked one
// or an unknown id exits 4 as a plain claim does.
func claimForSpawn(tf *model.TaskFile, taskFilePath, id string) error {
	task, _, err := model.FindTask(tf, id)
	if err != nil {
		output.Error(ExitState, err.Error())
		os.Exit(ExitState)
		return nil
	}
	if task.Status != model.StatusWIP {
		claimSingleTask(tf, task)
	} else if held := task.ClaimedBy; held != nil && held.Spawn != "" && held.Spawn != claimSpawn && !claimTakeover {
		output.Error(ExitState,
			fmt.Sprintf("cannot claim: task %s is held by spawn %s", task.ID, held.Spawn),
			fmt.Sprintf("If that agent is gone, run `tp claim %s --spawn %s --takeover`, or `tp unclaim %s`.", task.ID, claimSpawn, task.ID))
		os.Exit(ExitState)
		return nil
	}
	task.ClaimedBy = &model.ClaimedBy{Session: claimSession, Spawn: claimSpawn}
	tf.UpdatedAt = time.Now().UTC()
	if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
		// task-file-hint: the write that failed is the task file's.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	output.Success(fmt.Sprintf("claimed %s for spawn %s", task.ID, claimSpawn))
	printResultOrExit(taskWritten{Task: task, File: taskFileLabel(taskFilePath)})
	return nil
}
