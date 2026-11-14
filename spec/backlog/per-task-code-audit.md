# tp — A task's code is audited once, soon after it closes

A backlog decision drafted on 2026-11-14 with the tp-smart session, after the operator chose the
narrow form: go-safety only. Class: **decision** (the operator confirms Decision or Full after
reading; see §6). The measurements are in `spec/1.21.0-measurements.md` and
`spec/1.21.1-measurements.md`.

## 1. Context

- The release audit's code roles read the ten files with the most churn. In tp's history half of
  their findings, and 21 of 43 FAILs, sit on files beyond that cut, and a listed file's yield does
  not follow its rank.
- A trial gave a role the whole 121-file pool in one prompt. It graded every file and found a
  fraction of what ten-file prompts found on the same files. 8 of 10 of the control's extra findings
  were real, and both whole-pool runs missed the historical FAIL. Depth needs small prompts.
- Reading a file in full costs about 13k tokens however the pool is divided, against about 2k when
  skimmed. An agent's fixed cost also counts: a ten-file control run took 110–148k tokens.
- In tp's 558 done tasks a task's auditable files are few: median 2, mean 3, 90th percentile 6, and 7
  tasks above 10. A release like 1.8.0 closes 57 tasks.

## 2. Decision

1. A task's code is audited by the roles a new workflow field lists, default empty; tp's own config
   lists `go-safety`. Only code roles may be listed, never spec-coverage.
2. It is a unit, not a step inside `tp done`. tp emits the prompt, and an agent that did not write
   the code runs it. The oracle schedules it, concurrent like other read-only units, for each done
   task that has no recorded task audit.
3. The pool is the task's own commit files under today's filters, in prompts of at most ten files,
   with the same prompt and item ids as the release audit's file check. Tasks closed together may
   share a prompt up to ten files, so the agent's fixed cost is paid per ten files, not per task.
4. A finding does not block the close. It is recorded as an audit row under the same `(role,
   item_id)` identity, and the v1.18.0 rule applies: a blocking row holds audit convergence until a
   later grading passes it or the operator accepts it. The existing fix unit takes it before further
   implement units that touch the same file.
5. The rows live in the spec's state directory as the task audit record, `task-audit-<n>.ndjson`
   beside the round files, and the first release audit round lists them as prior rows in its carried
   set. Presence: for tasks closed after the field is set, the audit does not converge while a done
   task has no recorded task audit. Older tasks are exempt and listed.
6. The release audit is unchanged for now. Its code roles stay capped, and its carried set holds the
   unresolved task-audit rows. Whether the capped release go-safety role is still worth running is
   decided from the first release's overlap.
7. The field is project config, set like any workflow field. Under `tp run` an agent does not change
   it. Accepting a task-audit finding without a change is `accept-finding`, as for any audit finding.

## 3. Cost

- Shared prompts (§2.3): about one prompt per ten changed files.
  - For 1.8.0's 57 tasks touching about 170 auditable files: some 17 prompts, about 2–2.5M tokens
    for go-safety alone, against about 130k for today's capped role.
  - One prompt per task would be 57 prompts, several million.
- The field run (§6) measures the real figure before the tag.

## 4. Alternatives

- **Blocking the close.** It fixes while fresh, but adds a fix loop and a cap per task. Revisit if
  findings pile up unfixed.
- **A step in the implementer's brief.** It is not independent; the trial's depth came from fresh
  agents.
- **Chunking the release pool.** It costs the same tokens, and the findings arrive all at once at the
  end.
- **Raising the cap.** The trial: a long list is skimmed.

## 5. Non-goals

- Other roles per task in tp's own config.
- A diff-only prompt.
- Changing the release audit's cut.
- Cutting or chunking a named list.
- Auditing interactions between tasks.

## 6. Track and field run

A new unit kind, an oracle change, new state, a convergence rule and a config field make this large.
The operator confirms Decision (built in a clone first) or Full (the tp loop). Either way, a field
run with real agents on a real task file comes before the tag, with its criterion written first:

- findings per task audit;
- a labelled sample of real against not real;
- tokens per file and per prompt;
- what the release audit's rounds do afterwards.
