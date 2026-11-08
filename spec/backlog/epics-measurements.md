# epics — measurements

The sidecar of `epics.md`. The body stands without it; this file holds the provenance of what the
body rests on. The requesting project is not named here, by its owner's decision, and none of its
data — ids, counts, paths, files — is in this repository. What follows is its *process*, as that
project's own agent reported it to tp on 2026-11-08, in two messages: the request, and an account of
how the graph was built.

## The request as received

A project whose plan is a dependency graph of a few hundred nodes, each node an epic in
project-management terms, asked tp for an epic layer with its own dependency tree. The owner's
framing, relayed and then stated in the tp session the same day: each node is an epic; tp should
own the tree; status is derived, never typed ("no chores"); this is *epic design, not spec design*
— one spec per node — and the tree should be designable with tp, with epic loops working on its
dependencies as spec loops work on a spec.

The graph at the source: nodes with a phase and a kind, edges each carrying a `why`; a checker for
cycles, dangling ids, edges to a later phase, prose-table ↔ JSON coverage in both directions, and a
warning on a non-first-phase node with no dependencies. Gates are per-phase exit lists in the prose,
some with "may finish in the next phase" exceptions. Cross-cutting definitions of done exist (a set
of UI principles checked by two commands, required of every node with a screen).

## How the tree was built

| topic | as reported | taken into the body |
|---|---|---|
| discovery | three braided sources (a strategy skeleton, competitor module inventories, owner interviews); the count grew in topic waves, each wave one topic researched, decided, then atomized | Workflow E runs per topic wave, §3 |
| node size | judgement, no written criterion: one shippable, independently verifiable unit with its own done; a version split (`x-I` → `x-II`) when the same capability deepens in a later phase; refused when the halves cannot pass apart | the versioned-successor pattern, §4; no size rule in tp |
| edges | written with the node, `why` required; separate critical-only passes looked for missing edges; few edges removed, retargeting and `why` corrections far more common | `why` required at the writer, §2; the loop re-asks only changed units, §3 |
| phases and gates | strategic order, not technical layering; gates written minimal on purpose; moves between phases when an earlier phase's node or contract depended on it | `phase-economist`, §3; order rule within a track, §6 |
| review | per node neighbourhood and per slice, never the whole graph; critical-only, until a round came back clean; the checker caught structure, readers caught semantics; the owner corrected terminology, scope order, and one layout defect read as a graph defect | the slice and the carry, §3; non-goal 1, §11 |
| failure modes | recurring: later-phase edges (checker), restating `why` (reader), screen nodes unwired from their checker (reader, rounds late), dependency-free nodes misread as orphans on the map | `mechanized_classes` for the first, two roles for the next two, `provided_by` lint for the structured half of the third, §6; `thin-why` as a candidate warning |
| source of truth | prose primary, JSON derived, two-way coverage check fired on refactors; both edited in one commit | inverted: the graph file is primary and `--md` exports the table, §2 |
| review prompts that would have found defects earliest | a DoD-satisfiability auditor, a why auditor, a phase economist, a placement skeptic (renderer); run the first two first | the default panel, §3; the skeptic is the renderer's |

## What the body answers, question by question

| question | answer |
|---|---|
| (a) relax v0.1.0 by one level? | §1: a document above the spec and its loop; nothing below a task |
| (b) where does the list live? | §2: `.tp/epics.json`, tp-owned, migrated once by `tp epic import` |
| (c) manual, non-code work? | §4: `tp epic done --evidence`, no manual task kind |
| (d) epic-level audit? | §3 reviews the graph; acceptance of a built epic is `checks` (§4, non-goal 3) |
| (e) milestones and gates — tp concepts or tags? | §7: fields, because order is needed |

## Candidates to measure before they are rules

- **`thin-why`** (§6): on a real graph, count the `why` strings equal to or contained in the
  target's id or title and read a sample of the hits. The rule is worth a warning only if the hits
  are the restating edges the readers kept finding; the account did not count them. The measurement
  runs where the graph lives, and only its conclusion comes back here.
- **The slice radius**: one edge of neighbourhood is the account's practice, not a measurement.
  Record the prompt byte size of the largest milestone slice on a real graph before fixing it.

## Parked

- **A spec that opts out of its audit.** `DetectPhase` (`internal/engine/phase.go`) reports
  `release` only when every task is done and the audit loop has converged, and `audit_clean_rounds`
  ranges 1–10 with no zero. The question was raised on 2026-11-08 — many small nodes, one audit each
  — and decided for the epic layer by binding to the oracle (§4). If the cost shows up in practice,
  the change is a workflow field (`audit_clean_rounds=0`, or a per-spec `audit: false`), and the epic
  layer needs no change to follow it.

## What the fixture is

A generator in `testdata` writes a synthetic graph of configurable size in the §2 schema — tracks,
gates, tagged epics, `provided_by` chains, planted cycles and later-phase edges on request — for the
lint, the slice emission and, in the second release, the status derivation at scale. tp's own
repository, with two backlog candidates and one ordering between them, is the small fixture for
dogfooding Workflow E. No field data enters the repository (§11, non-goal 6).
