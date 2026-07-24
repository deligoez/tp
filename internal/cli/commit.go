package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var commitFiles string

func newCommitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "commit <id> [reason]",
		Short: "Stage, commit with structured message, record SHA on task",
		Long: `Atomic commit for a task. Stages changes, generates a structured commit
message from task metadata, commits, and records the SHA on the task.
Implicitly claims open tasks (open → wip).
Output: {id, sha, message}`,
		Example: `  tp commit auth-model "Model + migration done"
  tp commit auth-model --files "app/Models/*,migrations/*"`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runCommit,
	}
	cmd.Flags().StringVar(&commitFiles, "files", "", "comma-separated file globs to stage (default: all dirty)")
	return cmd
}

func runCommit(_ *cobra.Command, args []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	if resolveEffectiveStrategy(taskFilePath) == engine.CommitStrategyHC {
		output.Error(ExitUsage, "tp commit is not valid under commit_strategy hc", hcCommitHint)
		os.Exit(ExitUsage)
		return nil
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		task, _, err := model.FindTask(tf, args[0])
		if err != nil {
			output.Error(ExitState, err.Error())
			os.Exit(ExitState)
			return nil
		}

		// Implicit claim
		if task.Status == model.StatusOpen {
			done := make(map[string]bool)
			for i := range tf.Tasks {
				if tf.Tasks[i].Status == model.StatusDone {
					done[tf.Tasks[i].ID] = true
				}
			}
			for _, dep := range task.DependsOn {
				if !done[dep] {
					output.Error(ExitState, fmt.Sprintf("cannot commit: task %s is blocked by %s", task.ID, dep))
					os.Exit(ExitState)
					return nil
				}
			}
			now := time.Now().UTC()
			task.Status = model.StatusWIP
			task.StartedAt = &now
			task.DurationSource = model.DurationSourceImplicit
		}

		if task.Status == model.StatusDone {
			output.Error(ExitState, fmt.Sprintf("task %s is already done", task.ID))
			os.Exit(ExitState)
			return nil
		}

		// Resolve reason
		reason := ""
		if len(args) > 1 {
			reason = args[1]
		}

		// Stage implementation files (selected via --files or auto-detected).
		if err := gitStage(commitFiles); err != nil {
			output.Error(ExitFile, fmt.Sprintf("git stage failed: %v", err))
			os.Exit(ExitFile)
			return nil
		}

		// Best-effort: drop any accidentally-staged flock files from the index.
		// `git rm --cached --ignore-unmatch` is HEAD-independent (works before the
		// first commit) and never errors on a no-match, so ignoring its result
		// cannot leave a lock file staged for the commit.
		_ = runGit("rm", "--cached", "--ignore-unmatch", "-q", "--", "*.lock", "*.tasks.json.lock")

		// Check if there's anything to commit
		if !gitHasStagedChanges() {
			output.Error(ExitState, "no changes to commit")
			os.Exit(ExitState)
			return nil
		}

		// §5.1a: write the task file in its pre-close (wip) state and stage it
		// together with the implementation files, so the implementation commit C1
		// carries it. The closure is folded in afterward via amend.
		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}
		if err := runGit("add", "--", taskFilePath); err != nil {
			output.Error(ExitFile, fmt.Sprintf("stage task file failed: %v", err))
			os.Exit(ExitFile)
			return nil
		}

		// Build commit message
		msg := buildCommitMessage(task, reason)

		// Commit → C1, the pre-amend implementation sha (§5.1c).
		sha, err := gitCommit(msg)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("git commit failed: %v", err))
			os.Exit(ExitFile)
			return nil
		}

		// Record the pre-amend sha C1 on the task — never the post-amend object
		// sha, since a commit cannot contain its own sha (§5.1c).
		task.SetCommitSHAs([]string{sha})
		task.SetCommitFiles(resolveCommitFiles([]string{sha}))

		// Write the closure record (commit_sha = C1) into the task file.
		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		// §5.1b-d: fold the closure into C1 via amend (guarded), or a follow-up
		// commit on guard failure, leaving the working tree clean for tp-owned
		// paths. Best-effort: the closure record is already on disk.
		if foldErr := foldClosureCommit(".", task.ID, sha, []string{taskFilePath}); foldErr != nil {
			fmt.Fprintf(os.Stderr, "warning: could not fold closure into %s: %v\n", sha, foldErr)
		}

		return output.JSON(map[string]any{
			"id":      task.ID,
			"sha":     sha,
			"message": strings.SplitN(msg, "\n", 2)[0],
		})
	})
}

func buildCommitMessage(task *model.Task, reason string) string {
	// First line: conventional commit with task ID
	title := fmt.Sprintf("feat(%s): %s", task.ID, task.Title)

	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n")

	if reason != "" {
		body.WriteString("\n")
		body.WriteString(reason)
		body.WriteString("\n")
	}

	// Trailer
	body.WriteString("\n")
	fmt.Fprintf(&body, "Task: %s\n", task.ID)
	if task.Acceptance != "" {
		fmt.Fprintf(&body, "Acceptance: %s\n", task.Acceptance)
	}

	return body.String()
}

func gitStage(filesFlag string) error {
	if filesFlag == "" {
		return runGit("add", "-A")
	}

	for _, pattern := range strings.Split(filesFlag, ",") {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if strings.HasSuffix(pattern, ".tasks.json.lock") {
			return fmt.Errorf("refusing to stage lock file %q: lock files live under .tp/locks/ and are never committed", pattern)
		}
		if err := runGit("add", pattern); err != nil {
			return fmt.Errorf("staging %q: %w", pattern, err)
		}
	}
	return nil
}

func gitHasStagedChanges() bool {
	cmd := exec.Command("git", "diff", "--cached", "--quiet")
	return cmd.Run() != nil // exit 1 = has changes
}

func gitCommit(message string) (string, error) {
	if err := runGit("commit", "-m", message); err != nil {
		return "", err
	}

	// Get the SHA
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("get commit SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func runGit(args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// foldClosureCommit amends the implementation commit (implSHA = C1) to fold the
// closed task file into it, producing C2 whose tree carries the closure (§5.1b).
// Both guards must hold, else it falls back to a follow-up commit (§5.1d).
// writtenPaths are the paths tp wrote during this command (the task file, plus
// any .tp-review/ or .tp/ state tp wrote in the same invocation).
func foldClosureCommit(dir, taskID, implSHA string, writtenPaths []string) error {
	if canAmendClosure(dir, implSHA, writtenPaths) {
		for _, p := range writtenPaths {
			_ = runGitDir(dir, "add", "--", p)
		}
		if err := runGitDir(dir, "commit", "--amend", "--no-edit"); err == nil {
			return nil
		}
	}
	return followUpClosureCommit(dir, taskID, writtenPaths)
}

// canAmendClosure reports whether both §5.1b guards hold:
//
//	(i)  HEAD is still implSHA (no commit landed between tp's commit and amend);
//	(ii) git diff --name-only HEAD lists only paths tp itself wrote this command.
//
// A tp-owned path that was already dirty before tp ran (e.g. a hand-edited
// .tp/config.json) fails (ii) and forces the §5.1d fallback, as does any
// non-tp-owned differing path.
func canAmendClosure(dir, implSHA string, writtenPaths []string) bool {
	head, err := gitHeadShortSHA(dir)
	if err != nil || head != implSHA {
		return false
	}
	dirty, err := gitDiffNameOnlyHEAD(dir)
	if err != nil {
		return false
	}
	written := make(map[string]bool, len(writtenPaths))
	for _, p := range writtenPaths {
		written[repoRelativePath(dir, p)] = true
	}
	for _, p := range dirty {
		if !written[p] {
			return false
		}
	}
	return true
}

// followUpClosureCommit creates a separate follow-up commit chore(tp): record
// <id> closure carrying the closed task file (§5.1d), leaving C1 as commit_sha.
func followUpClosureCommit(dir, taskID string, paths []string) error {
	for _, p := range paths {
		_ = runGitDir(dir, "add", "--", p)
	}
	msg := fmt.Sprintf("chore(tp): record %s closure", taskID)
	return runGitDir(dir, "commit", "-m", msg)
}

func gitHeadShortSHA(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitDiffNameOnlyHEAD(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			paths = append(paths, line)
		}
	}
	return paths, nil
}

// repoRelativePath normalizes a cwd-relative path to repo-root-relative — the
// base git diff --name-only uses — so the §5.1b(ii) comparison is sound even
// when tp runs from a subdirectory.
func repoRelativePath(dir, p string) string {
	if n, err := engine.NormalizeKeepPath(dir, p); err == nil {
		return n
	}
	return p
}

// runGitDir runs git with args in dir, wiring stderr to the process stderr.
func runGitDir(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// resolveCommitFiles returns the union of repo-root-relative paths touched by
// the given commits (added/modified/deleted/renamed-new; renamed-old excluded),
// for SetCommitFiles to dedup, sort, and cap. It returns nil when git is
// unavailable or any sha cannot be resolved, so the field is omitted rather
// than guessed.
func resolveCommitFiles(shas []string) []string {
	if len(shas) == 0 {
		return nil
	}
	if !gitExists(".") {
		return nil
	}
	var all []string
	for _, sha := range shas {
		sha = strings.TrimSpace(sha)
		if sha == "" {
			continue
		}
		paths, ok := commitChangedPaths(".", sha)
		if !ok {
			return nil
		}
		all = append(all, paths...)
	}
	return all
}

// commitChangedPaths resolves the paths a single commit touched via
// `git show --name-status`: added/modified/deleted/renamed-new are kept and
// renamed-old is excluded. ok is false when git cannot resolve sha.
func commitChangedPaths(dir, sha string) ([]string, bool) {
	cmd := exec.Command("git", "show", "--name-status", "--pretty=format:", sha)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	paths := make([]string, 0)
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 || fields[0] == "" {
			continue
		}
		if c := fields[0][0]; c == 'R' || c == 'C' {
			if len(fields) >= 3 {
				paths = append(paths, fields[2])
			}
			continue
		}
		paths = append(paths, fields[1])
	}
	return paths, true
}
