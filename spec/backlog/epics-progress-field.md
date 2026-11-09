# Epic progress — field findings for v1.6.0

> Decision candidate. The first pilot batch of v1.5.0 on a large graph (hundreds of epics, several
> tracks), in shape only. One release answers every batch; more batches append here.

| # | finding | proposed answer |
|---|---|---|
| F1 | `tp epic ready` lists equally-ready epics with no ranking signal, so "what next" is computed by hand | each ready item carries derived `downstream` counts (longest chain, dependents, dependents in open and next milestones); `tp epic status` adds a cross-milestone critical path |
| F2 | one track forces sequential opening, but some milestones of a track are siblings, and one milestone may open after an earlier one in parallel with its neighbour | per-milestone `opens_after: [<milestone>...]`, defaulting to the previous milestone of its track; a decision, since it changes openness |
| F3 | a condition is listed as waiting on the operator from day zero, though its dependents have other undone dependencies | list a condition under `operator` only when it is the last undone dependency of some dependent, the rest under `--all`; carry `actionable` |
| F4 | `no-dependencies` warns on every condition outside a track's first milestone | exempt `condition: true` |
| F5 | a threshold has no "number still to write" state | declined: tp does not judge threshold text, and `--met` is the operator's evidenced decision |
| F6 | a slice recorded before v1.5.0 exports with `key: null` | derive the key from the recorded round on the first read |
| F7 | `status --compact` and `export` are large on a big graph, mostly the blocked lists of closed milestones | `tp epic status --open`, or closed milestones' blocked lists summarised |
| F8 | `tp resume` at the repository root does not find a bound file in a subdirectory | `tp resume/next/ready --epic <id>`, resolving the bound file |
| F9 | the export's `epics[]` carries only `{id, status, done_at, files}`; a drawing joins against the graph | repeat `milestone` and `condition`, and carry F1's counts, F3's `actionable` and what holds a deferrable epic |

## Batch 2, on the released v1.5.0

| # | finding | proposed answer |
|---|---|---|
| B1 | editing one gate's threshold means reading the raw graph, stripping every gate's `met`, and replacing the whole `gates` object | `tp epic set gates.<m>.threshold=<text>` and `gates.<m>.deferrable=<ids>`, carrying `met` like the other whole-object writers |
| B2 | `share_done` and `throughput` count a condition as delivered work | leave conditions out of both, as they are out of gate membership; report done conditions apart |
| B3 | the roll-up's estimate covers closed tasks only, so an in-flight epic's remaining work is invisible | add `estimate_total` over every task of the bound files beside the closed-task figures |
