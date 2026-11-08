package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// EpicImportRefusedError reports a tp epic import over a file that already
// holds epics, without --force (spec/1.3.0.md §3). Nothing was written.
type EpicImportRefusedError struct {
	Path  string
	Epics int
}

func (e *EpicImportRefusedError) Error() string {
	return fmt.Sprintf("%s already holds %d epics; tp epic import replaces it only with --force", e.Path, e.Epics)
}

// ImportEpicGraph replaces tpDir/epics.json with g under the file's write
// lock. A file that holds epics is refused with an *EpicImportRefusedError
// unless force; an absent file is created. The payload's own lint errors do
// not refuse the write (D4): the after-state's findings are returned for the
// caller to report. It returns the path of the graph file.
func ImportEpicGraph(tpDir string, g *model.EpicGraph, force bool) (string, []EpicLintFinding, error) {
	path := filepath.Join(tpDir, EpicGraphFile)
	if err := os.MkdirAll(tpDir, 0o755); err != nil {
		return path, nil, fmt.Errorf("create %s: %w", tpDir, err)
	}
	var findings []EpicLintFinding
	err := WithFileLock(path, func() error {
		if !force {
			current, _, err := LoadEpicGraph(tpDir)
			if err != nil && !errors.Is(err, ErrEpicGraphNotFound) {
				return err
			}
			if err == nil && len(current.Epics) > 0 {
				return &EpicImportRefusedError{Path: path, Epics: len(current.Epics)}
			}
		}
		if err := WriteEpicGraph(tpDir, g); err != nil {
			return err
		}
		findings = LintEpicGraph(g, nil)
		return nil
	})
	return path, findings, err
}
