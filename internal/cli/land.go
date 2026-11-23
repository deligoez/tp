package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

func newLandCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "land",
		Short: "Pick worktree agents' finished work onto this tree's branch and close their tasks",
		Args:  cobra.NoArgs,
		RunE:  runLand,
	}
}

// landItem is one land request tp land could not land.
type landItem struct {
	ID    string `json:"id"`
	Error string `json:"error"`
	Log   string `json:"log,omitempty"`
	Hint  string `json:"hint"`
}

// landResult is tp land's payload. Landed tasks are closed on this tree's
// branch; reopened ones failed to land and wait for a fresh agent; blocked
// ones keep their request until what blocks them is cleared; dropped requests
// were for tasks no longer wip.
type landResult struct {
	Landed   []string   `json:"landed"`
	Reopened []landItem `json:"reopened"`
	Blocked  []landItem `json:"blocked"`
	Dropped  []string   `json:"dropped"`
}

// lander carries what landing one request needs.
type lander struct {
	root, taskFile, dir string
	close               engine.CloseFunc
	reopen              func(id string) error
	res                 *landResult
}

// runLand is the coordinator's half of a worktree agent's close
// (spec/backlog/worktree-land.md). It exits 4 when a request is blocked,
// since that needs this tree's own changes committed first.
func runLand(_ *cobra.Command, _ []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		// task-file-hint: no task file was found to land into
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
	}
	enterTree(taskFilePath, "tp land")
	if abs, absErr := filepath.Abs(taskFilePath); absErr == nil {
		taskFilePath = abs
	}
	dir := engine.LandDir(taskFilePath)
	requests, err := engine.ReadLandRequests(dir)
	if err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	l := &lander{
		root: root, taskFile: taskFilePath, dir: dir,
		close:  closeTaskFunc(root, taskFilePath),
		reopen: reopenSoloFunc(taskFilePath),
		res:    &landResult{Landed: []string{}, Reopened: []landItem{}, Blocked: []landItem{}, Dropped: []string{}},
	}
	for i := range requests {
		l.land(&requests[i])
	}
	if jsonErr := output.JSON(l.res); jsonErr != nil {
		output.Error(ExitFile, jsonErr.Error(), internalEncodeHint)
	}
	if len(l.res.Blocked) > 0 {
		os.Exit(ExitState)
	}
	return nil
}

// land picks one request's commits, closes its task with them, and on a
// failure takes the pick back and reopens the task for a fresh agent.
func (l *lander) land(r *engine.LandRequest) {
	if taskStatusIn(l.taskFile, r.ID) != model.StatusWIP {
		l.drop(r.ID)
		l.res.Dropped = append(l.res.Dropped, r.ID)
		return
	}
	pre, landed, err := engine.LandPick(l.root, r.Commits)
	var blocked *engine.LandBlockedError
	if errors.As(err, &blocked) {
		l.res.Blocked = append(l.res.Blocked, landItem{ID: r.ID, Error: err.Error(),
			Hint: "commit those paths on this tree's branch, then run tp land again"})
		return
	}
	if err != nil {
		l.reopenAfter(r.ID, "", err, "")
		return
	}
	logs := filepath.Join(l.dir, "logs")
	if mkErr := os.MkdirAll(logs, 0o750); mkErr != nil {
		l.reopenAfter(r.ID, pre, mkErr, "")
		return
	}
	logPath := filepath.Join(logs, r.ID+".log")
	closeErr := l.close(r.ID, r.Reason, landed, logPath)
	var refused *engine.CloseRefusedError
	switch {
	case closeErr == nil || taskStatusIn(l.taskFile, r.ID) == model.StatusDone:
		l.drop(r.ID)
		l.res.Landed = append(l.res.Landed, r.ID)
	case errors.As(closeErr, &refused):
		_ = engine.LandUndo(l.root, pre)
		l.res.Blocked = append(l.res.Blocked, landItem{ID: r.ID, Error: "tp done refused the close", Log: logPath,
			Hint: "read the log; the request is kept for the next tp land"})
	default:
		l.reopenAfter(r.ID, pre, closeErr, logPath)
	}
}

// reopenAfter takes a failed landing back to pre (when it picked anything),
// reopens the task to run alone, and drops its request.
func (l *lander) reopenAfter(id, pre string, cause error, logPath string) {
	if pre != "" {
		_ = engine.LandUndo(l.root, pre)
	}
	item := landItem{ID: id, Error: cause.Error(), Log: logPath,
		Hint: fmt.Sprintf("task %s is open again: spawn a fresh agent from tp brief %s", id, id)}
	if err := l.reopen(id); err != nil {
		item.Error += "; reopening failed: " + err.Error()
		item.Hint = "the request is kept for the next tp land"
		l.res.Blocked = append(l.res.Blocked, item)
		return
	}
	l.drop(id)
	l.res.Reopened = append(l.res.Reopened, item)
}

func (l *lander) drop(id string) {
	if err := engine.RemoveLandRequest(l.dir, id); err != nil {
		output.Notice(fmt.Sprintf("the land request of %s could not be removed: %v", id, err))
	}
}

// taskStatusIn is task id's status in the task file at path, "" when it
// cannot be read.
func taskStatusIn(path, id string) string {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return ""
	}
	task, _, err := model.FindTask(tf, id)
	if err != nil {
		return ""
	}
	return task.Status
}
