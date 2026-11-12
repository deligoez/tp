# tp — `tp run` drives every prompt a round emits

A backlog decision, decided by the operator; it replaces the BUGS.md entry "`tp run` never spawns a
regression unit".

Class: **decision**.

## 1. The decision

**Context.** A round emitted after a fix can carry a regression prompt, appended by a predicate at
emission rather than read from the role corpus. `tp run`'s oracle builds one unit per role of the
corpus, so under a run that prompt is emitted and nobody answers it; the round records and can
converge with the check unread. Which phases can carry it is checked in the code before the build
and named here then.

**Decision.** Under a run, every prompt a round's emission prints has a unit. Regression is today's
instance: when the emission carries it, the run spawns a unit of the same kind as that phase's
other role units, reading a role prompt and writing a role file. The oracle and the emission decide
"does this round carry regression" through one shared function, not two copies. The acceptance is a
test over the whole panel, so the next role appended by a predicate is caught the same way.

Two assumptions are checked before the build and written down: that the run's record step merges
role parts by pattern, so a regression part is picked up; and that the plugin's write hook grants a
role unit its part path by pattern, covering the regression id. If the hook grants per corpus role,
the change is plugin content and a version bump.

**Consequences.** Rounds after a fix gain one unit under a run, so more tokens and time, and a run
that used to converge may find a regression and take another round. The budget a role unit counts
against is named here once checked; raising any `run_max_*` stays the operator's decision.

## 2. Alternatives

- **Do not emit regression under a run.** Rejected: consistent, but it drops a check silently.
- **Make regression an ordinary corpus role.** Rejected: it would run on rounds with nothing to
  verify, the cost epic review removed in 1.11.0.

## 3. Non-goals

- A new unit kind or agent.
