# tp — An epic's description

A backlog decision candidate for 1.11.0, decided by the operator, who is also the planner asking for
it; the candidate's job is to state the cost the request does not.

Class: **decision**. One optional graph field, read by the epic-reviewing roles as the epic's
primary text.

## 1. The decision

**Context.** An epic carries a title, a short acceptance and why'd edges. A planning phase wants
more per epic — purpose, scope, non-goals, decisions taken, open questions, background — without a
bound spec, which would mark every epic `specced`, and without long acceptances, which every role
pays for and the gate checks.

**Decision.** An optional markdown string, `epics[].description`, written only when non-empty and
never parsed. Long text is written through `tp epic apply` (a `set` row's JSON `value`), so no
shell quoting is needed; `tp epic set <id> description=<text>` takes short text.

- It is part of the epic's **own** block, so editing it re-asks that epic and nothing else. Where
  the epic appears as another epic's dependency, in that epic's hash and context, it appears without
  its description: a dependent relies on what the dependency delivers, which is its acceptance.
- An epic without a description renders the same block bytes as in 1.9.0, so its hash does not move
  on upgrade.
- The status-blind `tp epic scope` fingerprint excludes it: a description is not scope.
- The asked epic's description is shown in full; context epics get none. A prompt that grows too
  large is answered with a smaller slice (`--epics … --name`), not a cap.
- `dod-satisfiability`, `why-auditor` and `phase-economist` read it as the primary text of the epic
  under review: whether the acceptance satisfies its purpose and scope, whether the edges' whys
  agree with its decisions, whether the placement fits its purpose. Every such sentence is
  conditional on the epic having one. The acceptance stays the contract the gate checks; a
  contradiction between a description and an acceptance or an edge is a finding. A missing
  description is not a finding, and an open question a description lists is not reported back.
- `gate-scope` does not read it in this release; its prompt, hash and cost stay as in 1.9.0.
- `tp epic export` carries it in the raw graph (additive), and the dashboard's epic detail shows it
  as preformatted text, escaped and never rendered as markup.

**Consequences.** Every description edit is a round against the slice's cap. A corpus ejected
before 1.11.0 gets the text in its blocks and none of the new instructions. What belongs in a
description is intent, scope, non-goals, decisions and open questions; behaviour written there is
re-reviewed on every edit and ages as unbuilt spec text does.

**Prompt size.** Every epic may eventually carry a description, so a slice's epic-role prompts grow
by description length times asked epics. The cost is shown, not capped: a slice's `--status` reports
the bytes of the blocks a round would ask now, a proxy available before anything is emitted, and the
emission envelope reports each role's real prompt bytes. Before matters, because an emitted round
is frozen and taking it back is the operator's `--discard`. The peak is the first pass: describing a
whole milestone re-asks every epic of it at once, so the guideline is to add descriptions a set of
epics at a time and review that set as its own slice (`--epics … --name`). No size threshold and no
description-length lint until a field run shows agents failing on size.

**Release criterion, written before its field run.** A clean run on real epics whose descriptions
were written before any role output, judged by the operator's accepts and rejects. Descriptions that
already disagree with their acceptance are kept unreconciled for the run, with the disagreements
recorded beforehand and held back, so the run measures whether the roles catch them. The run also
reports whether a role agent read the whole prompt on a real slice at those description lengths.

## 2. Alternatives

- **A spec or unbound draft per epic.** Declined by the operator: the description belongs with the
  epic in the graph.
- **Longer acceptance.** Rejected: the acceptance is the gate's contract and every role pays for it.
- **A field no role reads.** Rejected: it would sit beside the acceptance and could contradict it
  unnoticed.
- **Descriptions in the dependents' hash and context, or a first paragraph for context epics.**
  Rejected: one prose edit would re-ask every dependent, and a first paragraph is whatever the author
  wrote first.
- **`gate-scope` reading descriptions.** Deferred by the operator: its missing-work check maps
  obligations to acceptances, and its value can be tried in the field first.

## 3. Non-goals

- A lint rule on descriptions, including id matching.
- Rendering markdown in the dashboard.
