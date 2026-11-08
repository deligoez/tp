# tp v1.3.0 — A graph of epics, and the loop that reviews it

The first of the two releases decided in `spec/backlog/epics.md`: the graph file, its writers, its
lint, the review loop over a slice of it, and the export. The second release — binding a task file
to an epic, derived status, readiness, gates and the status view — is written after this one has run
once on a real graph. Provenance, the interview record and the pre-registered stop rules are in
`spec/1.3.0-measurements.md`; this file states the decisions and stands without it.

Class: **loop** — a review loop over a new document type, reusing the spec loop's record,
convergence and cap rules unchanged. Round budgets: `review_max_rounds` 3, `audit_max_rounds` 3.

## 1. The decision

**Context.** tp runs loops over the inside of one spec and has no word for what sits between specs.
A project that plans in a dependency graph of hundreds of epics designed that graph elsewhere: a
structural checker caught every cycle, dangling id and out-of-order edge, and every semantic defect
— a `why` that restates its edge, an epic whose acceptance names a capability no dependency
provides, an epic placed by dependency where strategy wanted it elsewhere — was caught by a reader or by the project's owner
in manual critical-only rounds over a slice, repeated until one came back clean. That is the spec loop's problem one level up, and the backlog note
records the account it rests on.

**Decision.**

1. **One tp-owned file, `epics.json` under `.tp/`**, tracked, read by every command here and
   written only by `tp epic` writers (§2, §3).
2. **`tp epic lint` is the structural checker**, and the one semantic gap with a structure — a
   tag-attached check nobody in the dependency closure provides — is a lint rule (§4).
3. **`tp epic review` runs the spec loop over a slice of the graph**: emit one prompt per graph
   role, record findings with the review gate's four keys, carry an unchanged epic, converge and
   cap under the project's `review_*` fields (§5).
4. **`tp epic export` is the only view**; tp renders no map (§6).
5. **Workflow E in the skill** is how a graph is grown and reviewed, topic by topic (§7).

v0.1.0's *flat task list, no epics* is untouched below the task. An epic is a node of the plan that
owns the spec written for it; this release does not yet connect the two.

## 2. The graph file

The file lives in the project layer tp already discovers — the `.tp/` directory at the `.git`
boundary, beside `config.json` — and is committed like it. The plugin's write-deny hook names it
beside `config.json`, so it falls under the same fence (§7).

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
     "discovered_from": ""}
  ]
}
```

- `milestones` is a list of tracks, each an ordered list of names. `milestone` is required on every
  epic and names one of them.
- `id` is an opaque non-empty string, unique in the file; a project's own naming, spaces and
  non-ASCII included, stays as it is. `title`, `acceptance` and `why` are content and keep their
  language.
- `depends_on` entries are objects and `why` is required.
- `tags` are opaque to tp. `tag_checks.<tag>` attaches a check list to every epic carrying the tag;
  `provided_by` names the epic that makes those checks passable. `checks` entries have the workflow
  `checks` shape, `class` and `cmd`; this release stores and lints them and runs none.
- `gates.<milestone>` is `{members, deferrable}`; nothing reads it in this release beyond the lint.
- `trigger`, `estimate_minutes` and `discovered_from` are stored and exported, read by nothing else
  here.

**The graph file is primary.** The requesting project kept prose primary and derived its JSON, with
a two-way coverage check between them. tp inverts that: the file tp lints, reviews and writes is the
truth, and `tp epic export --md` writes the table a prose document embeds.

## 3. The writers

`tp epic add <json>` (`--stdin`, `--bulk <ndjson>`), `tp epic set <id> <field>=<value>`,
`tp epic remove <id>` and `tp epic import <file>` are the only writers. Each takes a per-file lock,
as the writers of `config.json` and `local.json` do, names the file it wrote as `file`, and runs §4's error rules over the
file **as it would be after the write**: a write that would introduce an error exits 1 naming the
rule and writes nothing. `tp epic remove` refuses an epic another epic depends on unless `--force`,
which also drops the edges into it. `tp epic import` refuses to replace a file that already holds
epics unless `--force`. The agent maps a project's own graph file into §2's shape; tp carries no
importer for anyone's format.

Under `TP_UNATTENDED` every writer exits 2 and writes nothing: changing the plan is the operator's,
and a unit that finds the need records it with `tp escalate --decision other`.

## 4. `tp epic lint`

Reads the file, writes nothing, exits 0 when no rule fires at error severity, 1 when one does, 3
when there is no file. The JSON payload carries `errors`, `warnings` and `findings[]`, each finding
`{rule, severity, epic, message}` with `epic` null for a rule about the file as a whole.

| rule | severity | fires when |
|---|---|---|
| `cycle` | error | the dependency graph has a cycle; the message names its members |
| `dangling-dependency` | error | a `depends_on` id names no epic |
| `missing-why` | error | an edge has no `why`, or one that is empty once trimmed |
| `unknown-milestone` | error | an epic's `milestone` is absent or in no track |
| `later-milestone-dependency` | error | an epic depends on one in a later milestone of the same track; edges across tracks are free |
| `gate-unknown-member` | error | a gate's member or deferrable names no epic |
| `gate-member-outside-milestone` | error | a gate's member is not in that milestone |
| `unprovided-tag-check` | error | an epic carries a tag whose `tag_checks.provided_by` is not in its transitive dependencies |
| `no-dependencies` | warning | an epic outside the first milestone of its track has no dependencies |
| `thin-why` | warning | a `why` equals, or is wholly contained in, the target epic's id or title |

`cycle`, `dangling-dependency` and `later-milestone-dependency` are the checker the requesting
project ran by hand; the other error rules are tp's own; `unprovided-tag-check` gives a structure to
the one semantic gap that project's readers found late. `thin-why` stays a warning until counted on
a real corpus (sidecar).

## 5. `tp epic review`

**The slice.** `tp epic review --milestone <m>` or `tp epic review --epic <id>`, exactly one of the
two; neither, or both, is a usage error (exit 2) that writes nothing. The slice is the named epics
plus every epic one edge away in either direction. The whole graph is never emitted. The slice is
rendered one block per epic — id, title, milestone, tags, acceptance, each outgoing edge with its
`why`, the gate it belongs to — with edges from outside the slice listed by id only.

**The unit and the carry.** The unit is the epic block, and the emission's index lists every unit
with the hash of its block. From round 2, a unit whose hash is unchanged since the round before and
that carried no open finding is marked `(carried)` in the index and is not asked about again;
unlike `tp ground`, a unit with an open finding is re-asked. A finding on an edge belongs to the
source epic's unit.

**Roles.** A graph role corpus, `.tp/epic-reviewers/`, beside the reviewer and auditor corpora,
ejected by `tp init --eject-roles` and following the same file schema and activation rules. The
default panel is three roles:

| role | asks |
|---|---|
| `dod-satisfiability` | does this epic's acceptance name a capability — a checker, a gate, a rule — that none of its dependencies, transitively, provides? |
| `why-auditor` | does each `why` say why *this* dependency, or would it paste onto any edge? |
| `phase-economist` | is this gate minimal — for each member, what breaks in the next milestone if it is deferred? is any epic placed by dependency where strategy wants it elsewhere? |

The built-in `regression` role is appended from round 2 when the slice changed since the round
before or a finding was resolved `fixed`, as in `tp review`. `--role <name>` narrows the emission to one prompt with the same three
name classes as `tp review --role`. Findings are recorded in English whatever language the graph
is in.

**Mechanized classes.** §4's error rules run at every emission as `mechanical_checks`, and their
slugs are named in `mechanized_classes` and withheld from the prompts: a role is not asked what a
command answers.

**Record, converge, cap.** `tp epic review --record <ndjson>` takes rows with the review gate's
four required keys — `severity`, `finding`, `location`, `evidence` — where `location` is
`epic:<id>` for an epic in the slice or `gate:<m>` for the slice's milestone; a row naming anything
else refuses the whole file at exit 1, naming every offending line, and writes no round. Rounds are
numbered per slice; the slice's state directory lives under `.tp-review/epics/` and `--status` names
it as `state_dir`. `--status` carries `slice`, `round`, `consecutive_clean`, `converged`, `carried`,
`in_flight_round` and `next_action`; `--status --check` exits 0 only when the slice is converged and
every mechanical check passes. Convergence and the cap resolve from the project layer over the
built-in default — `review_clean_rounds`, `review_converge_on`, `review_max_rounds` — with no task
file layer, because a graph has no task file. A reached cap refuses emission and record with exit 4
and the escalation hint `tp review` gives. `--resolve` and `--resolve-all` apply to a recorded round
as in `tp review`. Under `TP_UNATTENDED`, `--record` exits 2 and writes nothing.

A recorded finding leaves the round as a graph write through §3 or as a disposition; the next round
re-asks exactly the units those writes changed.

## 6. `tp epic export`

`--json` writes the file's content with each epic's `in_degree`, `out_degree` and `track` added;
`--md` writes one table per milestone, in track order, one row per epic with id, title, tags and
each dependency with its `why`. Both are read-only. `--compact` applies to both.

## 7. Documents and the plugin

`skills/tp/SKILL.md` gains Workflow E — interview the topic → `tp epic add --bulk` →
`tp epic lint` → `tp epic review` over the touched slice until `--status --check` exits 0 → the
next topic — and the inventory rows for every command and flag above. `skills/tp/REFERENCE.md`
gains the file schema, the lint rules, the state directory and the exit codes. `README.md` gains the
command index rows and the lint rule table rows. The write-deny hook under `hooks/` names `epics.json` beside
`config.json`. `.claude-plugin/plugin.json` is bumped before the tag.

## 8. Non-goals

1. **No binding, no derived status, no `tp epic ready`, `claim`, `done`, `gate` or `status`, and no
   read of the graph by `tp next`, `tp ready` or `tp claim`.** The second release.
2. **No `tp resume` phase and no `tp run` unit kind.** The loop is driven by the agent, as
   `tp ground` is.
3. **No rendering.** No ASCII, mermaid or HTML map.
4. **No `tp epic split`.** A split is `add` plus `set depends_on`, guarded by the lint and the next
   round; the file is tracked, so git is the history.
5. **No frontmatter overrides for graph roles.** A project edits its ejected corpus.
6. **No new convergence field and no new round shape.**
7. **No field data in this repository.** Fixtures are synthetic, generated to a configurable size.
8. **No task-to-epic edges, no manual task kind.** Decided in the backlog note.

## 9. Tests

Each row names its fixture and its mutant. The subjects do not exist at `HEAD`, so the two counts a
row pins belong to the implementing task's acceptance.

| # | WHEN | tp SHALL | fixture | mutant |
|---|---|---|---|---|
| 1 | `tp epic add` is given an edge without `why` | exit 1 naming `missing-why`, leave the file byte-identical | a file holding one epic `a` in milestone `A`; the payload adds `b` with `depends_on: [{"id": "a"}]` | a writer that validates after writing: the file now holds `b` |
| 2 | `tp epic add` would close a cycle | exit 1 naming `cycle` and both members, leave the file byte-identical | a file with `a → b`; the payload sets `b → a` | a writer that runs only the per-row rules, so the cycle lands and `tp epic lint` finds it later |
| 3 | `tp epic lint` runs over a file holding one defect per error rule | exit 1 with `errors` equal to the rule count and each error slug present exactly once | one file: a two-member cycle, one dangling id, one empty `why`, one epic with no `milestone`, one same-track later-milestone edge, one gate naming an unknown member, one gate member in the wrong milestone, one `ui`-tagged epic with no path to the provider | a lint that stops at the first error |
| 4 | an epic's path to `provided_by` is transitive | `unprovided-tag-check` does not fire; removing the middle edge makes it fire | tracks `[[A]]`; `ui-rule-checker`, `x → ui-rule-checker`, `screen` tagged `ui` with `screen → x` | a check over direct edges only, which fires on the intact fixture |
| 5 | an edge crosses a milestone boundary | `later-milestone-dependency` fires within a track and not across tracks | tracks `[[A, B], [L1]]`; `a` in `A` depends on `b` in `B` and on `l` in `L1` | a global milestone order, which fires on the `L1` edge too |
| 6 | `tp epic review --epic b` is emitted | the index lists `a`, `b`, `c` and not `d` | the chain `a → b → c → d` | emit the whole graph |
| 7 | round 2 is emitted over an unchanged slice, then over one edited `why` | every unit `(carried)` in the first, exactly the edited epic un-carried in the second | the chain of row 6, round 1 recorded with no findings | re-ask every unit |
| 8 | `--record` is given a row whose `location` names an epic outside the slice | exit 1 naming the line, write no round file | slice `--epic b` over row 6's chain; a two-row file whose second row's `location` is `epic:d` | accept any epic id in the file |
| 9 | two rounds are recorded with no findings | `converged: true` and `--status --check` exits 0 after the second, 1 after the first | `review_clean_rounds` 2 at the project layer; the slice of row 6 | converge on one clean round |
| 10 | the cap is reached unconverged | emission and `--record` exit 4 with the escalation hint | `review_max_rounds` 1 at the project layer; one round recorded holding a `high` finding | emit round 2 |
| 11 | the slice holds a cycle and two clean rounds are recorded | `mechanical_checks` names `cycle` as failing and `--status --check` exits 1 | row 6's chain plus `d → a`, the lint error present at both emissions; two rounds recorded with no findings | run the lint only under `tp epic lint` |
| 12 | `TP_UNATTENDED=1` is set | `tp epic add` and `tp epic review --record` exit 2 and write nothing | the payload of row 1 made legal with a `why`; a legal one-row findings file over row 6's slice | fence the writers and not `--record`, so the record lands |
| 13 | `tp epic export --md` runs | one table per milestone in track order, every epic in exactly one row | tracks `[[A, B], [L1]]` with two epics per milestone | group tables by tag |
| 14 | two slices are emitted | two `state_dir` values, both existing, neither inside the other | `--milestone A` and `--epic b` over row 6's chain in milestone `A` | one state directory for the graph |
| 15 | `tp epic review` is given both `--milestone` and `--epic`, or neither | exit 2, no state directory created | row 6's chain | pick the first flag and emit |
| 16 | `tp epic import` targets a file that already holds epics | exit 1 and the file byte-identical without `--force`; replaced with it; exit 2 and byte-identical with `--force` under `TP_UNATTENDED=1` | a file holding row 6's chain; an import payload holding one epic | an import that always replaces |
| 17 | round 2 is emitted after the slice changed | a `regression` prompt is appended; over an unchanged slice it is not | row 6's chain, round 1 recorded with no findings, one `why` edited before the first emission and nothing edited before the second | append `regression` on every round from 2 |
