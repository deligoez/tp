package engine

// EpicSliceEdges is one epic's edges in one slice (spec/1.8.0.md §11): the
// slice's state directory and key, and each recorded round whose snapshot
// holds the epic.
type EpicSliceEdges struct {
	StateDir string              `json:"state_dir"`
	Key      *EpicSliceKeyRecord `json:"key"`
	Rounds   []EpicRoundEdges    `json:"rounds"`
}

// EpicRoundEdges is the epic's edges as round's snapshot holds them, in block
// order. HashesOnly is true when the snapshot was written before blocks were
// kept: then its edges are unknown and Edges is empty.
type EpicRoundEdges struct {
	Round      int        `json:"round"`
	Edges      []EpicEdge `json:"edges"`
	HashesOnly bool       `json:"hashes_only"`
}

// EpicEdge is one dependency of an epic and its why, To naming it as the
// block does, a group as one of {A, B}.
type EpicEdge struct {
	To  string `json:"to"`
	Why string `json:"why"`
}

// EpicEdgeHistory reads epic id's edges and whys per recorded round of every
// slice in dirs whose snapshot holds it, in dirs' order. A slice none of whose
// recorded rounds holds it is left out, so an epic in no recorded slice has
// none.
func EpicEdgeHistory(dirs []EpicSliceDir, id string) ([]EpicSliceEdges, error) {
	out := []EpicSliceEdges{}
	for _, dir := range dirs {
		rounds, err := epicRoundEdges(dir.Path, "epic:"+id)
		if err != nil {
			return nil, err
		}
		if len(rounds) > 0 {
			out = append(out, EpicSliceEdges{StateDir: dir.Path, Key: dir.Key, Rounds: rounds})
		}
	}
	return out, nil
}

// epicRoundEdges reads unit key's edges from each recorded round's snapshot
// in stateDir that holds it.
func epicRoundEdges(stateDir, key string) ([]EpicRoundEdges, error) {
	rounds := []EpicRoundEdges{}
	next := EpicReviewNextRound(stateDir)
	for r := 1; r < next; r++ {
		snap, err := readEpicSnapshotFile(stateDir, r)
		if err != nil {
			return nil, err
		}
		if _, held := snap.Units[key]; !held {
			continue
		}
		round := EpicRoundEdges{Round: r, Edges: []EpicEdge{}}
		block, kept := snap.Blocks[key]
		round.HashesOnly = !kept
		p := parseEpicBlock(block)
		for _, to := range p.edgeOrder {
			round.Edges = append(round.Edges, EpicEdge{To: to, Why: p.edges[to]})
		}
		rounds = append(rounds, round)
	}
	return rounds, nil
}
