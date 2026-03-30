package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/output"
)

var (
	version = "dev"

	flagFile    string
	flagJSON    bool
	flagCompact bool
	flagQuiet   bool
	flagNoColor bool
)

func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tp",
		Short: "tp — task-plan: spec-to-task lifecycle for AI agents",
		Long:  "tp manages spec-to-task lifecycle for AI coding agents. Break specs into atomic, dependency-ordered tasks.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.PersistentFlags().StringVar(&flagFile, "file", "", "explicit task file (default: auto-detect *.tasks.json)")
	cmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "force JSON output")
	cmd.PersistentFlags().BoolVar(&flagCompact, "compact", false, "minimal JSON: omit descriptions, source_lines, metadata")
	cmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "suppress info-level output")
	cmd.PersistentFlags().BoolVar(&flagNoColor, "no-color", false, "disable colored output")

	cmd.Version = version
	cmd.SetVersionTemplate("tp version {{.Version}}\n")

	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		output.Configure(flagJSON, flagQuiet, flagNoColor)
	}

	// Register subcommands
	cmd.AddCommand(newLintCmd())
	cmd.AddCommand(newValidateCmd())
	cmd.AddCommand(newInitCmd())
	cmd.AddCommand(newAddCmd())
	cmd.AddCommand(newImportCmd())
	cmd.AddCommand(newReadyCmd())
	cmd.AddCommand(newShowCmd())
	cmd.AddCommand(newClaimCmd())
	cmd.AddCommand(newCloseCmd())
	cmd.AddCommand(newReopenCmd())
	cmd.AddCommand(newRemoveCmd())
	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newStatusCmd())
	cmd.AddCommand(newBlockedCmd())
	cmd.AddCommand(newGraphCmd())
	cmd.AddCommand(newStatsCmd())
	cmd.AddCommand(newPlanCmd())
	cmd.AddCommand(newDoneCmd())
	cmd.AddCommand(newNextCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}

func Execute() {
	if os.Getenv("NO_COLOR") != "" {
		flagNoColor = true
	}

	cmd := NewRootCmd()
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func GetFileFlag() string    { return flagFile }
func IsJSONOutput() bool     { return flagJSON }
func IsQuiet() bool          { return flagQuiet }
func IsNoColor() bool        { return flagNoColor }
func IsCompact() bool        { return flagCompact }
