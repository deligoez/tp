# tp — The plan is a graph, and a graph is a document with a loop

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `epics-measurements.md` beside it, and this file stands without them. It answers
a request received on 2026-11-08 from a project that plans in a dependency graph of epics, together
with that project's account of how its graph was built by hand (sidecar, *The request as received*,
*How the tree was built*). The project is not named, by its owner's decision, and none of its data is
in this repository; the account is quoted for its process only.

Class: **loop** — it adds a review loop over a new document type, with rounds, carry and
convergence; the spec loop's rules are reused, not changed. Track: **Full**, confirmed by the
operator on 2026-11-08, and shipped as **two releases** (§9).

## 1. The decision

**Context.** tp runs loops over the inside of one spec. A project whose plan is a dependency graph
of hundreds of nodes designs that graph somewhere else. The requesting project did, and its account
of how is the evidence this spec rests on (sidecar): nodes found in topic waves, edges written with
the node and each carrying a `why`, a structural checker for cycles, dangling ids and phase order,
and **manual critical-only review rounds over a slice — one milestone, or one node and its
neighbours — repeated until a round came back clean.** The checker caught every structural defect.
Every semantic defect was caught by a reader, and the same three recurred across rounds: a `why`
that restates the edge; a node whose definition of done names a capability none of its dependencies
provides (screen nodes not wired to the checker their own acceptance named, found rounds late); a
node placed by dependency where strategy wanted it elsewhere. Nothing about those rounds was
recorded, carried or counted: the same question was asked of the same unchanged node every round,
and "clean" was a feeling.

That is tp's own problem one level up. The loop machinery tp has — role prompts over a slice, a
recorded round, findings that are not re-reported, units carried when their text did not change,
convergence after a counted number of clean rounds, a cap the operator raises — is exactly what the
manual rounds converged to by hand. v0.1.0's *flat task list, no epics* stands untouched below the
task; what this spec adds is a document above the spec and the loop that reviews it. The operator's
framing fixes the unit: **one node of the plan is one epic, an epic owns the spec (usually one)
written for it, and the graph of epics is reviewed the way a spec is.**

**Decision.** Seven parts:

1. **One file, tp-owned: `.tp/epics.json`**, tracked, written only through `tp epic …` (§2).
2. **`tp epic review` runs the spec loop's rules over a slice of the graph** — emit, record,
   carry, converge, cap (§3).
3. **Status is derived, never typed** — from the bound task files, or from one recorded evidence
   when there is nothing to bind (§4).
4. **Dependencies live on epics only**; a bound task file is not ready while its epic's
   dependencies are not done (§5).
5. **`tp epic lint` takes the structural checker, and mechanizes the one semantic gap that has a
   structure** (§6).
6. **Milestones are ordered tracks and a gate is a milestone's exit list** (§7).
7. **The operator's view is an export**; tp renders no map (§8).

## 2. The file

`.tp/epics.json` sits in the project layer tp already discovers (walk up to the `.git` boundary,
v0.24.0) beside `config.json`, committed like it, covered by the scope fence that keeps `.tp/` state
out of hand edits. `tp epic add|set|remove|import|claim|done` are the writers, each under flock,
each naming the file it wrote.

```json
{
  "version": 1,
  "milestones": [["A", "B", "C"], ["L1", "L2"]],
  "gates": {"A": {"members": ["money-type", "rounding"], "deferrable": ["ledger-export"]}},
  "tag_checks": {"ui": {"provided_by": "ui-rule-checker", "checks": [{"class": "ui-rules", "cmd": "…"}]}},
  "epics": [
    {"id": "money-type", "title": "Money type", "milestone": "A", "tags": ["det"],
     "depends_on": [{"id": "event-store", "why": "amounts are event payload"}],
     "acceptance": "…", "checks": [], "trigger": "", "estimate_minutes": 0,
     "discovered_from": "", "claimed_at": null, "done": null}
  ]
}
```

- `id` is an opaque non-empty string; a project's own naming, spaces and non-ASCII included, stays
  as it is. `title`, `acceptance` and `why` are content and keep their language.
- `depends_on` entries are objects and `why` is required; an edge without one is a lint error.
- `tags` replaces the request's `kind`; tp interprets no tag. `tag_checks.<tag>` attaches a check
  list to every epic carrying the tag, and its `provided_by` names the epic that makes the check
  passable (§6).
- `checks` has the workflow `checks` shape (`class`, `cmd`) and semantics: run at status time, exit 0
  is clean.
- `trigger` is text the operator wrote before the measurement; `discovered_from` is provenance;
  `claimed_at` and `done` are the two fields tp writes besides the graph itself.

**The graph file is primary.** The requesting project keeps prose primary and derives the JSON,
with a two-way coverage check between them that fired on every refactor. tp inverts that: the file tp
reviews and writes is the truth, and `tp epic export --md` writes the table a prose document embeds.
Prose commentary around it is the project's; tp lints none of it.

**Binding.** A task file carries one new top-level field, `epic`, written by
`tp init <spec> --epic <id>` or `tp set --epic <id>`. An epic lists no specs: its bound task files
are derived by walking the repository's `*.tasks.json` (tracked and untracked, ignore rules
respected). **Migration** is `tp epic import <file>` over this schema; the agent maps a project's
own file into it, and tp carries no importer for anyone's format (P4).

## 3. `tp epic review` — the loop over the graph

**The slice.** `tp epic review --milestone <m>` or `--epic <id>` emits one prompt per active graph
role over a slice: the named epics plus every epic one edge away, inbound or outbound. The whole
graph is never emitted; the requesting project never reviewed its graph whole either, and hundreds
of nodes in one prompt is the token failure P3 exists for. The slice is rendered deterministically,
one block per epic: id, title, milestone, tags, acceptance, each outgoing edge with its `why`, and
the gate it belongs to. Edges into the slice from outside are listed by id only.

**The unit and the carry.** The unit is the epic block; its `text_sha` is the hash of that block.
From round 2, a unit whose hash is unchanged since the round before and that carried no open finding
is marked `(carried)` in the index and is not asked about again — `tp ground`'s rule, verbatim. A
finding on an edge belongs to the source epic's unit; a finding on a gate to the milestone's.

**Roles.** A graph role corpus beside the reviewer and auditor corpora, `.tp/epic-reviewers/`,
ejected by `tp init --eject-roles` and activated like the others. The default panel is the two roles
that found the field project's real defects earliest, in the order its account recommends, plus one:

| role | asks | found in the field by |
|---|---|---|
| `dod-satisfiability` | does this epic's acceptance name a capability — a checker, a gate, a rule — that none of its dependencies, transitively, provides? | a reader, rounds late |
| `why-auditor` | does each `why` say why *this* dependency, or would it paste onto any edge? | a reader, every round |
| `phase-economist` | is this gate minimal — for each member, what breaks in the next milestone if it is deferred? is any epic placed by dependency where strategy wants it elsewhere? | the project's owner |

Structural defects — cycles, dangling ids, later-phase edges, missing `why` — are a lint class, and
`mechanized_classes` (v0.33.0) keeps them out of the prompts: a role is not asked what a command
answers. The renderer's *placement skeptic* is not a role; it is a non-goal (§11).

**Record, converge, cap.** `tp epic review --record <ndjson>` takes rows with the review gate's
required keys — `severity`, `finding`, `location`, `evidence` — where `location` is `epic:<id>`,
`edge:<from>→<to>` or `gate:<m>`, and refuses a row naming an id the slice did not contain. Rounds
are numbered per slice under `.tp-review/epics/<slice>/`, and `--status`/`--status --check` report
`consecutive_clean`, `converged` and the carry, under `review_clean_rounds`, `review_converge_on` and
`review_max_rounds` resolved from the project layer. No new convergence field is added. A recorded
finding is repaired by a graph write (`tp epic set`, `add`, `remove`), and the next round re-asks
exactly the units those writes changed. `--resolve` and `--resolve-all` apply as in `tp review`.

**Where it sits.** The field project's discovery ran in topic waves: research a topic, decide,
atomize, wire, review the slice until clean. That is Workflow E in `skills/tp/SKILL.md`: interview
the topic → `tp epic add --bulk` → `tp epic lint` → `tp epic review` over the touched slice until
`--status --check` exits 0 → `tp epic gate`. An epic added mid-work with `discovered_from` reopens
its neighbourhood's slice for one round. Under `tp run` nothing here runs: changing the plan is the
operator's, and a unit that finds it records it with `tp escalate --decision other`.

## 4. Status is derived

No field holds an epic's status; it is computed at read time from the files that exist.

| status | holds when |
|---|---|
| `planned` | no bound task file, no `done` record |
| `claimed` | `planned` plus `claimed_at` |
| `specced` | a bound task file exists and no task in any bound file is `wip` or `done` |
| `in_progress` | at least one task is `wip` or `done`, and not every bound file is at `release` |
| `implemented` | every bound file's `tp resume` phase is `release`, and a check fails or no check ran |
| `done` | every bound file is at `release` and every check exits 0 — or, with no bound file, a `done` record exists |

`release` is the phase `tp resume` already reports; no second definition of finished is added. A
spec whose tasks are all done and whose audit has not converged is in tp's `audit` phase, not
finished, and the epic layer says the same thing about the same file or two oracles disagree. The
cost of one audit per epic is agent work under `tp run`, tuned where tp tunes it —
`audit_clean_rounds` and the auditor panel at the project layer — and not weakened here;
`implemented` exists so the view shows the work landing before the audit does. Whether a spec may
opt out of its audit is a workflow question (`audit_clean_rounds` has no zero at `HEAD`; its range
is 1–10), parked in the sidecar; the epic layer inherits any answer, since it reads the oracle.

**The one typed thing: `tp epic done <id> --evidence "<text or URL>"`**, for epics that never get a
task file — founding a company, a filing, a field study. It writes `done: {at, evidence}` and is
refused (exit 1, naming them) while any bound task file exists, so a node that is half code and half
paperwork is two epics with an edge, the versioned-successor pattern (`x-I` → `x-II`) the field
project already uses. No *manual task kind*: relaxing atomicity, `source_sections` and the commit
rule per kind forks every rule for a case that has no spec body to decompose. `done_at` is derived:
the record's time, else the latest `closed_at` across the bound files.

## 5. Dependencies live on epics only

A task's `depends_on` keeps naming tasks in its own file; no `epic:<id>` form is added. `tp next`,
`tp next --peek`, `tp ready` and `tp claim` on a bound task file consult the graph first: while any
of the epic's dependencies is not `done`, no task is ready, the response carries
`blocked_by_epics: [ids]`, and `tp claim` exits 1 naming them. An unbound task file behaves exactly
as today. If a task needs another epic's result, the epic needs it, and the edge goes where the lint
and the loop can see it.

`tp epic ready` lists epics whose dependencies are all `done`, with no bound task file, no
`claimed_at`, and an empty `trigger` or a `triggered: {at, evidence}` record written by
`tp epic set <id> triggered="<evidence>"`. `tp epic claim <id>` writes `claimed_at` under flock and
exits 1 when it is set; binding a task file clears it. Under `TP_UNATTENDED`, `tp epic add`,
`remove`, `set depends_on`, `done` and `triggered` exit 2.

## 6. `tp epic lint`

Errors (exit 1): a cycle; a `depends_on` id naming no epic; an edge without `why`; a milestone no
track names; an edge to a later milestone of the same track; a gate member or deferrable naming no
epic, or a member outside its milestone; **an epic carrying a tag whose `tag_checks.provided_by`
it does not transitively depend on.** The last rule mechanizes the field project's most-recurrent
semantic gap — screen nodes not wired to the checker their own acceptance requires — for the case
where the requirement has a structure; the `dod-satisfiability` role keeps the cases that do not.
Warnings: an epic with no dependencies outside the first milestone of its track (every case seen in
the field was legitimate, so informational); a `why` equal to, or wholly contained in, the target's
id or title (`thin-why`, the restating edge in its mechanical form; the sidecar names it as a
candidate to be measured on a real corpus before it is more than a warning). Edges across tracks are
free. Every writer in §2 runs the error half before writing and refuses a write that would introduce
one.

## 7. Milestones and gates

`milestones` is a list of tracks, each an ordered list of names; the order rule holds within a
track. `gates.<milestone>` is `{members, deferrable}`; with no entry, a milestone's gate is every
epic in it. `tp epic gate <m>` reports each member `green`/`red` and exits 0 only when every
non-deferrable member is `done`. The field project's gates were written deliberately small to avoid
a "perfect foundation" phase; that is the `phase-economist`'s question, not a rule tp can hold.

## 8. The view

`tp epic status [--milestone <m>]`: per milestone, counts by status, `percent` done, the ready and
blocked lists, `critical_path` (longest chain of not-done epics by count, and by summed estimate when
every epic on it has one, an epic's estimate deriving from its bound tasks when its own is zero),
`throughput` (epics reaching `done` per ISO week over `done_at`) and the estimate-versus-actual
roll-up `tp report` computes per task, summed per epic. `tp epic export --json` writes the graph
with every derived field filled; `--md` writes the milestone tables. `--compact` applies to all.

## 9. Two releases

The operator chose two releases on 2026-11-08, the design side first, because the requesting
project is still refining its graph and the loop is what tp adds that a project script cannot.

| release | ships | from |
|---|---|---|
| first | `.tp/epics.json`, its writers and `tp epic import`; `tp epic lint`; `tp epic review` with the graph role corpus; `tp epic export`; Workflow E | §2, §3, §6, §7 (file and lint halves), §8 (`export` only) |
| second | the `epic` binding, derived status, `tp epic ready`/`claim`/`done`, the `tp next`/`ready`/`claim` read of the graph, `tp epic gate`, `tp epic status` | §4, §5, §7 (`gate`), §8 (`status`) |

Each release is its own spec and its own loop; the second's spec is written after the first has
shipped and the loop in §3 has run once on a real graph, so that what it binds to is measured, not
assumed. Within the first release the field is `claimed_at`-free and `done`-free: those two fields
enter the schema with the second.

## 10. Alternatives considered

1. **Storage, lint and status only, no loop.** The first draft of this note. Rejected by the
   account in the sidecar: the structural checker never found a semantic defect, and the manual
   rounds that did are the thing tp exists to record and converge.
2. **Run `tp review` over a prose roadmap.** Rejected: the roles and the `§` anchors are
   spec-shaped, and the two-way prose↔JSON coverage check would become tp's to keep.
3. **Nothing in tp; a project script and an `epic` tag.** Rejected: ready-at-epic needs the join
   with task state, and the claim race needs the lock.
4. **Milestones as tags plus a gate file.** Rejected: the order rule needs an order.
5. **A manual task kind.** Rejected in §4. **Task-to-epic edges.** Rejected in §5.
6. **One release.** Rejected in §9: the operator's recurring cost is cycle length, and the second
   half binds to behaviour the first half has not yet exhibited.

## 11. Non-goals

1. **No rendering**: no ASCII, mermaid or HTML map. The layout lessons in the request, and the
   *placement skeptic*, belong to the renderer that reads `tp epic export`. The field project
   recorded a layout defect its owner read as a graph defect; a map is where that is fixed.
2. **No `tp epic split`.** A split is `add` plus `set depends_on`, guarded by the lint and the next
   round; the file is tracked, so git is the history.
3. **No role-prompt audit of an epic's acceptance.** Machine acceptance is `checks`; the
   spec-level audit feeds `release`.
4. **No new convergence field, no new round shape.** The loop reuses `review_*` as resolved.
5. **No money or cost fields; no subtasks; no inference of a trigger; no change to `tp resume`.**
6. **No field data in this repository.** Fixtures are synthetic, generated to a configurable size;
   a real graph is measured where it lives.

## 12. Tests

Every row names a mutant that must fail it; the subjects do not exist at `HEAD`, so counts belong to
the implementing tasks' acceptance. Rows 1–6, 12, 13 and 15 belong to the first release.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 | a slice emission contains every epic one edge from the named set and none two edges away | emit the whole graph |
| 2 | §3 | round 2 over an unchanged slice marks every unit `(carried)`; one edited `why` un-carries exactly that epic | re-ask everything |
| 3 | §3 | `--record` refuses a row whose `location` names an epic outside the slice | accept any id |
| 4 | §3 | with `review_clean_rounds=2`, two clean rounds converge and `--status --check` exits 0; one does not | converge on one |
| 5 | §3 | a lint-class finding (a cycle) appears in no role prompt | ask roles about cycles |
| 6 | §2 | `tp epic add` with an edge missing `why` exits 1 and writes nothing | accept the edge |
| 7 | §4 | an epic whose only bound file is at `release` and whose check exits 1 is `implemented`, not `done` | ignore checks |
| 8 | §4 | `tp epic done` on an epic with a bound task file exits 1 naming the file | write the record |
| 9 | §4 | no field named `status` is written to `.tp/epics.json` by any command | persist the derived status |
| 10 | §5 | `tp next` on a bound file whose epic has an undone dependency claims nothing and names it; an unbound file is byte-identical to before | consult the graph in `tp ready` only |
| 11 | §5 | two `tp epic claim` on one id: one wins, the other exits 1 | write without the lock |
| 12 | §6 | a `ui`-tagged epic with no transitive path to `provided_by` is an error; adding the edge clears it | check direct edges only |
| 13 | §6 | an edge to a later milestone in the same track is an error; across tracks it is not | order milestones globally |
| 14 | §7 | `tp epic gate` exits 0 with a deferrable member open and 1 with a member open | ignore `deferrable` |
| 15 | §3, §5 | under `TP_UNATTENDED`, `tp epic add` and `tp epic review --record` exit 2 and write nothing | let the run change the plan |
