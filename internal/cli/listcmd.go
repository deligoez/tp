package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	listStatus string
	listTag    string
	listIDs    bool
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks with filters",
		RunE:  runList,
	}
	cmd.Flags().StringVar(&listStatus, "status", "", "filter by status (comma-separated: open,wip)")
	cmd.Flags().StringVar(&listTag, "tag", "", "filter by tag")
	cmd.Flags().BoolVar(&listIDs, "ids", false, "just IDs (newline-separated)")
	return cmd
}

func runList(_ *cobra.Command, _ []string) error {
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

	// Build status filter set
	statusFilter := make(map[string]bool)
	if listStatus != "" {
		for _, s := range strings.Split(listStatus, ",") {
			statusFilter[strings.TrimSpace(s)] = true
		}
	}

	filtered := make([]model.Task, 0, len(tf.Tasks))
	for i := range tf.Tasks {
		t := &tf.Tasks[i]

		if len(statusFilter) > 0 && !statusFilter[t.Status] {
			continue
		}

		if listTag != "" {
			hasTag := false
			for _, tag := range t.Tags {
				if tag == listTag {
					hasTag = true
					break
				}
			}
			if !hasTag {
				continue
			}
		}

		filtered = append(filtered, *t)
	}

	if listIDs {
		for i := range filtered {
			fmt.Println(filtered[i].ID)
		}
		return nil
	}

	if flagCompact {
		return output.JSON(output.CompactTasks(filtered))
	}

	return output.JSON(filtered)
}
