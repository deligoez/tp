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

**Decision (candidate).** A warning, `deferrable-required`, on every deferrable of a gate that the
gate's required set holds: what the not-done members reach through unsatisfied plain edges, the set
`tp epic scope` already computes. A deferrable reached only through an any-of alternative is not
reported, since choosing the other option leaves it free.

## 2. Alternatives

- **Leave it to `tp epic scope`.** The fact is already there, but a lint warning reaches every
  graph without a scope run.
- **An error.** Rejected: the operator may keep the list on purpose while the plan moves.

## 3. Non-goals

- Changing `deferrable` itself or what a gate's green means.
