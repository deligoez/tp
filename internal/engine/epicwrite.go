package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicWriteRefusedError reports a write whose after-state holds lint errors
// the before-state did not (spec/1.3.0.md §3); nothing was written.
type EpicWriteRefusedError struct {
	New []EpicLintFinding
}

func (e *EpicWriteRefusedError) Error() string {
	parts := make([]string, 0, len(e.New))
	for i := range e.New {
		parts = append(parts, e.New[i].RefusalText())
	}
	return "write refused: it adds lint errors the file did not hold: " + strings.Join(parts, "; ")
}

// RefusalText is the finding as a refused write names it: its rule, the epics
// it names in brackets unless it names none (a milestone, gate or key
// finding), and its message.
func (f *EpicLintFinding) RefusalText() string {
	if len(f.Epics) == 0 {
		return fmt.Sprintf("%s (%s)", f.Rule, f.Message)
	}
	return fmt.Sprintf("%s [%s] (%s)", f.Rule, strings.Join(f.Epics, ", "), f.Message)
}

// EpicWriteOutcome is what a graph write leaves behind: the file's path, the
// paths of the keys outside the schema it did not carry, and the file's
// revision after the write. Ancestry is an import's ancestry_changed
// against the graph it replaced, [] when there was none.
type EpicWriteOutcome struct {
	Path     string
	Dropped  []string
	Revision string
	Ancestry []AncestryChange
}

// UpdateEpicGraph applies mutate to tpDir/epics.json under the file's write
// lock and writes the result, unless the after-state holds an error the
// before-state did not: then it writes nothing and returns an
// *EpicWriteRefusedError. With ifRevision set, a file whose revision differs
// is refused first with an *EpicStaleRevisionError. An error from mutate also
// writes nothing and is returned as is. An absent file starts as an empty
// version-1 graph, so any writer creates it. Keys outside the schema are not
// carried into the written file, so the after-state is linted without them;
// their paths are returned as dropped, an empty list when there were none.
func UpdateEpicGraph(tpDir string, ifRevision *string, mutate func(*model.EpicGraph) error) (EpicWriteOutcome, error) {
	return updateEpicGraph(tpDir, ifRevision, mutate, true)
}

// UpdateEpicRecord is UpdateEpicGraph for an operator's record, a done
// record or its removal: it writes the after-state whatever lint errors it
// holds, which tp epic lint then reports. A done --undo can re-create a
// milestone wait cycle the record had cut; refusing it would refuse the
// operator's own account of what is done.
func UpdateEpicRecord(tpDir string, ifRevision *string, mutate func(*model.EpicGraph) error) (EpicWriteOutcome, error) {
	return updateEpicGraph(tpDir, ifRevision, mutate, false)
}

// PreviewEpicGraph is UpdateEpicGraph without the write (tp epic apply
// --dry-run, spec/1.7.0.md §6): it checks ifRevision, applies mutate to the
// graph tpDir holds and refuses the same new lint errors, then writes
// nothing. Like every read it takes no lock. The outcome's revision is the
// file's current one, "" when it is absent.
func PreviewEpicGraph(tpDir string, ifRevision *string, mutate func(*model.EpicGraph) error) (EpicWriteOutcome, error) {
	out := EpicWriteOutcome{Path: filepath.Join(tpDir, EpicGraphFile), Dropped: []string{}}
	_, unknown, err := mutateEpicGraph(tpDir, ifRevision, mutate, true)
	if err != nil {
		return out, err
	}
	out.Dropped = unknownKeyPaths(unknown)
	out.Revision, err = EpicGraphRevision(tpDir)
	return out, err
}

// updateEpicGraph is the locked write both share; refuse says whether new
// lint errors refuse it.
func updateEpicGraph(tpDir string, ifRevision *string, mutate func(*model.EpicGraph) error, refuse bool) (EpicWriteOutcome, error) {
	out := EpicWriteOutcome{Path: filepath.Join(tpDir, EpicGraphFile), Dropped: []string{}}
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		return out, fmt.Errorf("create %s: %w", tpDir, err)
	}
	err := WithFileLock(out.Path, func() error {
		g, unknown, err := mutateEpicGraph(tpDir, ifRevision, mutate, refuse)
		if err != nil {
			return err
		}
		if err := WriteEpicGraph(tpDir, g); err != nil {
			return err
		}
		out.Dropped = unknownKeyPaths(unknown)
		out.Revision, err = EpicGraphRevision(tpDir)
		return err
	})
	return out, err
}

// mutateEpicGraph loads the graph tpDir holds after checking ifRevision, an
// absent file as an empty version-1 graph, and applies mutate to it; with
// refuse, an after-state holding a lint error the before-state did not is an
// *EpicWriteRefusedError. It returns the after-state and the keys outside
// the schema the file held, which the after-state is linted without.
func mutateEpicGraph(tpDir string, ifRevision *string, mutate func(*model.EpicGraph) error, refuse bool) (*model.EpicGraph, []EpicGraphUnknownKey, error) {
	if err := checkEpicRevision(tpDir, ifRevision); err != nil {
		return nil, nil, err
	}
	g, unknown, err := LoadEpicGraph(tpDir)
	if errors.Is(err, ErrEpicGraphNotFound) {
		g, err = &model.EpicGraph{Version: 1, Milestones: [][]string{}, Epics: []model.Epic{}}, nil
	}
	if err != nil {
		return nil, nil, err
	}
	before := LintEpicGraph(g, unknown)
	if err := mutate(g); err != nil {
		return nil, nil, err
	}
	if added := NewEpicErrors(before, LintEpicGraph(g, nil)); refuse && len(added) > 0 {
		return nil, nil, &EpicWriteRefusedError{New: added}
	}
	return g, unknown, nil
}

// unknownKeyPaths returns the path of each of unknown, an empty list when
// there is none.
func unknownKeyPaths(unknown []EpicGraphUnknownKey) []string {
	paths := []string{}
	for _, k := range unknown {
		paths = append(paths, k.Path)
	}
	return paths
}

// AddEpics appends epics to g in order. With replace, an epic whose id g
// already holds replaces that epic in place instead (a --bulk row, §3),
// keeping the claimed and done records the replaced epic carries (§8).
func AddEpics(g *model.EpicGraph, epics []model.Epic, replace bool) {
	for k := range epics {
		e := &epics[k]
		if i := epicIndex(g, e.ID); replace && i >= 0 {
			claimed, done := g.Epics[i].Claimed, g.Epics[i].Done
			g.Epics[i] = *e
			g.Epics[i].Claimed, g.Epics[i].Done = claimed, done
			continue
		}
		g.Epics = append(g.Epics, *e)
	}
}

// RefuseEpicRecords returns an error when e names a claimed or done record:
// those are written by tp epic claim, tp epic done and tp epic import only,
// and a writer replacing the epic carries the existing ones over (§8).
func RefuseEpicRecords(e *model.Epic) error {
	switch {
	case e.Claimed != nil:
		return fmt.Errorf("epic %q names claimed: tp epic claim writes it, and a write replacing the epic keeps the existing one", e.ID)
	case e.Done != nil:
		return fmt.Errorf("epic %q names done: tp epic done writes it, and a write replacing the epic keeps the existing one", e.ID)
	}
	return nil
}

func epicIndex(g *model.EpicGraph, id string) int {
	for i := range g.Epics {
		if g.Epics[i].ID == id {
			return i
		}
	}
	return -1
}
