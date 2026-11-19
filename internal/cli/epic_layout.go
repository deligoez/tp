package cli

import (
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/output"
)

// epicLayoutUsageHint answers a tp epic layout invocation the shared rules
// refuse, in the command's flags rather than the endpoint's query.
const epicLayoutUsageHint = "name one view: --view plan (the default) or milestones, --view scope --milestone <m>, or --view neighbourhood --epic <id> [--depth <n>], depth clamped to 1-4, each with an optional --pack rows|tree"

// epicLayoutFlags are the command's flags, each the /api/layout parameter
// of the same name.
var epicLayoutFlags = []string{"view", "milestone", "epic", "depth", layoutPackParam}

func newEpicLayoutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "layout",
		Short: "Print one view of the epic map laid out, the payload tp dashboard's /api/layout serves; writes nothing",
		Args:  cobra.NoArgs,
		RunE:  runEpicLayout,
	}
	cmd.Flags().String("view", layoutViewPlan, "plan, milestones, scope or neighbourhood")
	cmd.Flags().String("milestone", "", "the gate a scope view lays out")
	cmd.Flags().String("epic", "", "the epic a neighbourhood view centres on")
	cmd.Flags().String("depth", "", "a neighbourhood's steps each way, clamped to 1-4 (default 2)")
	cmd.Flags().String(layoutPackParam, "", "how the forest's columns pack: rows (the default) or tree")
	return cmd
}

// runEpicLayout prints epicLayout's payload for the flags given, read as
// the endpoint reads its query; a refusal exits with the endpoint's code.
func runEpicLayout(cmd *cobra.Command, _ []string) error {
	q := url.Values{}
	for _, name := range epicLayoutFlags {
		if name == "view" || cmd.Flags().Changed(name) {
			v, _ := cmd.Flags().GetString(name)
			q.Set(name, v)
		}
	}
	out, derr := epicLayout(q)
	if derr != nil {
		hint := derr.Hint
		if derr.Code == ExitUsage {
			hint = epicLayoutUsageHint
		}
		output.Error(derr.Code, derr.Message, hint)
		os.Exit(derr.Code)
	}
	if err := output.JSON(out); err != nil {
		output.Error(ExitFile, err.Error(), internalEncodeHint)
		os.Exit(ExitFile)
	}
	return nil
}
