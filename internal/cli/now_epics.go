package cli

import (
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// nowEpics is each epic of the graph of the project found from start, in
// graph order, with the done and total tasks of its bound files and whether
// tp epic status derives it done; nil when the project has no epic graph.
// start is the task file's project root, so tp now --file into another
// project reads that project's graph. A task file that cannot be read may
// bind any epic or none: it is named on stderr and skipped, and the epics are
// reported from the files that can be read. A graph that cannot be read, or a
// state that cannot be derived, is said on stderr and reported as nil, so the
// rest of the payload still reaches the pane.
func nowEpics(start string) []nowEpic {
	tpDir := engine.DiscoverTPDir(start)
	if tpDir == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(tpDir, engine.EpicGraphFile)); err != nil {
		return nil
	}
	g, _, err := engine.LoadEpicGraph(tpDir)
	if err != nil {
		output.Notice("warning: the epic graph cannot be read: " + err.Error())
		return nil
	}
	bound, files, skipped := engine.EpicBoundTaskFiles(tpDir)
	for _, s := range skipped {
		output.Notice("warning: skipping a task file that cannot be read: " + s.Error())
	}
	states, err := engine.DeriveEpicStatesOf(g, bound, files)
	if err != nil {
		output.Notice("warning: the epic states cannot be derived: " + err.Error())
		return nil
	}
	out := make([]nowEpic, 0, len(g.Epics))
	for i := range g.Epics {
		e := &g.Epics[i]
		row := nowEpic{ID: e.ID, Name: e.Title, State: "open"}
		if states[e.ID].Status == engine.EpicStatusDone {
			row.State = "done"
		}
		for _, path := range bound[e.ID] {
			c := countTasks(files[path])
			row.Done += c.Done
			row.Total += c.Total
		}
		out = append(out, row)
	}
	return out
}
