package cli

import (
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

// foundAnswerHint names the answers and what each carries, so a refused
// decision unit can rebuild the call without reading the help.
const foundAnswerHint = "answer one undecided request: task (--expected, --observed, a blank Fix field falls back to the request's, plus --cite and --quote), conforms (--reproduction that passes, --cite and --quote), or human; when in doubt, human"

// foundQuoteHint says what the quote check accepts.
const foundQuoteHint = "--cite names the spec, a CLAUDE.md, CONVENTIONS.md or REFERENCE.md inside the project, and --quote is copied verbatim from it (runs of whitespace may differ); a choice no such quote settles is a human answer"

// foundAnswerFlags are tp found answer's inputs.
type foundAnswerFlags struct {
	location, reproduction, expected, observed, cite, quote string
}

// errFoundAnswerRefused marks a plan error that is the caller's to fix, not
// the store's: tp found answer exits 2 on it and writes nothing.
var errFoundAnswerRefused = errors.New("refused")

func newFoundAnswerCmd() *cobra.Command {
	var f foundAnswerFlags
	cmd := &cobra.Command{
		Use:   "answer <request> <task|conforms|human>",
		Short: "Record the decision unit's answer to an undecided request",
		Long: `Record one answer to an undecided request; the request leaves the undecided state.

task: the Fix fields (a blank one falls back to the request's), a
reproduction the unit ran and saw fail, and a quote tp finds verbatim, runs
of whitespace collapsed, in the cited file: the spec, a CLAUDE.md,
CONVENTIONS.md or REFERENCE.md. It is applied like a Fix-shaped request,
attached, deduplicated, capped by found_max and placed, but never as a
duplicate of the request it answers; past the cap it becomes an open queue
entry carrying the quote.

conforms: the reproduction of the expected behaviour passes, with a quote
checked the same way. No task opens; the queue keeps it as a decided entry.

human: everything else. It becomes an open queue entry with the request's
question and alternatives.

A quote the cited file does not hold exits 2 and the request stays undecided.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			return runFoundAnswer(args[0], args[1], &f)
		},
	}
	cmd.Flags().StringVar(&f.location, "location", "", "task: where the defect is, as file:line (default the request's)")
	cmd.Flags().StringVar(&f.reproduction, "reproduction", "", "task: a command that fails; conforms: one that passes (default the request's)")
	cmd.Flags().StringVar(&f.expected, "expected", "", "task: the expected behaviour")
	cmd.Flags().StringVar(&f.observed, "observed", "", "task: the observed behaviour")
	cmd.Flags().StringVar(&f.cite, "cite", "", "task, conforms: the file that settles the choice, relative to the project root")
	cmd.Flags().StringVar(&f.quote, "quote", "", "task, conforms: the sentence copied verbatim from --cite")
	return cmd
}

// runFoundAnswer implements tp found answer (spec/1.31.0.md §5).
func runFoundAnswer(id, kind string, f *foundAnswerFlags) error {
	if kind != engine.FoundAnswerTask && kind != engine.FoundAnswerConforms && kind != engine.FoundAnswerHuman {
		return foundRefusal(fmt.Sprintf("unknown answer %q: want task, conforms or human", kind), foundAnswerHint)
	}
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		// task-file-hint: no task file resolved, so there is no request to answer.
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}
	root := engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
	ans := &engine.FoundAnswer{
		Kind: kind, Location: f.location, Reproduction: f.reproduction,
		Expected: f.expected, Observed: f.observed, Quote: f.quote,
	}
	if kind != engine.FoundAnswerHuman {
		cited, msg := checkFoundQuote(root, taskFilePath, f.cite, f.quote)
		if msg != "" {
			return foundRefusal(msg, foundQuoteHint)
		}
		ans.Cited = cited
	}
	foundMax := engine.EffectiveWorkflowForTaskFile(taskFilePath).FoundMax
	plan, err := answerFoundRequest(root, taskFilePath, foundMax, id, ans)
	if errors.Is(err, errFoundAnswerRefused) {
		return foundRefusal(strings.TrimSuffix(err.Error(), ": "+errFoundAnswerRefused.Error()), foundAnswerHint)
	}
	var lockErr *engine.LockTimeoutError
	if errors.As(err, &lockErr) {
		// §12.2: either lock timing out is a state error (exit 4), which the
		// root dispatcher maps with the lock's own hint.
		return err
	}
	if err != nil {
		output.Error(ExitFile, "cannot record the answer: "+err.Error(), foundStoreWriteHint)
		os.Exit(ExitFile)
		return nil
	}
	out := foundPlanOutput(&plan, foundMax)
	out["request"], out["answer"] = id, kind
	if len(plan.Entries) > 0 && kind != engine.FoundAnswerTask {
		out["entry"] = plan.Entries[0].ID
	}
	return output.JSON(out)
}

// checkFoundQuote runs the quote check of a task or conforms answer: cite
// must name a citable file inside the project and quote must be in it,
// whitespace-normalised. It returns cite relative to root, or why the answer
// is refused.
func checkFoundQuote(root, taskFilePath, cite, quote string) (cited, refusal string) {
	if strings.TrimSpace(cite) == "" || strings.TrimSpace(quote) == "" {
		return "", "a task or conforms answer needs --cite and --quote"
	}
	path := cite
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Sprintf("--cite %s is outside the project", cite)
	}
	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		return "", "cannot read the task file to find its spec: " + err.Error()
	}
	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)
	if !engine.FoundCitable(canonicalFile(path), canonicalFile(specPath)) {
		return "", fmt.Sprintf("--cite %s is not a citable file", cite)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Sprintf("cannot read --cite %s: %v", cite, err)
	}
	if !engine.FoundQuoteIn(string(data), quote) {
		return "", fmt.Sprintf("the quote is not in %s", rel)
	}
	return filepath.ToSlash(rel), ""
}

// canonicalFile is path made absolute with its symlinks resolved, so the
// spec named relative to the task file and the one --cite names compare
// equal; a path that cannot be resolved is returned absolute.
func canonicalFile(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// answerFoundRequest records an answer under the task-file lock, then the
// store's, the order tp found apply takes them in, and returns the plan with
// its queue entries numbered. A plan error is a refusal and writes nothing.
// answerFoundRequest records an answer under the task-file lock, then the
// store's, the order tp found apply takes them in, and returns the plan with
// its queue entries numbered. A plan error is a refusal and writes nothing.
// The task file and queue are written before the store, so when the store
// write fails they are put back: a task left on disk beside a request still
// undecided is one dedup cannot see, and answering again would open a second.
func answerFoundRequest(root, taskFilePath string, foundMax int, id string, ans *engine.FoundAnswer) (engine.FoundApplyPlan, error) {
	var plan engine.FoundApplyPlan
	err := engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			return err
		}
		queue, err := engine.ReadFoundQueue(taskFilePath)
		if err != nil {
			return err
		}
		before, err := snapshotFoundFiles(taskFilePath)
		if err != nil {
			return err
		}
		wrote := false
		err = engine.UpdateFoundRequests(root, taskFilePath, func(reqs []engine.FoundRequest) ([]engine.FoundRequest, error) {
			plan, err = engine.PlanFoundAnswer(reqs, tf.Tasks, queue, foundMax, id, ans)
			if err != nil {
				return nil, fmt.Errorf("%w: %w", err, errFoundAnswerRefused)
			}
			wrote = true
			return plan.Requests, writeFoundPlan(taskFilePath, tf, queue, &plan, nil)
		})
		if err != nil && wrote {
			return errors.Join(err, before.restore())
		}
		return err
	})
	return plan, err
}

// fileSnapshot is a file's bytes and mode as they were before an answer
// wrote it; a file that did not exist has existed false.
type fileSnapshot struct {
	path    string
	data    []byte
	mode    os.FileMode
	existed bool
}

// foundFiles are the task file and the queue an answer writes before the
// store.
type foundFiles []fileSnapshot

// snapshotFoundFiles reads the task file and the queue an answer may write.
// The caller holds the task-file lock.
func snapshotFoundFiles(taskFilePath string) (foundFiles, error) {
	var files foundFiles
	for _, path := range []string{taskFilePath, engine.FoundQueuePath(taskFilePath)} {
		snap, err := snapshotFile(path)
		if err != nil {
			return nil, err
		}
		files = append(files, snap)
	}
	return files, nil
}

func snapshotFile(path string) (fileSnapshot, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{path: path}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fileSnapshot{}, err
	}
	return fileSnapshot{path: path, data: data, mode: info.Mode().Perm(), existed: true}, nil
}

// restore puts the snapshot back, each file by rename so a lock-free reader
// never sees a partial document, and removes a file the answer created.
func (files foundFiles) restore() error {
	var errs []error
	for i := range files {
		f := &files[i]
		if !f.existed {
			if err := os.Remove(f.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				errs = append(errs, err)
			}
			continue
		}
		if err := replaceFileByRename(f.path, f.data, f.mode); err != nil {
			errs = append(errs, err)
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("put back the task file and queue: %w", err)
	}
	return nil
}

// replaceFileByRename writes data to a temporary file beside path and renames
// it over path with mode.
func replaceFileByRename(path string, data []byte, mode os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
