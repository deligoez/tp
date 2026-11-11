package engine

import "github.com/deligoez/tp/internal/model"

// EpicLayout is one epic's place on the map (spec/1.8.0.md §9): the index of
// its milestone's lane in EpicMapLanes' order, that lane's kind, its
// milestone's column and its row inside the milestone.
type EpicLayout struct {
	Lane     int    `json:"lane"`
	LaneKind string `json:"lane_kind"`
	Column   int    `json:"column"`
	Row      int    `json:"row"`
}

// EpicMapLayout places every epic of a tracked milestone on the map, from
// EpicMapLanes and EpicMapRows; an epic in a milestone no track lists has no
// place and no entry.
func EpicMapLayout(g *model.EpicGraph) map[string]EpicLayout {
	lanes := EpicMapLanes(g)
	out := map[string]EpicLayout{}
	rows := EpicMapRows(g, lanes)
	for i, lane := range lanes.Lanes {
		for _, m := range lane.Milestones {
			for row, id := range rows[m] {
				out[id] = EpicLayout{Lane: i, LaneKind: lane.Kind, Column: lanes.Columns[m], Row: row}
			}
		}
	}
	return out
}
