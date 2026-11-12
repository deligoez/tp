# tp — What a milestone's gate actually pulls in

A backlog decision candidate, named by slug; its priority and release number are assigned later.
It absorbs the earlier `tp epic path --to <m>` idea. Decided between two tp sessions at the
operator's request after a field project's proposal; the field graph's ids, counts and lists stay
out of tp, and every fixture this note leads to is synthetic.

Class: **decision**. Part A is one new read-only command, derived from the graph and writing
nothing. Part B is gated on a trial the field project runs first and has no behaviour text yet.

## 1. The decision

**Context.** Epic review grades slices: each role reads its units, and none is asked whether a
milestone's gate, taken whole, is the work its threshold needs. A field project found by hand that
a first milestone's gate closure held work no consumer needed until later, and that the threshold
leaned on an epic of a later milestone. What it needed was a measurement of the closure, the set of
epics a gate's effective members reach through `depends_on` and any-of groups. Part of it exists:
`later-milestone-dependency` and `dependency-opens-later` report closures that reach a later
milestone of the same track, and a milestone's own critical path is
`milestones[m].critical_path` in a non-compact `tp epic status --milestone m`. The rest does not.

**Decision, Part A — `tp epic scope <m>`.** One read-only command per milestone, runnable under
`TP_UNATTENDED`, that prints:

1. the gate's effective members, and `members: implicit` when `members` is unset or empty, which
   makes every non-deferrable epic of the milestone a member;
2. the closure, composed by milestone and track: the cross-track part that lint leaves free by
   design appears here as information, never as an error;
3. the milestone's epics outside the closure (empty by definition under implicit members, and said
   so rather than printed as `[]`);
4. the *removal shrink* of each closure epic whose removal takes at least one other epic with it:
   for an epic that is not a member, the epics that leave the closure when every edge into it is
   cut; for a member, the epics that leave when it is dropped from the gate, marked as a member
   since that row is a gate decision. Sorted by size, so the few epics that carry most of the
   closure come first, and an epic that takes only itself prints no row;
5. the epics outside the closure with no dependents at all;
6. epics an earlier milestone's gate holds that no later gate's closure contains, naming which later
   gates were implicit, since an implicit gate's closure contains everything;
7. the milestone's own critical path, by count and by estimate, repeated here so a reader needs one
   call;
8. a `fingerprint` over the member list, the closure's id set, the path's id sequence and the
   threshold text, so "unchanged since the last round" is a string compare and a rewritten
   threshold counts as a change even when the closure is identical.

**Decision, Part B — gated, not decided.** Whether graph review gains the second direction —
*what work does the threshold need that the closure lacks* — waits on a field trial: Part A's
measurement given by hand to a scope reviewer and, as the control, to `phase-economist` alone. Its
criterion was written before that round: how many of the questions the operator held back the
reviewer finds unaided, and how many of its findings the operator rejects.

The first half has reported, one run per arm. The scope reviewer raised the held-back questions the
criterion named. The control, `phase-economist` given the same measurement with the gate asked,
returned nothing. A third arm, the control plus the scope prompt's one-line purpose and nothing
else, returned findings — all on the asked epic units, about their placement, and none on the gate.
Read together: the purpose deepens a role along its own questions, and the gate-wide missing work
appears only when the second direction is asked of the gate as a whole. The third arm's findings
and the scope arm's did not overlap, so neither role replaces the other.

The operator has disposed of every row, per group. Most missing-work findings were accepted, one
of them narrowed; the rejections were one false positive the role had itself hedged and one
operator choice that later became moot. No surplus cut was rejected; some were superseded by an
accepted cut that removed their chain. Every one of the third arm's rows was accepted. Applying
them shrank the first milestone's closure and its critical path even with new members. The apply
also exposed two cases the measurement already names: a control epic left the closure with a chain
it hung from, which item 2 shows, and the set item 6 reports grew. In this trial one disposition
round moved the measurement, and a scope pass reruns after an apply because the fingerprint
changed.

The lean, now carried by the trial: a separate gate-only role owning both directions, with
`phase-economist`'s gate question moving to it, since the scope pass has its own trigger (a changed
fingerprint), its own input (Part A's output), and the third arm left the gate untouched even with
the purpose; and an optional `gates.<m>.purpose`, kept apart from the threshold and entering the
fingerprint, given to the gate-only role, which needs it, and to `phase-economist`, the one role
tested with it, whose purpose-fed rows were all accepted. For the other graph roles it is open:
giving it to all of them would rest on uniformity, one prompt line and no per-role switch, which is
a simplicity argument and not a trial result. All three are user-visible choices for the operator.
Until the operator makes them, no behaviour is written for Part B.

**Consequences.** An operator can ask what a gate pulls in, and which few epics to cut or split,
without running a review round. Review state, convergence and the role corpus do not change in
Part A. A project can trial its own scope role today: a populated `.tp/epic-reviewers/` replaces the
built-in graph roles.

## 2. Alternatives

- **Extend `tp epic status --milestone`.** Rejected: status is already the largest epic payload,
  and every agent reading it would pay for lists only a scope question needs.
- **Lint rules.** Rejected: items 3, 5 and 6 are information, and cross-track edges are free by
  design, so an error there contradicts the design.
- **Match epic ids in the threshold text.** Rejected: the threshold is prose, the first real use
  produced a false positive, and a warning that is wrong on purpose teaches agents to ignore
  warnings. The reviewer reads the threshold.
- **"Single dependent inside the closure" as the cut signal.** Rejected for removal shrink: on the
  field graph it flagged a large share of the closure, while shrink singled out the few epics the
  operator went on to decide about.
- **The second direction as a question inside `phase-economist`.** Leaning against (Part B): the
  scope pass runs on a changed fingerprint with Part A's output as input, which no other graph role
  needs, and two built-in roles asking "is the gate minimal" at one location is the overlap this
  list rejects.
- **A new finding state or the condition mechanism for operator-held findings.** Rejected: a
  condition is plan content and a finding is review state. Findings already wait: `tp epic apply` is a
  separate act, and accepting one without a change is the operator's decision. Routing a class to
  the operator is `.tp/routed-classes.json`.
- **Closure stability as the convergence rule.** Rejected as a replacement: a stable closure can be
  stably wrong. The fingerprint is offered as a signal beside clean rounds, and as the trigger for
  re-running a scope pass, since the measurement is deterministic and a second unchanged round
  only measures reviewer variance.
- **`tp epic path --to <m>` as the name.** Rejected: it promises a sequence and an estimate total,
  and the command returns a set and set differences; estimates are often unset.

## 3. Non-goals

- Moving epics or changing gate members: the command reports, it never writes.
- Any change to `later-milestone-dependency`, `dependency-opens-later` or the epic export contract.
- Judging whether the threshold itself is right.
