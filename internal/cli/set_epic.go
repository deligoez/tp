package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// epicBindingRefusedHint answers a binding §2 refuses.
const epicBindingRefusedHint = "bind the file to a work epic the graph holds that is not done: tp epic lint lists the graph's epics"

// epicBindingWritten is tp set --epic's payload: the file written and the
// epic it now names, empty once the binding is removed.
type epicBindingWritten struct {
	File string `json:"file"`
	Epic string `json:"epic"`
}

// runSetEpic implements tp set --epic <id> (spec/1.5.0.md §2): it writes id
// as the task file's epic, or removes the binding when id is empty.
func runSetEpic(args []string, id string) error {
	if len(args) != 0 {
		output.Error(ExitUsage, "tp set --epic takes no positional arguments", `tp set --epic <id>, or tp set --epic "" to remove the binding`)
		os.Exit(ExitUsage)
	}
	if id != "" {
		exitOnEpicBindingRefusal(id)
	}
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file was found to bind.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			// task-file-hint: the read that failed is the task file's.
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
		}
		tf.Epic = id
		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			// task-file-hint: the write that failed is the task file's.
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
		}
		output.Success(fmt.Sprintf("%s epic=%q", taskFileLabel(taskFilePath), id))
		return output.JSON(epicBindingWritten{File: taskFileLabel(taskFilePath), Epic: id})
	})
}

// exitOnEpicBindingRefusal exits 1 when §2 refuses binding a task file to
// epic id, and 3 when the graph file cannot be read; it returns otherwise.
func exitOnEpicBindingRefusal(id string) {
	err := engine.CheckEpicBinding(engine.ProjectConfigDir("."), id)
	if err == nil {
		return
	}
	var refused *engine.EpicBindingRefusedError
	if errors.As(err, &refused) {
		output.Error(ExitValidation, err.Error(), epicBindingRefusedHint)
		os.Exit(ExitValidation)
	}
	output.Error(ExitFile, err.Error(), epicGraphFileHint)
	os.Exit(ExitFile)
}
