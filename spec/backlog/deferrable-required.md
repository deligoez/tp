# tp — A deferrable that a required epic needs

A backlog decision candidate, named by slug; its priority and release number are assigned later.
It came out of a field run of `tp epic scope` (`spec/backlog/epic-scope.md`); the field graph's
ids and counts stay out of tp.

Class: **decision**. One new lint warning.

## 1. The decision

**Context.** A gate's `deferrable` list says which of its milestone's epics may wait. When a member
reaches a deferrable through plain edges, the deferral changes nothing: the gate cannot open
without it. `tp epic scope <m>` reports such epics as `deferrable_in_closure`, but only for a reader
who runs it, and no lint rule says it. The existing `gate-member-deferrable` catches an epic listed
in both `members` and `deferrable`, not one a member depends on.

**Decision.** A warning, `deferrable-required`, on every deferrable of a gate that a member reaches
through plain edges, naming the deferrable, the gate and one member path to it (`m -> x -> d`).
Like every other epic lint rule it reads the graph alone: "not done" is the graph's own `done`
field, and an epic already listed as both member and deferrable is left to `gate-member-deferrable`.
A deferrable reached only through an any-of alternative is not reported, since choosing the other
option leaves it free. `tp epic lint` still exits 0 on it, and README's lint table gains its row.

**Consequences.** Graphs that hold the case gain a warning on their next lint. The rule and scope's
`deferrable_in_closure` can differ once work is done: scope derives done from bound task files as
well, lint from the graph file only, so a deferrable whose requiring work is finished in a task file
but not yet closed in the graph is still a lint warning and no longer a scope row.

## 2. Alternatives

- **Leave it to `tp epic scope`.** The fact is already there, but a lint warning reaches every
  graph without a scope run.
- **An error.** Rejected: the operator may keep the list on purpose while the plan moves.
- **Read derived states in lint for this rule.** Rejected: lint is a property of the graph file, and
  one rule reading bound files would make `tp epic lint` depend on task files.

## 3. Non-goals

- Changing `deferrable` itself or what a gate's green means.
