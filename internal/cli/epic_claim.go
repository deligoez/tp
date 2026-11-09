package cli

import (
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// epicClaimUsageHint answers a malformed tp epic claim call.
const epicClaimUsageHint = "tp epic claim <id> [--by <name>] or tp epic claim <id> --release"

// epicClaimRefusedHint answers a claim refused on an epic.
const epicClaimRefusedHint = "pick another epic from tp epic ready; a held claim is freed with tp epic claim <id> --release, and a bound epic is worked through its task file"

// epicClaimDefaultHolder is the holder a claim without --by names when $USER
// is unset.
const epicClaimDefaultHolder = "agent"

func newEpicClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim <id>",
		Short: "Reserve an epic for a holder, or free it with --release; refused when a claim exists, on a condition, a done epic and a bound epic",
		Args:  cobra.ExactArgs(1),
		RunE:  runEpicClaim,
	}
	cmd.Flags().String("by", "", "the claim's holder (default $USER, else \"agent\")")
	cmd.Flags().Bool("release", false, "remove the epic's claim, on a bound epic too")
	epicIfRevisionFlag(cmd)
	return cmd
}

// runEpicClaim implements tp epic claim (spec/1.5.0.md §7): one locked write
// that exits 1 on a refusal and 4 for an absent epic; --by with --release
// exits 2.
func runEpicClaim(cmd *cobra.Command, args []string) error {
	if release, _ := cmd.Flags().GetBool("release"); release {
		if cmd.Flags().Changed("by") {
			output.Error(ExitUsage, "--by names a new claim's holder and cannot go with --release", epicClaimUsageHint)
			os.Exit(ExitUsage)
		}
		writeEpicGraph(cmd, engine.ReleaseEpicClaim(args[0]))
		return nil
	}
	writeEpicGraph(cmd, engine.ClaimEpic(args[0], epicClaimHolder(cmd), ".", time.Now()))
	return nil
}

// epicClaimHolder is --by, else $USER, else epicClaimDefaultHolder.
func epicClaimHolder(cmd *cobra.Command) string {
	if by, _ := cmd.Flags().GetString("by"); by != "" {
		return by
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return epicClaimDefaultHolder
}
