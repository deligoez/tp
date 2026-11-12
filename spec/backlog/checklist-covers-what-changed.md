# tp — The audit's file list covers what changed, and says what it graded

A backlog decision for 1.15.0, cut down on 2026-11-13 to what is still missing at v1.12.0: the spec
excerpt's cut is reported since v1.2.1 and is gone from it; its prerequisite,
`audit-records-what-was-graded`, has shipped. The probes and the earlier draft are in
`checklist-covers-what-changed-measurements.md`. The operator delegated the night's decisions to the
agent, which took a second opinion from the tp-smart session.

Class: **decision**. A stopgap: ranking a capped list better does not audit the rest.

## 1. Context

`tp audit` hands each code-lens role a file list cut to a fixed cap in path order — the danger files
first, then the rest alphabetically — so on a large change the most-changed files can fall outside it.
A list the operator named with `--affected-files` or `--affected-from-tasks` is cut the same way,
while the payload's `truncated` reads false, because that flag describes only the auto-detect cap.
Nothing in a recorded round says how many files a role graded of how many it was given, and nothing
says when the audited universe itself looks wrong.

## 2. Decision

1. **Rank by churn.** The code roles' list is ordered by lines added plus deleted, descending, ties by
   path; files without a measured diff come last, in path order.
2. **A named list is not cut.** When `--affected-files` or `--affected-from-tasks` replaced the
   universe, the code roles get the whole list.
3. **Say so when anything is cut.** Each prompt entry in the payload carries the pool its list was
   cut from; a cut below the auto-detect cap prints a notice on stderr.
4. **The recorded round says what was graded.** At emission tp keeps each code role's pool size beside
   the round's snapshot; at `--record` it counts the distinct files each role graded, and `--record`,
   `--status` and `next_action` report "graded N of M" per role. The stored file is a round artifact
   the dirty-state classifier, `scripts/clean-emissions.sh` and the write fence know by name.
5. **Say so when the universe looks wrong.** A notice when files reach the range only through a merge
   (`--first-parent` sees fewer), and when the auto-detected universe and the files the plan's tasks
   touched differ.

## 3. Alternatives

- **Cover the whole surface in bounded prompts.** The real fix, and a separate subject (dividing a
  round by section); this is the stopgap.
- **Carry last round's open findings into file selection.** Withdrawn in the draft; see the sidecar.

## 4. Non-goals

- Raising or removing the caps.
- Changing the spec excerpt's cut (reported since v1.2.1).
