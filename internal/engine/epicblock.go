package engine

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// EpicBlock is what a bound task file's epic does to starting its tasks
// (spec/1.5.0.md §6). BlockedBy names the epic's direct dependencies that are
// not done, in depends_on order; while it is not empty no task of the file
// may start. Notice is set when the binding is ignored because the graph does
// not hold the epic or there is no graph file: the file then behaves as
// unbound, and the caller says so.
type EpicBlock struct {
	Epic      string
	BlockedBy []string
	Notice    string
	// Conditions are the entries of BlockedBy that are conditions.
	Conditions []string
}

// EpicOperatorAction is what clears a block whose every blocker is a
// condition: no agent finishes one, so it names the operator's decision
// (the escalation tp run records) and the record of each condition.
type EpicOperatorAction struct {
	Decision string   `json:"decision"`
	Commands []string `json:"commands"`
}

// OperatorAction returns the operator's action when every blocker of b is a
// condition, nil when b blocks nothing or some blocker is work.
func (b *EpicBlock) OperatorAction() *EpicOperatorAction {
	if len(b.BlockedBy) == 0 || len(b.Conditions) != len(b.BlockedBy) {
		return nil
	}
	commands := make([]string, 0, len(b.Conditions))
	for _, id := range b.Conditions {
		commands = append(commands, "tp epic done "+id+" --evidence <evidence>")
	}
	return &EpicOperatorAction{Decision: EscalateEpicDone, Commands: commands}
}

// EpicBlockFor returns the epic block of tf, read from taskFilePath. An
// unbound file has none: it returns nil and reads nothing, so it behaves as it
// did at v1.4.2. A bound file reads the graph of the .tp/ directory above it;
// a dependency's done record counts as done whatever its own dependencies
// are, and a dependency without one is derived from its bound files. A graph
// or bound file that cannot be read is an error.
func EpicBlockFor(tf *model.TaskFile, taskFilePath string) (*EpicBlock, error) {
	if tf.Epic == "" {
		return nil, nil
	}
	block := &EpicBlock{Epic: tf.Epic, BlockedBy: []string{}}
	dir := filepath.Dir(taskFilePath)
	tpDir := DiscoverTPDir(dir)
	if tpDir == "" {
		block.Notice = unboundNotice(tf.Epic, "there is no .tp/"+EpicGraphFile)
		return block, nil
	}
	g, _, err := LoadEpicGraph(tpDir)
	if errors.Is(err, ErrEpicGraphNotFound) {
		block.Notice = unboundNotice(tf.Epic, "there is no .tp/"+EpicGraphFile)
		return block, nil
	}
	if err != nil {
		return nil, err
	}
	i := epicIndex(g, tf.Epic)
	if i < 0 {
		block.Notice = unboundNotice(tf.Epic, ".tp/"+EpicGraphFile+" does not hold it")
		return block, nil
	}
	bound := &boundIndex{dir: dir}
	for _, d := range g.Epics[i].DependsOn {
		done, err := epicDepDone(g, d.ID, bound)
		if err != nil {
			return nil, err
		}
		if !done {
			block.BlockedBy = append(block.BlockedBy, d.ID)
			if j := epicIndex(g, d.ID); j >= 0 && g.Epics[j].Condition {
				block.Conditions = append(block.Conditions, d.ID)
			}
		}
	}
	return block, nil
}

// epicDepDone reports whether dependency id of g is done: by its done record,
// else by the status its bound files derive. An id the graph does not hold is
// not done.
func epicDepDone(g *model.EpicGraph, id string, bound *boundIndex) (bool, error) {
	j := epicIndex(g, id)
	if j < 0 {
		return false, nil
	}
	dep := &g.Epics[j]
	if dep.Done != nil {
		return true, nil
	}
	files, err := bound.files(id)
	if err != nil {
		return false, err
	}
	st, err := deriveEpicState(dep, files)
	if err != nil {
		return false, err
	}
	return st.Status == EpicStatusDone, nil
}

// boundIndex is the bound-file index EpicBoundFiles walks from dir, walked on
// first need and once.
type boundIndex struct {
	dir   string
	index map[string][]string
}

func (b *boundIndex) files(id string) ([]string, error) {
	if b.index == nil {
		index, err := EpicBoundFiles(b.dir)
		if err != nil {
			return nil, err
		}
		b.index = index
	}
	return b.index[id], nil
}

// unboundNotice says a file bound to epic behaves as unbound, and why.
func unboundNotice(epic, why string) string {
	return fmt.Sprintf("task file is bound to epic %q but %s; it behaves as unbound", epic, why)
}
