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
// were for tasks no longer wip under their claim, and unlanded names those
// whose task was closed without the request's commits.
type landResult struct {
	Landed   []string   `json:"landed"`
	Reopened []landItem `json:"reopened"`
	Blocked  []landItem `json:"blocked"`
	Dropped  []string   `json:"dropped"`
	Unlanded []landItem `json:"unlanded"`
	// Commit is tp's own commit of the task file this land wrote
	// (spec/backlog/land-lifecycle.md §2.1); TaskFileChanged names the task
	// file when it is left changed for the agent to commit instead.
	Commit          string `json:"commit,omitempty"`
	TaskFileChanged string `json:"task_file_changed,omitempty"`
	CommitError     string `json:"commit_error,omitempty"`
	// Pruned are the landed worktrees removed with their branches, and
	// kept those left in place, each with why (§2.2).
	Pruned []landPruned `json:"pruned"`
	Kept   []landKept   `json:"kept"`
}

// landPruned is a landed task's worktree tp land removed, with its branch.
type landPruned struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
	Branch   string `json:"branch,omitempty"`
}

// landKept is a landed task's worktree tp land left in place, and why.
type landKept struct {
	ID       string `json:"id"`
	Worktree string `json:"worktree"`
	Reason   string `json:"reason"`
}

// lander carries what landing one request needs.
type lander struct {
	root, taskFile, dir string
	close               engine.CloseFunc
	reopen              func(id string) error
	res                 *landResult
	// landed are the requests landed, whose worktrees may be pruned.
	landed []engine.LandRequest
}

// runLand is the coordinator's half of a worktree agent's close
// (spec/backlog/worktree-land.md). It exits 4 when a request is blocked,
// since that needs this tree's own changes committed first, and 3 when its
// commit of the task file fails.
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
		res: &landResult{Landed: []string{}, Reopened: []landItem{}, Blocked: []landItem{},
			Dropped: []string{}, Unlanded: []landItem{}, Pruned: []landPruned{}, Kept: []landKept{}},
	}
	for _, path := range unreadable {
		l.res.Blocked = append(l.res.Blocked, landItem{ID: strings.TrimSuffix(filepath.Base(path), ".json"),
			Error: "the land request " + path + " cannot be read",
			Hint:  "delete it (rm " + path + "); the task keeps its claim, and its agent asks again with tp done or the pool offers the claim to a fresh one"})
	}
	for i := range requests {
		l.land(&requests[i])
	}
	l.commitTaskFile()
	l.pruneWorktrees()
	if jsonErr := output.JSON(l.res); jsonErr != nil {
		output.Error(ExitFile, jsonErr.Error(), internalEncodeHint)
	}
	if l.res.CommitError != "" {
		os.Exit(ExitFile)
	}
	if len(l.res.Blocked) > 0 {
		os.Exit(ExitState)
	}
	return nil
}

// commitTaskFile commits what this land wrote to the task file, once for
// every task it landed or reopened (spec/backlog/land-lifecycle.md §2.1).
// tp makes the commit only under builtin, as tp done --auto-commit does, and
// only in the main worktree, where claims may be committed freely; otherwise
// the output names the task file left changed for the agent to commit.
func (l *lander) commitTaskFile() {
	if len(l.res.Landed)+len(l.res.Reopened) == 0 || !engine.TaskFileChanged(l.root, l.taskFile) {
		return
	}
	if resolveEffectiveStrategy(l.taskFile) != engine.CommitStrategyBuiltin || engine.InLinkedWorktree(l.root) {
		l.res.TaskFileChanged = l.relFile()
		return
	}
	sha, err := engine.LandCommitTaskFile(l.root, l.taskFile, landCommitMessage(l.res))
	if err != nil {
		l.res.TaskFileChanged = l.relFile()
		l.res.CommitError = err.Error()
		return
	}
	l.res.Commit = sha
}

// pruneWorktrees removes each landed request's worktree and branch when that
// is safe, and names each one kept with why (spec/backlog/land-lifecycle.md
// §2.2). Every commit of a landed request was picked: tp land lands a
// request whole or not at all.
func (l *lander) pruneWorktrees() {
	for i := range l.landed {
		r := &l.landed[i]
		if r.Worktree == "" {
			continue
		}
		branch, reason, found := engine.PruneLandedWorktree(l.root, r.Worktree, r.Commits)
		switch {
		case !found:
		case reason != "":
			l.res.Kept = append(l.res.Kept, landKept{ID: r.ID, Worktree: r.Worktree, Reason: reason})
		default:
			l.res.Pruned = append(l.res.Pruned, landPruned{ID: r.ID, Worktree: r.Worktree, Branch: branch})
		}
	}
}

// landCommitMessage is the message of a land's task-file commit, naming the
// tasks it closed and those it reopened.
func landCommitMessage(res *landResult) string {
	reopened := make([]string, 0, len(res.Reopened))
	for _, item := range res.Reopened {
		reopened = append(reopened, item.ID)
	}
	if len(res.Landed) == 0 {
		return "chore(tp): reopen " + strings.Join(reopened, ", ") + " after a failed land"
	}
	msg := "chore(tp): record " + strings.Join(res.Landed, ", ") + " closure"
	if len(reopened) > 0 {
		msg += "\n\nReopened: " + strings.Join(reopened, ", ")
	}
	return msg
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
	return r.MadeUnder(task.ClaimedBy)
}

// land picks one request's commits, closes its task with them, and on a
// failure takes the pick back and reopens the task for a fresh agent.
func (l *lander) land(r *engine.LandRequest) {
	if !holdsRequest(l.taskFile, r) {
		l.dropStale(r)
		return
	}
	session := landClaimSession(l.taskFile, r.ID)
	pre, landed, err := engine.LandPick(l.root, r.Commits)
	var blocked *engine.LandBlockedError
	if errors.As(err, &blocked) {
		hint := "commit, stash or revert those paths on this tree's branch, then run tp land again"
		switch {
		case blocked.Abort != "":
			hint = "finish it, or run " + blocked.Abort + " in this tree, then run tp land again"
		case blocked.Cause != "":
			hint = "clear what git reports in this tree, then run tp land again"
		}
		l.res.Blocked = append(l.res.Blocked, landItem{ID: r.ID, Error: err.Error(), Hint: hint})
		return
	}
	if err != nil {
		l.reopenAfter(r, session, "", err.Error(), "")
		return
	}
	logs := filepath.Join(l.dir, "logs")
	if mkErr := os.MkdirAll(logs, 0o750); mkErr != nil {
		l.reopenAfter(r, session, pre, mkErr.Error(), "")
		return
	}
	logPath := filepath.Join(logs, r.ID+".log")
	closeErr := l.close(r.ID, r.Reason, landed, logPath)
	var refused *engine.CloseRefusedError
	switch {
	case closeErr == nil || taskStatusIn(l.taskFile, r.ID) == model.StatusDone:
		l.outcome(r, session, "")
		l.drop(r)
		l.res.Landed = append(l.res.Landed, r.ID)
		l.landed = append(l.landed, *r)
	case errors.As(closeErr, &refused):
		_ = engine.LandUndo(l.root, pre)
		l.res.Blocked = append(l.res.Blocked, landItem{ID: r.ID, Error: "tp done refused the close: " + closeFailure(closeErr, logPath), Log: logPath,
			Hint: "the request is kept: clear what the error names, then run tp land again"})
	default:
		l.reopenAfter(r, session, pre, closeFailure(closeErr, logPath), logPath)
	}
}

// reopenAfter takes a failed landing back to pre (when it picked anything),
// reopens the task to run alone, records the outcome for the agent's
// worktree under the claim's session, and drops its request.
func (l *lander) reopenAfter(r *engine.LandRequest, session, pre, cause, logPath string) {
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
	l.outcome(r, session, cause)
	l.drop(r)
	l.res.Reopened = append(l.res.Reopened, item)
}

// dropStale drops a request whose task is no longer wip under its claim, and
// still records an outcome for the agent's worktree: landed when the task is
// done here and this branch carries every commit, else why not. A task closed
// without the commits names them, so they are not lost without a word.
func (l *lander) dropStale(r *engine.LandRequest) {
	cause := ""
	if taskStatusIn(l.taskFile, r.ID) != model.StatusDone {
		cause = fmt.Sprintf("task %s is no longer claimed under the claim this worktree's request was made under", r.ID)
	} else if missing := l.uncarried(r.Commits); len(missing) > 0 {
		cause = fmt.Sprintf("task %s was closed without the commits %s", r.ID, strings.Join(missing, ", "))
		l.res.Unlanded = append(l.res.Unlanded, landItem{ID: r.ID, Error: cause,
			Hint: fmt.Sprintf("if they belong to task %s, run git cherry-pick %s in this tree and commit; otherwise leave them", r.ID, strings.Join(missing, " "))})
	}
	l.outcome(r, "", cause)
	l.drop(r)
	l.res.Dropped = append(l.res.Dropped, r.ID)
}

// uncarried lists the commits this tree's branch does not carry.
func (l *lander) uncarried(commits []string) []string {
	missing := []string{}
	for _, sha := range commits {
		if !engine.CarriedOnHead(l.root, sha) {
			missing = append(missing, sha)
		}
	}
	return missing
}

// outcome records what this landing did for the request's worktree; cause
// is "" for a task landed and closed.
func (l *lander) outcome(r *engine.LandRequest, session, cause string) {
	o := &engine.LandOutcome{ID: r.ID, Worktree: r.Worktree, Session: session, Landed: cause == "", Error: cause}
	if err := engine.WriteLandOutcome(l.dir, o); err != nil {
		output.Notice(fmt.Sprintf("the land outcome of %s could not be recorded: %v", r.ID, err))
	}
}

// drop removes the request r was read as; one written since is kept.
func (l *lander) drop(r *engine.LandRequest) {
	if err := engine.RemoveLandRequest(l.dir, r); err != nil {
		output.Notice(fmt.Sprintf("the land request of %s could not be removed: %v", r.ID, err))
	}
}

// landClaimSession is the session of task id's claim in the task file at path,
// "" when it has none.
func landClaimSession(path, id string) string {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return ""
	}
	task, _, err := model.FindTask(tf, id)
	if err != nil || task.ClaimedBy == nil {
		return ""
	}
	return task.ClaimedBy.Session
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
