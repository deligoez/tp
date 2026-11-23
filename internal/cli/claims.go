package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

var claimsSpawn string

func newClaimsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claims --spawn <tool_use_id>",
		Short: "Tasks an Agent spawn claimed, and whether each is done or waiting",
		Long: `List the tasks claimed for one Agent spawn in any task file of the
repository, reading every worktree's copy: a task is accounted for when some
copy has it done, or wip with waiting set (waiting on an open copy is a hold,
not a parked claim). Prints one line of JSON; the plugin's
SubagentStop hook reads its unaccounted ids.`,
		Args: cobra.NoArgs,
		RunE: runClaims,
	}
	cmd.Flags().StringVar(&claimsSpawn, "spawn", "", "the spawn's tool_use_id (required)")
	return cmd
}

func runClaims(_ *cobra.Command, _ []string) error {
	if !engine.ValidClaimToken(claimsSpawn) {
		output.Error(ExitUsage, fmt.Sprintf("--spawn %q: want 1-128 of A-Z a-z 0-9 _ -", claimsSpawn))
		os.Exit(ExitUsage)
		return nil
	}
	raw, err := json.Marshal(engine.ClaimsOfSpawn(".", claimsSpawn))
	if err != nil {
		return err
	}
	_, err = fmt.Println(string(raw))
	return err
}
