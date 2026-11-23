package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	requests, unreadable, err := engine.ReadLandRequests(dir)
	if err != nil {
		output.Error(ExitFile, err.Error(), runtimeFailureHint)
		os.Exit(ExitFile)
	}
	refuseLandOutsideHolder(taskFilePath, requests)
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	l := &lander{
		root: root, taskFile: taskFilePath, dir: dir,
		close:  closeTaskFunc(root, taskFilePath),
		reopen: reopenSoloFunc(taskFilePath),
		res:    &landResult{Landed: []string{}, Reopened: []landItem{}, Blocked: []landItem{}, Dropped: []string{}},
	}
	for _, path := range unreadable {
		l.res.Blocked = append(l.res.Blocked, landItem{ID: strings.TrimSuffix(filepath.Base(path), ".json"),
			Error: "the land request " + path + " cannot be read",
			Hint:  "delete it (rm " + path + "); the task keeps its claim, and its agent asks again with tp done or the pool offers the claim to a fresh one"})
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

// refuseLandOutsideHolder exits 2 before anything is touched when another
// tree holds a claim a request was made under and this one does not: run
// from an agent's worktree, or from a tree with a claim of its own, tp land
// would drop the request or land it on the wrong branch.
func refuseLandOutsideHolder(taskFilePath string, requests []engine.LandRequest) {
	for i := range requests {
		r := &requests[i]
		if holdsRequest(taskFilePath, r) {
			continue
		}
		if holder := engine.SpawnHolderCopy(taskFilePath, r.ID, r.Spawn); holder != "" {
			exitUsage(fmt.Sprintf("tp land runs in the tree that holds the claims, and task %s is claimed in %s", r.ID, holder),
				"an agent in its own worktree stops after tp done; the coordinator runs tp land in its own tree")
		}
	}
}

// holdsRequest reports whether the copy at taskFilePath has r's task wip
// under the claim r was made under.
func holdsRequest(taskFilePath string, r *engine.LandRequest) bool {
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		return false
	}
	task, _, err := model.FindTask(tf, r.ID)
	if err != nil || task.Status != model.StatusWIP {
		return false
	}
	return r.Spawn == "" || (task.ClaimedBy != nil && task.ClaimedBy.Spawn == r.Spawn)
}

// land picks one request's commits, closes its task with them, and on a
// failure takes the pick back and reopens the task for a fresh agent.
func (l *lander) land(r *engine.LandRequest) {
	if !holdsRequest(l.taskFile, r) {
		l.drop(r.ID)
		l.res.Dropped = append(l.res.Dropped, r.ID)
		return
	}
	pre, landed, err := engine.LandPick(l.root, r.Commits)
	var blocked *engine.LandBlockedError
	if errors.As(err, &blocked) {
		hint := "commit, stash or revert those paths on this tree's branch, then run tp land again"
		if blocked.Cause != "" {
			hint = "finish or abort what git names in this tree (a cherry-pick, merge or rebase in progress), then run tp land again"
		}
		l.res.Blocked = append(l.res.Blocked, landItem{ID: r.ID, Error: err.Error(), Hint: hint})
		return
	}
	if err != nil {
		l.reopenAfter(r, "", err.Error(), "")
		return
	}
	logs := filepath.Join(l.dir, "logs")
	if mkErr := os.MkdirAll(logs, 0o750); mkErr != nil {
		l.reopenAfter(r, pre, mkErr.Error(), "")
		return
	}
	logPath := filepath.Join(logs, r.ID+".log")
	closeErr := l.close(r.ID, r.Reason, landed, logPath)
	var refused *engine.CloseRefusedError
	switch {
	case closeErr == nil || taskStatusIn(l.taskFile, r.ID) == model.StatusDone:
		l.outcome(r, "")
		l.drop(r.ID)
		l.res.Landed = append(l.res.Landed, r.ID)
	case errors.As(closeErr, &refused):
		_ = engine.LandUndo(l.root, pre)
		l.res.Blocked = append(l.res.Blocked, landItem{ID: r.ID, Error: "tp done refused the close: " + closeFailure(closeErr, logPath), Log: logPath,
			Hint: "the request is kept: clear what the error names, then run tp land again"})
	default:
		l.reopenAfter(r, pre, closeFailure(closeErr, logPath), logPath)
	}
}

// reopenAfter takes a failed landing back to pre (when it picked anything),
// reopens the task to run alone, records the outcome for the agent's
// worktree, and drops its request.
func (l *lander) reopenAfter(r *engine.LandRequest, pre, cause, logPath string) {
	if pre != "" {
		_ = engine.LandUndo(l.root, pre)
	}
	item := landItem{ID: r.ID, Error: cause, Log: logPath,
		Hint: fmt.Sprintf("task %s is open again: spawn a fresh agent from tp brief %s --file %s, and tell it this error", r.ID, r.ID, l.relFile())}
	if err := l.reopen(r.ID); err != nil {
		item.Error += "; reopening failed: " + err.Error()
		item.Hint = "the request is kept for the next tp land"
		l.res.Blocked = append(l.res.Blocked, item)
		return
	}
	l.outcome(r, cause)
	l.drop(r.ID)
	l.res.Reopened = append(l.res.Reopened, item)
}

// outcome records what this landing did for the request's worktree; cause
// is "" for a task landed and closed.
func (l *lander) outcome(r *engine.LandRequest, cause string) {
	o := &engine.LandOutcome{ID: r.ID, Worktree: r.Worktree, Landed: cause == "", Error: cause}
	if err := engine.WriteLandOutcome(l.dir, o); err != nil {
		output.Notice(fmt.Sprintf("the land outcome of %s could not be recorded: %v", r.ID, err))
	}
}

func (l *lander) drop(id string) {
	if err := engine.RemoveLandRequest(l.dir, id); err != nil {
		output.Notice(fmt.Sprintf("the land request of %s could not be removed: %v", id, err))
	}
}

// closeFailure is why the tp done child failed: the error its log ends
// with, or err itself when the log names none.
func closeFailure(err error, logPath string) string {
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		return err.Error()
	}
	// The child's error is a JSON object, pretty-printed, opening a line.
	cause := ""
	for i := range data {
		if data[i] != '{' || (i > 0 && data[i-1] != '\n') {
			continue
		}
		var e struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(bytes.NewReader(data[i:])).Decode(&e) == nil && e.Error != "" {
			cause = e.Error
		}
	}
	if cause == "" {
		return err.Error()
	}
	return cause
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

// relFile is the task file relative to the repository root, where the
// pool's commands run from.
func (l *lander) relFile() string {
	return engine.RepoRelativePath(l.taskFile)
}
