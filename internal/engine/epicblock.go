package engine

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicBlock is what a bound task file's epic does to starting its tasks
// (spec/1.5.0.md §6). BlockedBy names the epic's plain dependencies that are
// not done, in depends_on order; BlockedByAnyOf holds each unsatisfied any-of
// group's members, one list per group in depends_on order (spec/1.7.0.md
// §7). While either is not empty (Blocks) no task of the file may start.
// Notice is set when the binding is ignored because the graph does not hold
// the epic or there is no graph file: the file then behaves as unbound, and
// the caller says so.
type EpicBlock struct {
	Epic           string
	BlockedBy      []string
	BlockedByAnyOf [][]string
	Notice         string
	// Conditions are the entries of BlockedBy that are conditions.
	Conditions []string
	// AnyOfConditions holds, for each group of BlockedByAnyOf, its members
	// the operator can record now: conditions whose milestone is open and
	// whose own dependencies are done, as actionability counts a group as a
	// condition (spec/1.7.0.md §7).
	AnyOfConditions [][]string
}

// Blocks reports whether b holds the file's tasks: a plain dependency or a
// group is not satisfied.
func (b *EpicBlock) Blocks() bool {
	return len(b.BlockedBy) > 0 || len(b.BlockedByAnyOf) > 0
}

// Describe names what blocks b, as a message says it: each plain blocker by
// id, then each group as one of {A, B} (epicDepLabel's wording).
func (b *EpicBlock) Describe() string {
	parts := slices.Clone(b.BlockedBy)
	for _, members := range b.BlockedByAnyOf {
		parts = append(parts, "one of {"+strings.Join(members, ", ")+"}")
	}
	return strings.Join(parts, ", ")
}

// EpicOperatorAction is what clears a block whose every blocker is a
// condition: no agent finishes one, so it names the operator's decision
// (the escalation tp run records) and the record of each condition.
// Choices holds one list per group, its records as alternatives: any one
// satisfies the group (spec/1.7.0.md decision 3).
type EpicOperatorAction struct {
	Decision string     `json:"decision"`
	Commands []string   `json:"commands"`
	Choices  [][]string `json:"choices,omitempty"`
}

// OperatorAction returns the operator's action when every blocker of b is a
// condition and every group has a member the operator can record now
// (AnyOfConditions), nil when b blocks nothing or some blocker needs work.
func (b *EpicBlock) OperatorAction() *EpicOperatorAction {
	if !b.Blocks() || len(b.Conditions) != len(b.BlockedBy) || len(b.AnyOfConditions) != len(b.BlockedByAnyOf) {
		return nil
	}
	op := &EpicOperatorAction{Decision: EscalateEpicDone, Commands: epicDoneCommands(b.Conditions)}
	for _, ids := range b.AnyOfConditions {
		if len(ids) == 0 {
			return nil
		}
		op.Choices = append(op.Choices, epicDoneCommands(ids))
	}
	return op
}

// Steps words op for a message: every command, then each group's choices
// joined by "or", all separated by "; ".
func (op *EpicOperatorAction) Steps() string {
	steps := slices.Clone(op.Commands)
	for _, alternatives := range op.Choices {
		steps = append(steps, strings.Join(alternatives, " or "))
	}
	return strings.Join(steps, "; ")
}

// epicDoneCommands returns the operator's record of each condition of ids.
func epicDoneCommands(ids []string) []string {
	commands := make([]string, 0, len(ids))
	for _, id := range ids {
		commands = append(commands, "tp epic done "+id+" --evidence <evidence>")
	}
	return commands
}

// EpicBlockFor returns the epic block of tf, read from taskFilePath. An
// unbound file has none: it returns nil and reads nothing, so it behaves as it
// did at v1.4.2. A bound file reads the graph of the .tp/ directory above it;
// a dependency's done record counts as done whatever its own dependencies
// are, and a dependency without one is derived from its bound files; a group
// is satisfied once one member is done by the same rule. A graph or bound
// file that cannot be read is an error.
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
	if err := block.judge(g, &g.Epics[i], &boundIndex{dir: dir}); err != nil {
		return nil, err
	}
	return block, nil
}

// judge fills b from e's depends_on entries: each plain dependency that is
// not done, and each group none of whose members is.
func (b *EpicBlock) judge(g *model.EpicGraph, e *model.Epic, bound *boundIndex) error {
	for _, d := range e.DependsOn {
		if d.IsGroup() {
			satisfied, err := epicGroupDone(g, d, bound)
			if err != nil {
				return err
			}
			if !satisfied {
				b.BlockedByAnyOf = append(b.BlockedByAnyOf, d.Members())
			}
			continue
		}
		done, err := epicDepDone(g, d.ID, bound)
		if err != nil {
			return err
		}
		if !done {
			b.BlockedBy = append(b.BlockedBy, d.ID)
			if j := epicIndex(g, d.ID); j >= 0 && g.Epics[j].Condition {
				b.Conditions = append(b.Conditions, d.ID)
			}
		}
	}
	return b.recordableGroups(g, bound)
}

// recordableGroups fills b.AnyOfConditions, one list per unsatisfied group:
// its condition members whose milestone is open and whose own dependencies
// are done, judged over every epic's derived state. It reads nothing more
// when no group blocks.
func (b *EpicBlock) recordableGroups(g *model.EpicGraph, bound *boundIndex) error {
	if len(b.BlockedByAnyOf) == 0 {
		return nil
	}
	index, err := bound.all()
	if err != nil {
		return err
	}
	states, err := DeriveEpicStates(g, index)
	if err != nil {
		return err
	}
	gates, byID := MilestoneGates(g, states), epicsByID(g)
	for _, members := range b.BlockedByAnyOf {
		ids := []string{}
		for _, id := range members {
			if m, ok := byID[id]; ok && m.Condition && gates[m.Milestone].Open && epicDepsDone(m, states) {
				ids = append(ids, id)
			}
		}
		b.AnyOfConditions = append(b.AnyOfConditions, ids)
	}
	return nil
}

// epicGroupDone reports whether one member of the group d is done
// (epicDepDone).
func epicGroupDone(g *model.EpicGraph, d model.EpicDep, bound *boundIndex) (bool, error) {
	for _, m := range d.Members() {
		done, err := epicDepDone(g, m, bound)
		if err != nil || done {
			return done, err
		}
	}
	return false, nil
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
	index, err := b.all()
	if err != nil {
		return nil, err
	}
	return index[id], nil
}

// all returns the whole index, walking it on first need.
func (b *boundIndex) all() (map[string][]string, error) {
	if b.index == nil {
		index, err := EpicBoundFiles(b.dir)
		if err != nil {
			return nil, err
		}
		b.index = index
	}
	return b.index, nil
}

// unboundNotice says a file bound to epic behaves as unbound, and why.
func unboundNotice(epic, why string) string {
	return fmt.Sprintf("task file is bound to epic %q but %s; it behaves as unbound", epic, why)
}
