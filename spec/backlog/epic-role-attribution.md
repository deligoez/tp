# tp — Attributing epic review rows to their role

A backlog decision candidate, named by slug; its priority and release number are assigned later.

Class: **decision**.

## 1. The decision

**Context.** `tp epic review --record` takes one concatenated file, and the finding contract asks
for no `role`, so rows written by role sub-agents count as `unattributed` in `open_by_role`. The
tool knows which role each prompt went to; the model does not need to echo it.

**Decision (candidate).** Stamp `role` from the source: record from the per-role inbox files
(`<state>/inbox/<role>-r<N>.ndjson`), or accept one file per role, so attribution comes from the
tool and not from the agent.

## 2. Alternatives

- **Ask each prompt to write its role id.** Rejected as the first choice: it asks the agent for
  something the tool already knows.

## 3. Non-goals

- Changing the finding contract's other fields.
