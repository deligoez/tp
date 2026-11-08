# tp v1.3.0 — A graph of epics, and the loop that reviews it

The first of the two releases decided in `spec/backlog/epics.md`: the graph file, its writers, its
lint, the review loop over a slice of it, and the export. Binding, derived status, readiness, gates
and the status view are the second, written after this one has run on a real graph. The interview
and the stop rules are in `spec/1.3.0-measurements.md`; this file stands without it.

Class: **loop** — a review loop over a new document type, reusing the spec loop's rules unchanged.

## 1. The decision

**Context.** tp runs loops inside one spec and has no word for what sits between specs. A project
planning in a graph of hundreds of epics designed it elsewhere: a checker caught every structural
defect, and every semantic one — a `why` restating its edge, an acceptance no dependency can
satisfy, an epic placed by dependency against strategy — was caught by a reader or the owner in
manual rounds over a slice, repeated until clean. That is the spec loop's problem one level up.

**Decision.**

1. **One tp-owned file, `epics.json` under `.tp/`**, tracked, read by every command here and
   written only by `tp epic` writers (§2, §3).
2. **`tp epic lint` is the structural checker**, and the one semantic gap with a structure — a
   tag's provider missing from the dependency closure — is a lint rule (§4).
3. **`tp epic review` runs the spec loop over a slice of the graph**: emit per graph role, record
   with the review gate's four keys, carry an unchanged epic, converge and cap under `review_*` (§5).
4. **`tp epic export` is the only view**; tp renders no map (§6).
5. **Workflow E in the skill** is how a graph is grown and reviewed, topic by topic (§7).

## 2. The graph file

The file lives in the project layer tp already discovers — the `.tp/` directory at the `.git`
boundary, beside `config.json` — and is committed like it. The plugin's write-deny hook names it
beside `config.json`, so it falls under the same fence (§7).

```json
{
  "version": 1,
  "milestones": [["A", "B", "C"], ["L1", "L2"]],
  "gates": {"A": {"members": ["money-type", "rounding"], "deferrable": ["ledger-export"]}},
  "tag_checks": {"ui": {"provided_by": "ui-rule-checker"}},
  "epics": [
    {"id": "money-type", "title": "Money type", "milestone": "A", "tags": ["det"],
     "depends_on": [{"id": "event-store", "why": "amounts are event payload"}],
     "acceptance": "…"}
  ]
}
```

- `milestones` is a list of tracks, each an ordered list of names; a name appears once across all
  tracks. `milestone` is required on every epic and names one of them.
- `id` is an opaque non-empty string, unique in the file; a project's own naming, spaces and
  non-ASCII included, stays as it is. `title`, `acceptance` and `why` are content and keep their
  language.
- `depends_on` entries are objects and `why` is required.
- `tags` are opaque to tp. `tag_checks.<tag>.provided_by` names the epic every epic carrying the
  tag must depend on, directly or transitively. The check commands a tag will attach, and the
  per-epic fields the second release reads — `checks`, `trigger`, `estimate_minutes`,
  `discovered_from` — are not in this release's schema; an unknown key is a lint error.
- `gates.<milestone>` is `{members, deferrable}`, read by the lint and the gate unit (§5).

**The graph file is primary**, inverting the requesting project's prose-first practice: the file tp
lints, reviews and writes is the truth, and `--md` export writes the table a prose document embeds.

## 3. The writers

`tp epic add <json>` (`--stdin`, `--bulk <ndjson>`), `tp epic set <id> <field>=<value>`,
`tp epic set <top-level-field>=<json>` for `milestones`, `gates.<m>` and `tag_checks.<tag>`,
`tp epic remove <id>` and `tp epic import <file>` are the only writers. Each takes a per-file lock,
as the writers of `config.json` and `local.json` do, names the file it wrote as `file`, and runs §4's
error rules over the file as it would be after the write: a write whose after-state holds an error
the before-state did not exits 1 naming the rule and writes nothing, so a file already holding an
error is repaired one write at a time. A `--bulk` row whose `id` exists replaces that epic, so a
round's repairs land in one call. `tp epic remove` refuses an epic another epic depends on unless
`--force`, which also drops the edges into it and its gate memberships; a provider named in
`tag_checks` is refused even then. `tp epic import` refuses to replace a file that already holds
epics unless `--force`; the agent maps a project's own file into §2's shape, tp has no importer.

Under `TP_UNATTENDED` every writer exits 2 and writes nothing: changing the plan is the operator's,
and a unit that finds the need records it with `tp escalate --decision other`.

## 4. `tp epic lint`

Reads the file, writes nothing, exits 0 when no rule fires at error severity, 1 when one does, 3
when there is no file. The JSON payload carries `errors`, `warnings` and `findings[]`, each finding
`{rule, severity, epic, message}` with `epic` null for a rule about the file as a whole. A cycle is
one finding, `epic` its first member in file order, the message naming every member.

| rule | severity | fires when |
|---|---|---|
| `cycle` | error | the dependency graph has a cycle |
| `dangling-reference` | error | a `depends_on` id, a gate member or deferrable, or a `provided_by` names no epic; a `gates` key names no milestone |
| `duplicate-name` | error | an epic id or a milestone name appears twice |
| `unknown-key` | error | a key outside §2's schema |
| `missing-why` | error | an edge has no `why`, or one that is empty once trimmed |
| `unknown-milestone` | error | an epic's `milestone` is absent or in no track |
| `later-milestone-dependency` | error | an epic depends on one in a later milestone of the same track; edges across tracks are free |
| `gate-member-outside-milestone` | error | a gate's member is not in that milestone |
| `unprovided-tag-check` | error | an epic carries a tag whose `provided_by` is not in its transitive dependencies; the provider itself is exempt |
| `no-dependencies` | warning | an epic outside the first milestone of its track has no dependencies |
| `thin-why` | warning | a non-empty `why` equals, or is wholly contained in, the target epic's id or title |

`cycle`, `dangling-reference` and `later-milestone-dependency` were the requesting project's hand-run
checker; `unprovided-tag-check` structures the semantic gap its readers found late; `thin-why` stays
a warning until counted on a real corpus (sidecar).

## 5. `tp epic review`

**The slice.** `tp epic review --milestone <m>` or `tp epic review --epic <id>`, exactly one of the
two; neither, or both, is a usage error (exit 2) that writes nothing. The units are the named epics
— every epic of the milestone, or the one epic. Each unit is rendered as a block: id, title,
milestone, tags, acceptance, and each outgoing edge with its `why`. Below them, as context and not as
units, the block of every epic one edge away in either direction; a `--milestone` slice adds one gate
unit, the gate's members and deferrables. No command emits the graph unsliced.

**The unit and the carry.** The index lists every unit with a hash covering its own block and the
blocks of its direct dependencies. From round 2, a unit whose hash is unchanged since the round
before and that carried no open finding is marked `(carried)` and is not asked about again; unlike
`tp ground`, a unit with an open finding is re-asked. A finding on an edge belongs to the source
epic's unit; a finding on the gate to the gate unit.

**Roles.** A graph role corpus, `.tp/epic-reviewers/`, ejected by `tp init --eject-roles` with the
reviewer file schema. Every file in it is an active role, deleted to deactivate, `domains` unread:

| role | asks |
|---|---|
| `dod-satisfiability` | does this epic's acceptance name a capability — a checker, a gate, a rule — that none of its dependencies, transitively, provides? |
| `why-auditor` | does each `why` say why *this* dependency, or would it paste onto any edge? |
| `phase-economist` | is this gate minimal — for each member, what breaks in the next milestone if it is deferred? is any epic placed by dependency where strategy wants it elsewhere? |

The built-in `regression` role is appended from round 2 when the slice changed since the round
before or a finding was resolved `fixed`, as in `tp review`. `--role <name>` narrows the emission to
one prompt with the same three name classes as `tp review --role`. Findings are recorded in English
whatever language the graph is in.

**Mechanized classes.** §4's error rules run over the whole file at every emission as
`mechanical_checks` — a graph with a structural error is not reviewable in any slice — and their
slugs are named in `mechanized_classes` and withheld from the prompts.

**Record, converge, cap.** `tp epic review --record <ndjson>` takes rows with the review gate's
four required keys — `severity`, `finding`, `location`, `evidence` — where `location` is
`epic:<id>` for a unit of the slice or, in a `--milestone` slice, `gate:<m>` for its gate; a row
naming anything else refuses the whole file at exit 1, naming every offending line, and writes no
round. Each slice has its own state directory under `.tp/epic-review/`, named by a hash of the slice
key so that `--milestone A` and `--epic A` never share one and no id reaches the file system;
`--status` names it as `state_dir`. `--status` carries `slice`, `round`, `consecutive_clean`,
`converged`, `done`, `done_by`, `stale`, `carried`, `in_flight_round` and `next_action`; `stale` is
true while any unit's hash differs from the last recorded round's, and a stale slice is not
converged. `--status --check` exits 0 only when the slice's loop is `done` — converged, or at the
cap as in `tp review` — and every mechanical check passes. Convergence and the cap resolve from the
project layer over the built-in default — `review_clean_rounds`, `review_converge_on`,
`review_max_rounds` — with no task file layer, because a graph has no task file. A reached cap
refuses emission and record with exit 4 and `tp review`'s escalation hint; `--resolve`/`--resolve-all`
apply as in `tp review`; under `TP_UNATTENDED`, `--record` exits 2 and writes nothing.

A recorded finding leaves the round as a graph write through §3 or as a disposition; the next round
re-asks the units whose hash changed and the units with an open finding.

## 6. `tp epic export`

`--json` writes the file's content as stored, normalised; `--md` writes one table per milestone, in
track order, one row per epic with id, title, tags and each dependency with its `why`. Both are
read-only. `--compact` applies to `--json`.

## 7. Documents and the plugin

`skills/tp/SKILL.md` gains Workflow E — interview the topic → `tp epic set milestones=…` on a new
file → `tp epic add --bulk` → `tp epic lint` → `tp epic review` over the touched slice until
`--status --check` exits 0 → the next topic — and the inventory rows for every command and flag
above. `skills/tp/REFERENCE.md` gains the file schema, the lint rules, the state directory and the
exit codes. `README.md` gains the command index rows and the lint rule table rows. The write-deny
hook under `hooks/` names `epics.json` and `epic-review/` beside `config.json`. `.claude-plugin/plugin.json` is bumped
before the tag.

## 8. Non-goals

1. **No binding, no derived status, no `tp epic ready`, `claim`, `done`, `gate` or `status`, and no
   read of the graph by `tp next`, `tp ready` or `tp claim`.** The second release.
2. **No `tp resume` phase and no `tp run` unit kind.** The loop is driven by the agent, as
   `tp ground` is.
3. **No rendering.** No ASCII, mermaid or HTML map.
4. **No `tp epic split`.** A split is `add` plus `set`, guarded by the lint and the next round.
5. **No frontmatter overrides for graph roles** (a project edits its ejected corpus), **no new
   convergence field, no new round shape, no task-to-epic edges, no manual task kind.**
6. **No field data in this repository.** Fixtures are synthetic, generated to a configurable size.

## 9. Tests

Each row names its fixture and its mutant. The subjects do not exist at `HEAD`, so the two counts a
row pins belong to the implementing task's acceptance. "The chain" is `a → b → c → d`, all in
milestone `A` of tracks `[[A, B], [L1]]`, every `why` distinct.

| # | WHEN | tp SHALL | fixture | mutant |
|---|---|---|---|---|
| 1 | `tp epic add` is given an edge without `why` | exit 1 naming `missing-why`, leave the file byte-identical | a file holding one epic `a`; the payload adds `b` with `depends_on: [{"id": "a"}]` | a writer that validates after writing: the file now holds `b` |
| 2 | a write would add an error to a file that already holds one | exit 1 naming only the new rule; a write that removes the old error and adds none exits 0 | a file with a dangling edge on `a`; first `add` closes a cycle `b → a → b`, then `set` replaces `a`'s `depends_on` with a legal edge | refuse any after-state error, so the repairing `set` exits 1 |
| 3 | `tp epic lint` runs over a file holding one defect per error rule | exit 1 with `errors` equal to the rule count and each error slug present exactly once | one file: a two-member cycle, one dangling edge, one duplicated id, one unknown key, one empty `why`, one epic with no `milestone`, one same-track later-milestone edge, one gate member in the wrong milestone, one `ui`-tagged epic with no path to the provider | a lint that stops at the first error |
| 4 | an epic's path to `provided_by` is transitive, or the epic is the provider | `unprovided-tag-check` does not fire; removing the middle edge makes it fire on the dependent only | `ui-rule-checker` tagged `ui`, `x → ui-rule-checker`, `screen` tagged `ui` with `screen → x` | a check over direct edges only, or one that does not exempt the provider |
| 5 | an edge crosses a milestone boundary | `later-milestone-dependency` fires within a track and not across tracks | `a` in `A` depends on `b` in `B` and on `l` in `L1` | a global milestone order, which fires on the `L1` edge too |
| 6 | `tp epic review --epic b` is emitted | the index lists `b` as its only unit, the context lists `a` and `c`, and `d` appears nowhere | the chain | emit neighbours as units, or the whole graph |
| 7 | round 2 is emitted over an unchanged slice and recorded with no rows, then one `why` on `a` is edited and round 3 emitted | every unit `(carried)` in round 2; in round 3 exactly `a` and its dependant `b` un-carried | `--milestone A` over the chain, round 1 recorded with no findings | hash the own block only, so `b` stays carried |
| 8 | `--record` is given a row whose `location` names an epic outside the slice, or `gate:A` in an `--epic` slice | exit 1 naming the line, write no round file | slice `--epic b` over the chain; a three-row file whose second row's `location` is `epic:d` and third `gate:A` | accept any id in the file |
| 9 | two rounds are recorded with no findings | `converged: true` and `--status --check` exits 0 after the second, 1 after the first | `review_clean_rounds` 2 at the project layer; `--milestone A` over the chain | converge on one clean round |
| 10 | the cap is reached | emission and `--record` exit 4 with the escalation hint; `--status` reads `done: true, done_by: cap` and `--check` exits 0 | `review_max_rounds` 1 at the project layer; one round recorded holding a `high` finding | emit round 2, or keep `--check` at 1 |
| 11 | the file holds a cycle outside the slice and two clean rounds are recorded | `mechanical_checks` names `cycle` as failing and `--status --check` exits 1 | the chain plus `d → a`, slice `--epic b`; two rounds recorded with no findings | run the lint over the slice only |
| 12 | `TP_UNATTENDED=1` is set | every writer of §3 and `--record` exit 2 and write nothing | one legal payload per writer, and a legal one-row findings file over `--epic b` | fence `add` and `--record` only |
| 13 | `tp epic export --md` runs | one table per milestone in track order, every epic in exactly one row | two epics per milestone over `[[A, B], [L1]]` | group tables by tag |
| 14 | `--milestone A` and `--epic A` are emitted on a graph holding an epic named `A` | two distinct `state_dir` values, both under `.tp/epic-review/`, neither containing the id | the chain plus an epic `A`; a second graph whose only epic is `../x` | derive the directory from the id text |
| 15 | `tp epic review` is given both `--milestone` and `--epic`, or neither | exit 2, no state directory created | the chain | pick the first flag and emit |
| 16 | `tp epic import` targets a file that already holds epics | exit 1 and the file byte-identical without `--force`; replaced with it | a file holding the chain; an import payload holding one epic | an import that always replaces |
| 17 | round 2 is emitted and recorded with no rows, then a `why` is edited and round 3 emitted; separately round 3 is emitted with no edit after a round-2 row resolved `fixed` | the first round 3 carries a `regression` prompt; a round 3 after a clean, unedited round 2 does not; the second carries one | `--milestone A` over the chain, round 1 recorded with no findings | append `regression` on every round from 2, or only on a changed slice |
| 18 | a unit's finding is open at record time | round 2's index does not mark that unit `(carried)` while its hash is unchanged; resolving the finding `wontfix` and re-emitting marks it | `--epic b` over the chain, round 1 recorded with one `medium` row on `epic:b` | carry on hash alone |
| 19 | the graph changes after a slice converged | `--status` reads `stale: true` and `converged: false` until a round is recorded over the new text | `--milestone A` over the chain converged in two rounds; then `a`'s title is edited | read convergence from the recorded rounds alone |
| 20 | `tp epic set milestones=…` runs on a file with no epics, then `add --bulk` with an existing id | the tracks are written; the bulk row replaces the epic rather than failing on `duplicate-name` | an empty file; tracks `[[A]]`; two bulk calls carrying the same `id` with different titles | refuse the second bulk row |
