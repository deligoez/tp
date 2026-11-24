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
	// Landing are claims whose worktree agent asked for its close; tp land
	// picks the agent's commits onto this tree's branch and closes them
	// (spec/backlog/worktree-land.md). One holds a slot only while its
	// agent still runs, and while any waits the coordinator is asked to
	// land them, slot or no slot.
	Landing []PoolTask `json:"landing"`
	Free    int        `json:"free"`
	Ready   []PoolTask `json:"ready"`
	Ask     bool       `json:"ask"`
	Reason  string     `json:"reason,omitempty"`
	// common is the repository's git common directory, where land
	// requests live; home is the coordinator's worktree root.
	common, home string
	// landingLive counts the landing claims whose agent still runs.
	landingLive int
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

// otherCopies is what the repository's other worktrees say about one task:
// whether any copy has it done, and the spawns whose claim a copy carries done
// or waiting. A copy speaks for a claim only when it carries that claim's
// spawn; one cut before the claim says nothing of it.
type otherCopies struct {
	done             bool
	doneBy, parkedBy map[string]bool
}

// PoolOf computes the pool of in.Session in the repository holding in.Start.
func PoolOf(in PoolInput) Pool {
	p := Pool{Session: in.Session, Unresolved: in.Unresolved,
		Running: []PoolTask{}, Parked: []PoolTask{}, Orphans: []PoolTask{}, Respawn: []PoolTask{},
		Unmerged: []PoolTask{}, Landing: []PoolTask{}, Ready: []PoolTask{}}
	home := gitToplevel(in.Start)
	if home == "" || !ValidClaimToken(in.Session) {
		return p
	}
	p.common, p.home = gitCommonDir(home), home
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
	p.Free = max(p.Cap-len(p.Running)-len(p.Parked)-p.landingLive-p.Unresolved, 0)
	// A session that never claimed has cap 0, so no free slot to ask about.
	p.Ask = len(p.Landing) > 0 || (p.Free > 0 && len(p.Ready)+len(p.Orphans)+len(p.Respawn) > 0)
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
	claimed, busy := p.addClaims(tf, rel, live, others)
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
	// tp run's own pick: touches disjoint from every wip task's, and a solo
	// task only once nothing runs, so the hook never asks for what the driver
	// would refuse. Its bound is widened past the wip tasks, since an orphan
	// holds no slot; the free count bounds the ask. A task another worktree
	// closed is not offered again.
	for _, id := range ImplementUnits(tf, limit+len(busy), busy) {
		if !others[claimKey{file: rel, id: id}].done {
			p.Ready = append(p.Ready, PoolTask{ID: id, File: rel})
		}
	}
}

// addClaims classifies the session's claims in tf, the file at rel, and
// returns whether the session coordinates the file and which tasks are wip.
// A task a failed landing reopened lost its claim but is still this session's
// to refill, so it keeps the session the file's coordinator.
func (p *Pool) addClaims(tf *model.TaskFile, rel string, live map[string]bool, others map[claimKey]otherCopies) (claimed bool, busy []string) {
	reopened := map[string]bool{}
	if p.common != "" {
		reopened = ReopenedFor(LandDirFor(p.common, rel), p.Session)
	}
	for i := range tf.Tasks {
		task := &tf.Tasks[i]
		if task.Status == model.StatusWIP {
			busy = append(busy, task.ID)
		}
		if task.Status == model.StatusOpen && reopened[task.ID] {
			claimed = true
		}
		if task.ClaimedBy == nil || task.ClaimedBy.Session != p.Session {
			continue
		}
		claimed = true
		switch {
		case task.Status == model.StatusWIP:
			p.classify(task, rel, live, others[claimKey{file: rel, id: task.ID}])
		case task.Status == model.StatusDone && live[task.ClaimedBy.Spawn]:
			// Closed, and its agent is still finishing: the slot is not free yet.
			p.Running = append(p.Running, PoolTask{ID: task.ID, File: rel})
		}
	}
	return claimed, busy
}

// classify puts one wip task the session claimed in its class: a land
// request first, then a live spawn, then a close or a park another
// worktree's copy made for this same
// spawn, then waiting here.
func (p *Pool) classify(task *model.Task, rel string, live map[string]bool, other otherCopies) {
	pt := PoolTask{ID: task.ID, File: rel, Waiting: task.Waiting}
	spawn := task.ClaimedBy.Spawn
	switch {
	case p.common != "" && HasLandRequest(LandDirFor(p.common, rel), task.ID):
		p.Landing = append(p.Landing, pt)
		// Its agent asked to land and was told to stop; until it has, it
		// still runs and holds its slot.
		if spawn != "" && live[spawn] {
			p.landingLive++
		}
	case spawn != "" && live[spawn]:
		p.Running = append(p.Running, pt)
	case spawn != "" && other.doneBy[spawn]:
		p.Unmerged = append(p.Unmerged, pt)
	case task.Waiting != "" || (spawn != "" && other.parkedBy[spawn]):
		p.Parked = append(p.Parked, pt)
	case spawn == "":
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
				out[k] = out[k].with(&tf.Tasks[i])
			}
		}
	}
	return out
}

// with folds one more copy of the task into c.
func (c otherCopies) with(t *model.Task) otherCopies {
	c.done = c.done || t.Status == model.StatusDone
	if t.ClaimedBy == nil || t.ClaimedBy.Spawn == "" {
		return c
	}
	if c.doneBy == nil {
		c.doneBy, c.parkedBy = map[string]bool{}, map[string]bool{}
	}
	spawn := t.ClaimedBy.Spawn
	c.doneBy[spawn] = c.doneBy[spawn] || t.Status == model.StatusDone
	c.parkedBy[spawn] = c.parkedBy[spawn] || (t.Status == model.StatusWIP && t.Waiting != "")
	return c
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
	if len(p.Landing) > 0 {
		b.WriteString("Worktree agents finished ")
		files := map[string]bool{}
		for i, t := range p.Landing {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(t.ID)
			files[t.File] = true
		}
		b.WriteString(": land them first, which picks their commits onto this tree's branch and closes them, with")
		for _, t := range p.Landing {
			if files[t.File] {
				fmt.Fprintf(&b, " `%s`", landCommand(p.home, t.File))
				files[t.File] = false
			}
		}
		b.WriteString(".")
		if p.Free == 0 || len(p.Ready)+len(p.Orphans)+len(p.Respawn) == 0 {
			return b.String()
		}
		b.WriteString(" Then: ")
	}
	fmt.Fprintf(&b, "%d of %d parallel slots are free (run_max_parallel). Run these from the repository root.", p.Free, p.Cap)
	if n := min(p.Free, len(p.Ready)); n > 0 {
		b.WriteString(" Start each of these in its own background subagent, its prompt holding the brief's output, whose tp-task marker claims the task:")
		for i, t := range p.Ready[:n] {
			sep := ","
			if i == n-1 {
				sep = "."
			}
			fmt.Fprintf(&b, " `tp brief %s --file %s`%s", t.ID, t.File, sep)
		}
	}
	for _, t := range p.Respawn {
		fmt.Fprintf(&b, " Claim %s has no agent: spawn one with `tp brief %s --file %s`.", t.ID, t.ID, t.File)
	}
	for _, t := range p.Orphans {
		fmt.Fprintf(&b, " Claim %s lost its agent: free it with `tp claim %s --takeover --file %s`, then spawn one with its brief.", t.ID, t.ID, t.File)
	}
	b.WriteString(" If a task must not start yet for a reason that is not a task, record it with `tp set <id> waiting=\"<why>\" --file <task file>`, which holds it out of the ready set; then end the turn.")
	return b.String()
}

// landCommand is the tp land call for the task file at rel (relative to the
// worktree root home): tp land works in the project the file belongs to, so
// a project below home is entered first.
func landCommand(home, rel string) string {
	home = CanonicalPath(home)
	path := filepath.Join(home, filepath.FromSlash(rel))
	dir, err := filepath.Rel(home, CanonicalProjectRoot(filepath.Dir(path)))
	if err != nil || dir == "." || strings.HasPrefix(dir, "..") {
		return "tp land --file " + rel
	}
	file, err := filepath.Rel(filepath.Join(home, dir), path)
	if err != nil {
		return "tp land --file " + rel
	}
	// A subshell, so the coordinator's own working directory stays put.
	return fmt.Sprintf("(cd %s && tp land --file %s)", filepath.ToSlash(dir), filepath.ToSlash(file))
}
