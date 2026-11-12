# tp — Attributing epic review rows to their role

A backlog decision candidate, named by slug; its priority and release number are assigned later.

Class: **decision**.

## 1. The decision

**Context.** `tp epic review --record` takes one concatenated file, and the finding contract asks
for no `role`, so rows written by role sub-agents count as `unattributed` in `open_by_role`. The
tool knows which role each prompt went to; the model does not need to echo it.

**Decision.** A new flag, `tp epic review <slice> --record-inbox`, records the round from the files
its emission named, one per emitted role, read from the emission snapshot. Each row is stamped with
that file's role, and a row naming a different role is refused; a missing file is refused, naming
the role, since the contract asks a role that found nothing for an empty file. The files are
recorded in panel order through the same path the `tp run` driver merges its role parts by. The
slice's `next_action` and SKILL's step name the flag. `--record <file>` stays for hand-made rows.

**Consequences.** Recorded rows carry their role, so `open_by_role` stops counting `unattributed`
for inbox rounds, and a role that never ran is caught on this path. It is not caught on
`--record <file>`, so BUGS.md's entry on roles that never ran stays open for that path. A discarded
emission's files are removed with it (`spec/backlog/epic-reemit-guard.md`), so a new round never
records rows written against old prompts.

## 2. Alternatives

- **Ask each prompt to write its role id.** Rejected: it asks the agent for something the tool
  already knows.
- **`--record` with an optional value, or a positional mode.** Rejected: one flag with one meaning.

## 3. Non-goals

- Changing the finding contract's other fields.
