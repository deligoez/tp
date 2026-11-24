package cli

import (
	"fmt"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	claimSpawn    string
	claimSession  string
	claimToken    string
	claimTakeover bool
)

// refuseBadSpawnFlags is the usage check of tp claim's spawn flags, made
// before any file is read: --session needs --spawn, --takeover stands alone,
// each takes one id, and each value has the claim-token shape, checked here
// at the sink where it is written into the task file.
func refuseBadSpawnFlags(cmd *cobra.Command, args []string) {
	sessionSet := cmd.Flags().Changed("session")
	tokenSet := cmd.Flags().Changed("claim-token")
	usage := func(msg string) {
		output.Error(ExitUsage, msg)
		os.Exit(ExitUsage)
	}
	if claimTakeover {
		if claimSpawn != "" || sessionSet || tokenSet || claimAllReady || len(args) != 1 {
			usage("--takeover takes exactly one task id and no other claim flag")
		}
		return
	}
	if claimSpawn == "" {
		if sessionSet || tokenSet {
			usage("--session and --claim-token need --spawn")
		}
		return
	}
	if tokenSet && !engine.ValidClaimToken(claimToken) {
		usage(fmt.Sprintf("--claim-token %q: want 1-128 of A-Z a-z 0-9 _ -", claimToken))
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
// spawn holds it. A done task, a blocked one or an unknown id exits 4 as a
// plain claim does.
func claimForSpawn(tf *model.TaskFile, taskFilePath, id string) error {
	task := findOrExit(tf, id)
	if task.Status != model.StatusWIP {
		claimSingleTask(tf, task)
	} else if held := task.ClaimedBy; held != nil && held.Spawn != "" && held.Spawn != claimSpawn {
		output.Error(ExitState,
			fmt.Sprintf("cannot claim: task %s is held by spawn %s", task.ID, held.Spawn),
			fmt.Sprintf("If that agent is gone, free the claim with `tp claim %s --takeover` (keeps started_at) and spawn again from a fresh `tp brief %s`, or `tp unclaim %s`.", task.ID, task.ID, task.ID))
		os.Exit(ExitState)
		return nil
	}
	var retired []string
	if task.ClaimedBy != nil {
		retired = task.ClaimedBy.Retired
	}
	if claimToken != "" && slices.Contains(retired, claimToken) {
		output.Error(ExitState,
			fmt.Sprintf("cannot claim: task %s was taken over from the agent spawned from this brief", task.ID),
			fmt.Sprintf("run tp brief %s --file %s again and spawn from that fresh brief", task.ID, taskFileLabel(taskFilePath)))
		os.Exit(ExitState)
		return nil
	}
	task.ClaimedBy = &model.ClaimedBy{Session: claimSession, Spawn: claimSpawn, Token: claimToken, Retired: retired}
	return writeClaim(tf, taskFilePath, task, fmt.Sprintf("claimed %s for spawn %s", task.ID, claimSpawn))
}

// takeoverClaim is tp claim <id> --takeover: a wip task's claim is freed from
// its spawn, keeping started_at and the session, so the next spawn from the
// brief binds it as it binds a coordinator's own claim. A task that is not
// wip exits 4.
func takeoverClaim(tf *model.TaskFile, taskFilePath, id string) error {
	task := findOrExit(tf, id)
	if task.Status != model.StatusWIP {
		output.Error(ExitState, fmt.Sprintf("cannot take over: task %s is %s (must be wip)", task.ID, task.Status))
		os.Exit(ExitState)
		return nil
	}
	if task.ClaimedBy != nil {
		session := task.ClaimedBy.Session
		task.ClaimedBy = task.ClaimedBy.Released()
		if session != "" {
			if task.ClaimedBy == nil {
				task.ClaimedBy = &model.ClaimedBy{}
			}
			task.ClaimedBy.Session = session
		}
	}
	return writeClaim(tf, taskFilePath, task, fmt.Sprintf("freed %s from its spawn", task.ID))
}

func findOrExit(tf *model.TaskFile, id string) *model.Task {
	task, _, err := model.FindTask(tf, id)
	if err != nil {
		output.Error(ExitState, err.Error())
		os.Exit(ExitState)
	}
	return task
}

func writeClaim(tf *model.TaskFile, taskFilePath string, task *model.Task, msg string) error {
	tf.UpdatedAt = time.Now().UTC()
	if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
		// task-file-hint: the write that failed is the task file's.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	output.Success(msg)
	printResultOrExit(taskWritten{Task: task, File: taskFileLabel(taskFilePath)})
	return nil
}
