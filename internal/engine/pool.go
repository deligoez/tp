package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// A filled pool for a hand-driven coordinator (spec/backlog/filled-pool.md):
// the session that claimed tasks through spawns is told, when its turn ends,
// how many of its run_max_parallel slots are free and which ready tasks of
// its task files could fill them.

// PoolTask is one task the pool names: its id, its task file relative to the
// repository root, and the waiting text of a parked one.
type PoolTask struct {
	ID      string `json:"id"`
	File    string `json:"file"`
	Waiting string `json:"waiting,omitempty"`
}

// Pool is `tp pool`'s answer for one session.
type Pool struct {
	Session     string `json:"session"`
	Coordinator bool   `json:"coordinator"`
	Cap         int    `json:"cap"`
	// Running are wip claims whose spawn is a live agent; Parked are wip
	// claims with waiting set in some worktree's copy. Both hold a slot.
	Running []PoolTask `json:"running"`
	Parked  []PoolTask `json:"parked"`
	// Unresolved counts live agents whose spawn could not be read. Each
	// holds a slot, and while any is left no claim is called an orphan.
	Unresolved int `json:"unresolved"`
	// Orphans are wip claims with no live agent and no waiting; Respawn are
	// claims freed with --takeover and not yet bound to a new spawn. Neither
	// holds a slot.
	Orphans []PoolTask `json:"orphans"`
	Respawn []PoolTask `json:"respawn"`
	// Unmerged are claims closed in another worktree's copy and not yet in
	// this tree; they hold no slot.
	Unmerged []PoolTask `json:"unmerged"`
	Free     int        `json:"free"`
	Ready    []PoolTask `json:"ready"`
	Ask      bool       `json:"ask"`
	Reason   string     `json:"reason,omitempty"`
}

// PoolInput is what the pool is computed from: the tree the coordinator works
// in, its session, the spawn ids of its live agents, and how many live agents
// had no readable spawn.
type PoolInput struct {
	Start      string
	Session    string
	Live       []string
	Unresolved int
}

// otherCopies is what the repository's other worktrees say about one task.
type otherCopies struct{ done, waiting bool }

// PoolOf computes the pool of in.Session in the repository holding in.Start.
func PoolOf(in PoolInput) Pool {
	p := Pool{Session: in.Session, Unresolved: in.Unresolved,
		Running: []PoolTask{}, Parked: []PoolTask{}, Orphans: []PoolTask{}, Respawn: []PoolTask{},
		Unmerged: []PoolTask{}, Ready: []PoolTask{}}
	home := gitToplevel(in.Start)
	if home == "" || !ValidClaimToken(in.Session) {
		return p
	}
	live := map[string]bool{}
	for _, s := range in.Live {
		live[s] = true
	}
	others := otherWorktreeCopies(home)
	for _, path := range repoTaskFiles(home) {
		p.addFile(home, path, live, others)
	}
	if p.Unresolved > 0 {
		p.Orphans = []PoolTask{}
	}
	p.Free = max(p.Cap-len(p.Running)-len(p.Parked)-p.Unresolved, 0)
	// A session that never claimed has cap 0, so no free slot to ask about.
	p.Ask = p.Free > 0 && len(p.Ready)+len(p.Orphans)+len(p.Respawn) > 0
	if p.Ask {
		p.Reason = poolReason(&p)
	}
	return p
}

// addFile folds one of home's task files into p when the session claimed a
// task in it: its claims are classified, its ready tasks listed unless an
// epic blocks the file, and its run_max_parallel bounds the cap.
func (p *Pool) addFile(home, path string, live map[string]bool, others map[claimKey]otherCopies) {
	tf, err := model.ReadTaskFile(path)
	if err != nil {
		return
	}
	rel := repoRelativeTo(home, path)
	claimed := false
	for i := range tf.Tasks {
		task := &tf.Tasks[i]
		if task.ClaimedBy == nil || task.ClaimedBy.Session != p.Session {
			continue
		}
		claimed = true
		if task.Status == model.StatusWIP {
			p.classify(task, rel, live, others[claimKey{file: rel, id: task.ID}])
		}
	}
	if !claimed {
		return
	}
	limit := implementMaxParallel(path, tf)
	if !p.Coordinator || limit < p.Cap {
		p.Cap = limit
	}
	p.Coordinator = true
	if block, blockErr := EpicBlockFor(tf, path); blockErr != nil || (block != nil && block.Blocks()) {
		return
	}
	for _, t := range ReadyTasks(tf) {
		p.Ready = append(p.Ready, PoolTask{ID: t.ID, File: rel})
	}
}

// classify puts one wip task the session claimed in its class. A close in
// another worktree wins, then a live spawn, then waiting in any copy.
func (p *Pool) classify(task *model.Task, rel string, live map[string]bool, other otherCopies) {
	pt := PoolTask{ID: task.ID, File: rel, Waiting: task.Waiting}
	switch {
	case other.done:
		p.Unmerged = append(p.Unmerged, pt)
	case task.ClaimedBy.Spawn != "" && live[task.ClaimedBy.Spawn]:
		p.Running = append(p.Running, pt)
	case task.Waiting != "" || other.waiting:
		p.Parked = append(p.Parked, pt)
	case task.ClaimedBy.Spawn == "":
		p.Respawn = append(p.Respawn, pt)
	default:
		p.Orphans = append(p.Orphans, pt)
	}
}

// otherWorktreeCopies reads every worktree but home: which tasks are done in
// some copy, and which carry waiting in some copy. A hold there may be a
// worktree agent parking its claim on a copy cut before the claim, so the
// pool reads it as parked: a slot left idle costs a turn, a task given to
// two agents costs the work.
func otherWorktreeCopies(home string) map[claimKey]otherCopies {
	out := map[claimKey]otherCopies{}
	for _, tree := range worktreeRoots(home) {
		if samePath(tree, home) {
			continue
		}
		for _, path := range repoTaskFiles(tree) {
			tf, err := model.ReadTaskFile(path)
			if err != nil {
				continue
			}
			rel := repoRelativeTo(tree, path)
			for i := range tf.Tasks {
				k := claimKey{file: rel, id: tf.Tasks[i].ID}
				c := out[k]
				c.done = c.done || tf.Tasks[i].Status == model.StatusDone
				c.waiting = c.waiting || tf.Tasks[i].Waiting != ""
				out[k] = c
			}
		}
	}
	return out
}

// samePath reports whether a and b name the same directory once symlinks
// are resolved, as git and the caller may spell a worktree differently.
func samePath(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return ra == rb
}

// poolReason is the block reason the Stop hook hands the coordinator: the
// free slots, the ready tasks up to them with their brief commands, the
// claims to spawn again, and how to record a gate that keeps a task back.
func poolReason(p *Pool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d of %d parallel slots are free (run_max_parallel).", p.Free, p.Cap)
	if n := min(p.Free, len(p.Ready)); n > 0 {
		b.WriteString(" Start each of these in its own background subagent, its prompt being the brief:")
		for _, t := range p.Ready[:n] {
			fmt.Fprintf(&b, " `tp brief %s --file %s`;", t.ID, t.File)
		}
	}
	for _, t := range p.Respawn {
		fmt.Fprintf(&b, " Claim %s has no agent: spawn one with `tp brief %s --file %s`.", t.ID, t.ID, t.File)
	}
	for _, t := range p.Orphans {
		fmt.Fprintf(&b, " Claim %s lost its agent: free it with `tp claim %s --takeover --file %s`, then spawn one with its brief.", t.ID, t.ID, t.File)
	}
	b.WriteString(" If a task must not start yet for a reason that is not a task, record the reason with `tp set <id> waiting=\"<why>\"`, which holds it out of the ready set; then end the turn.")
	return b.String()
}
