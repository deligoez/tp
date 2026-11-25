package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

var (
	poolSession    string
	poolLive       []string
	poolUnresolved int
	poolStopHook   bool
)

func newPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool --session <id> [--live <spawn>]...",
		Short: "A coordinator session's parallel slots: running, parked, orphaned claims and ready tasks",
		Long: `Report the pool of a session that claimed tasks through Agent spawns
(spec/backlog/filled-pool.md): its claims in the current tree classified as
running (the spawn is in --live), parked (waiting set in some worktree's copy),
orphaned, freed for a new spawn, or closed in another worktree; the cap
(run_max_parallel); the free slots; and the ready tasks of the task files the
session claimed in. ask is true when the session is a coordinator, a slot is
free and a ready task or an unbound claim could fill it.

--stop-hook reads Claude Code's Stop hook input on stdin instead and prints
only the hook's block decision, or nothing; it always exits 0.`,
		Args: cobra.NoArgs,
		RunE: runPool,
	}
	cmd.Flags().StringVar(&poolSession, "session", "", "the coordinator's session id")
	cmd.Flags().StringSliceVar(&poolLive, "live", nil, "spawn ids (tool_use_id) of the session's live agents; repeatable or comma-separated")
	cmd.Flags().IntVar(&poolUnresolved, "unresolved", 0, "live agents whose spawn id is unknown; each holds a slot")
	cmd.Flags().BoolVar(&poolStopHook, "stop-hook", false, "read a Stop hook payload on stdin and print the block decision")
	return cmd
}

func runPool(_ *cobra.Command, _ []string) error {
	if poolStopHook {
		poolFromStopHook(os.Stdin, os.Stdout)
		return nil
	}
	if !engine.ValidClaimToken(poolSession) {
		output.Error(ExitUsage, fmt.Sprintf("--session %q: want 1-128 of A-Z a-z 0-9 _ -", poolSession))
		os.Exit(ExitUsage)
		return nil
	}
	return output.JSON(engine.PoolOf(engine.PoolInput{Start: ".", Session: poolSession, Live: poolLive, Unresolved: poolUnresolved}))
}

// poolFromStopHook is --stop-hook: a payload it cannot read, a second stop
// (stop_hook_active) or a pool with nothing to ask prints nothing, since this
// hook only nudges and must never wedge a session. A payload with no
// background_tasks says nothing of which agents live, so no claim may be
// called lost: it asks only for unapplied found requests (spec/1.31.0.md §2).
func poolFromStopHook(in io.Reader, out io.Writer) {
	var payload engine.StopPayload
	if json.NewDecoder(in).Decode(&payload) != nil || payload.StopHookActive || payload.Cwd == "" {
		return
	}
	live, unresolved := payload.LiveSpawns()
	p := engine.PoolOf(engine.PoolInput{Start: payload.Cwd, Session: payload.SessionID, Live: live, Unresolved: unresolved})
	reason := p.Reason
	if payload.BackgroundTasks == nil {
		// Unapplied found requests need no agent list to be named.
		reason = ""
		if len(p.Unapplied) > 0 {
			reason = engine.FoundApplyReason(p.Unapplied)
		}
	}
	if !p.Ask || reason == "" {
		return
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(map[string]string{"decision": "block", "reason": reason})
}
