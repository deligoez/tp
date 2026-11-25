package cli

import (
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// nowEpics is each epic of the project's graph in graph order, with the done
// and total tasks of its bound files and whether tp epic status derives it
// done; nil when the project has no epic graph. A graph or bound file that
// cannot be read is said on stderr and reported as nil, so the rest of the
// payload still reaches the pane.
func nowEpics() []nowEpic {
	tpDir := engine.DiscoverTPDir(".")
	if tpDir == "" {
		return nil
	}
	if _, err := os.Stat(filepath.Join(tpDir, engine.EpicGraphFile)); err != nil {
		return nil
	}
	g, states, bound, rerr := readEpicStates()
	if rerr != nil {
		output.Notice("warning: the epic graph cannot be read: " + rerr.msg)
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
			tf, err := model.ReadTaskFile(path)
			if err != nil {
				output.Notice("warning: a bound task file cannot be read: " + err.Error())
				return nil
			}
			c := countTasks(tf)
			row.Done += c.Done
			row.Total += c.Total
		}
		out = append(out, row)
	}
	return out
}
