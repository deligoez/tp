# A clean tree audits the task commits

Decision note (Decision track). Operator decision of 2026-11-24: option (a).

## Problem

Auto-detection reads the working tree, the index and, when the repository has a tag, `<tag>...HEAD`.
A project's first cycle has no tag, and implement ends with every task committed, so the audit's
own next step finds nothing: `tp resume`'s `next_action.command` is `tp audit <spec>`, `tp run`'s
audit-role units run `tp audit <spec> --role <r>`, and each exits 4 with `no changed files detected`.
The hint names `--affected-from-tasks`, which works, so an agent recovers, but every audit unit of
every round pays one failed call first (`BUGS.md`). The same happens on a tagged repository whose
HEAD is the tag.

## Decision

When auto-detection finds no changed file at all and no `--base` was given, `tp audit` falls back to
the files the done tasks' commits touched — the set `--affected-from-tasks` derives — and emits the
round instead of exiting 4.

1. The fallback set is treated as auto-detected, not named: it is capped at the auto-detect cap (the
   cut is printed and kept in the payload, as auto-detection's is) and each code role's list is cut
   to its usual size, so a large plan does not become one long prompt (the reason a named list is
   read shallowly).
2. Its churn is not measured: no comparison covers it, so no file carries a diff annotation, as under
   `--affected-from-tasks`.
3. The payload says so: a notice of kind `tasks-fallback` names why the fallback ran and how many
   files it audits, of how many when the cap cut them, and survives `--compact` like every notice.
4. When the done tasks yield no file either, the current exit 4 with `suggested_files: []` stands.
5. With `--base`, an empty range stays exit 4: the caller named the comparison.

## Alternatives

- **`brief_command` and `tp run` name `--affected-from-tasks`.** Fixes only the commands tp writes; a
  hand-typed `tp audit <spec>`, the command the skill teaches, still fails first. The named form is
  also uncut, so a large plan gets the long prompt the cut exists to avoid.
- **Keep the exit and document the rerun.** What v1.29.5 does; it costs one call per unit per round.

## Non-goals

- A tree with any change keeps today's selection, including the exit 4 when only skipped types
  (Markdown, task files) changed: the fallback runs only when auto-detection saw no change.
- A git failure that empties auto-detection (a shallow clone without the tag) is warned on stderr
  as before, and then falls back like an empty diff.
- `--affected-files` and `--affected-from-tasks` are unchanged.
- No ranking of the fallback set beyond what auto-detection does with unmeasured files (path order).
