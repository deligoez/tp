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
		f := &e.New[i]
		parts = append(parts, fmt.Sprintf("%s [%s] (%s)", f.Rule, strings.Join(f.Epics, ", "), f.Message))
	}
	return "write refused: it adds lint errors the file did not hold: " + strings.Join(parts, "; ")
}

// UpdateEpicGraph applies mutate to tpDir/epics.json under the file's write
// lock and writes the result, unless the after-state holds an error the
// before-state did not: then it writes nothing and returns an
// *EpicWriteRefusedError. An error from mutate also writes nothing and is
// returned as is. An absent file starts as an empty version-1 graph, so any
// writer creates it. Keys outside the schema are not carried into the
// written file, so the after-state is linted without them. It returns the
// path of the graph file.
func UpdateEpicGraph(tpDir string, mutate func(*model.EpicGraph) error) (string, error) {
	path := filepath.Join(tpDir, EpicGraphFile)
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		return path, fmt.Errorf("create %s: %w", tpDir, err)
	}
	return path, WithFileLock(path, func() error {
		g, unknown, err := LoadEpicGraph(tpDir)
		if errors.Is(err, ErrEpicGraphNotFound) {
			g, err = &model.EpicGraph{Version: 1, Milestones: [][]string{}, Epics: []model.Epic{}}, nil
		}
		if err != nil {
			return err
		}
		before := LintEpicGraph(g, unknown)
		if err := mutate(g); err != nil {
			return err
		}
		if added := NewEpicErrors(before, LintEpicGraph(g, nil)); len(added) > 0 {
			return &EpicWriteRefusedError{New: added}
		}
		return WriteEpicGraph(tpDir, g)
	})
}

// AddEpics appends epics to g in order. With replace, an epic whose id g
// already holds replaces that epic in place instead (a --bulk row, §3).
func AddEpics(g *model.EpicGraph, epics []model.Epic, replace bool) {
	for _, e := range epics {
		if i := epicIndex(g, e.ID); replace && i >= 0 {
			g.Epics[i] = e
			continue
		}
		g.Epics = append(g.Epics, e)
	}
}

func epicIndex(g *model.EpicGraph, id string) int {
	for i := range g.Epics {
		if g.Epics[i].ID == id {
			return i
		}
	}
	return -1
}
