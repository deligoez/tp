# a-finding-can-leave-an-audit-round — measurements

Supplemental material for `a-finding-can-leave-an-audit-round.md`; the spec stands without it. **This
file is not a spec and `tp ground` never grades it.** It exists because the grading of the spec it
was split from measured what the forensics cost: 149 floor units over 376 lines, one unit per 2.5
lines, and a round that spent 718k tokens across three graders. Counting the graded rows by `kind`
says where the cost went and where the value did:

| `kind` | units | findings | rate |
|---|---|---|---|
| `mechanism` | 16 | 8 | **50%** |
| `corpus` | 22 | 8 | 36% |
| `behaviour` | 39 | 10 | 26% |
| `document` | 29 | 7 | 24% |
| `code-structure` | 16 | 2 | 12% |

Counting rule: the 125 rows recorded in
`spec/backlog/.tp-review/a-finding-can-leave-an-audit-round/ground-round-2.ndjson`; a *finding* is a
row whose verdict is neither `PASS` nor `NOT-A-CLAIM`.

`mechanism` — a design or universal claim — is 13% of the graded rows and 22% of the findings.
`corpus` — a figure re-derived from the tree — matches it on rate and not on worth: *"four days later"
was two days*, *"eight of ten" does not reach eight by its stated derivation*. Both are correct
findings about sentences that, unwritten, would have offered nothing to find. **So the expense is not
the apparatus; it is the number inside the apparatus,** because every figure obliges a grader to run a
command. That is why the material below sits here and the spec keeps decisions.

---

## The accepted finding that changed nothing

Measured on a built fixture outside the repository — a two-section `spec.md`, `tp init spec.md`, and a
one-row results file:

```json
{"item_id":"i1","status":"FAIL","severity":"warning","role":"go-safety",
 "resolved":{"status":"wontfix","evidence":"accepted, named next version",
             "resolved_at":"2026-09-02T00:00:00Z"}}
```

`tp audit spec.md --record results.ndjson` prints:

```
round: 1   clean: false   consecutive_clean: 0   converged: false   findings: 1
```

Re-recording the identical row as rounds 2, 3 and 4 leaves `consecutive_clean` at 0 every time. The
mechanism is `internal/engine/auditclean.go`'s `AuditRowsClean`, which consults `AuditRowIsPass` and
the row's severity and **never reads the `resolved` key at all**. Substituting `duplicate` for
`wontfix` gives the same result, for the same reason.

**An accepted finding gates permanently under the default `audit_converge_on: all`.** Nothing a
recorder can write into the row changes that, because the field that records the acceptance is not
read.

**The row stays in the round and stays visible.** It appears in `role_streaks`' open count and in
`tp audit --merge`, both confirmed at HEAD: recording the fixture gives
`role_streaks: [{role: go-safety, consecutive_clean: 0, open: 1}]`, and `tp audit --merge` emits the
row with its `resolved` block intact. (`--report` is a `tp review` flag only — `tp audit --help` does
not list it.)

## What the shipped severity policy already reaches, and what it does not

**One escape exists today and destroys nothing** — and it is the fixture above that shows it. Its
severity is `warning`, which `auditclean.go`'s `advisoryAuditSeverities` classes advisory, so under
the field v0.37.0 shipped:

```
tp set --workflow audit_converge_on=blocking
tp audit spec.md --record results.ndjson    →  round: 1  clean: true  consecutive_clean: 1  findings: 1
```

The row is still recorded, still counted as a finding, and the round is clean.

**It does not reach the case the spec is about.** The same fixture with `severity: "error"`, under
`audit_converge_on=blocking`, records `clean: false`, `consecutive_clean: 0`. So an accepted **`error`**
row — and any row whose severity tp cannot grade, which `auditclean.go` blocks by design — has no exit
that leaves the record intact.

No exhaustiveness claim is made about the ways out. An earlier draft said *"the only ways out are to
stop recording it or to record it as `PASS`"*, which the fixture above falsifies — `blocking` is a
third, and it destroys nothing.

**No retroactive re-grading, measured.** `clean` is stamped at record time and stays stamped: four
rounds recorded under `all` still read `clean: false` after `tp set --workflow
audit_converge_on=blocking`, although the same rows recorded fresh under `blocking` grade clean.

## The third design, found twice

§2.1 of the spec once claimed *"there is no design that both keeps §2's reason and adds no field."*
**Two independent designs refuted it**, and recording both is the point: one came from a peer session
asked to try, the other from the grading unit whose brief told it to construct a counter-example. n=2.

**Design A — re-stamp at `--resolve`.** After writing the disposition into the recorded round file,
`--resolve` re-stamps that round's `state.json` entry under one rule: if every non-`PASS` row is
disposed (`wontfix`/`duplicate` with evidence) then `Clean=true`, otherwise the stamp stands. No reader
changes, no policy is read, no field is added. `spec/0.37.0.md` §2 already anticipates an operator
write changing the stamp — *"An operator who wants it undone re-records the round."*

**Design B — recover the policy from the stamp.** Because `all` implies `blocking`, exactly three
cases arise over the recorded rows: no non-`PASS` row (clean under both); the two policies disagreeing
(so the stored `clean` identifies which was in force, uniquely); both false (unrecoverable, so grade
survivors under `all`, the stricter candidate). Built and run: on the spec's own case the probe reports
`consecutive_clean 1`, `open 0` where the `HEAD` binary reports zeros and `open 1`; a round recorded
under `all` and then read with `blocking` adopted stays at 0, and one recorded under `blocking` then
relaxed stays at 1.

**Both are conservative in one enumerable case**, which is what the spec now states instead of the
universal. Design B's: a round stamped not-clean where both policies said not-clean, whose disposals
remove every blocking row but leave a non-blocking one, reports not-clean where `blocking` would say
clean. Unreachable under the built-in default, since `audit_converge_on` defaults to `all` and
recovery is exact there. Design A's: under `blocking`, a partial acceptance does not clear; the exit
is `--resolve-all` with a shared justification.

## Why the reversal is not a gap being filled

`spec/0.37.0.md` §2 does not merely stamp `clean`; it names `--resolve` in the alternative it rejects,
and it writes: *"Surviving is deliberately not the word: it would imply a disposition that removes a
row from the count, which §5 declines to create."* So this is not a gap being filled. What justifies
reversing it is that §2's **argument** — 67 non-`PASS` rows across v0.29.0–v0.32.0 carrying severities
outside the enum, which a live re-grade would make blocking on install — is about a *policy switch*,
not about a disposition. A disposition removes a row the operator wrote evidence for; it cannot
retroactively re-grade a round nobody touched.

**And it reverses a documented decision, not a bug.** `skills/tp/SKILL.md` states the behaviour and
its reason, in the Workflow D step that describes `--resolve` — cited by its anchor phrase, because
the line number moved twice on 2026-09-03:

> **A disposition is not an escape hatch from the gate:** `tp audit --record` counts every row whose
> `status` is not exactly `PASS` and reads no disposition at all, so parking a finding leaves the
> round's finding count, the role streak and `--status --check` exactly where they were.

A second passage, in the `divergence` paragraph, carries the policy half rather than repeating the
mechanism — *accepting findings outside spec conformance is a user-approved decision, never the
agent's.* **The fear is correct**: a disposition an agent can write is a way for an agent to make its
own findings stop counting — which is why this release carries the write fence itself (§3), rather
than leaving it to the sibling spec it was drafted in. Both `SKILL.md` passages are rewritten by the release, identified by their
anchor phrases and not by line number.

## The third exit

"The only exits are repairing it or destroying the record" is false: recording the **same rows** again
under a different `audit_converge_on` moves the verdict. Measured — round 1 `clean False`, set
`blocking`, re-record → round 2 `clean True`, round 1 intact. Three limits make it sharper rather than
weaker: it reaches only non-`error` severities, `blocking` is human-only under `TP_UNATTENDED`, and it
costs a round of budget. **It also moved `clean` while leaving `role_streaks` at `{0, open 1}`** — the
two-predicate split `a-findings-exits-agree.md` §5 closes, visible on a live tree.

## Decided at the 2026-09-08 decision pass

Two entries of `spec/undecided.md` were decided onto this spec, which ships the stops-blocking half
of the same subject.

**From *A durable home for an accepted finding*.** Decided: a repository-level
**`.tp/accepted.ndjson`**, appended by the audit resolve that accepts, surfaced by `tp resume` and
`tp status` as **`accepted_open`** until a task file's `covered_by` names the finding id. Of the three
target shapes named and never chosen, this is the one that satisfies the property already agreed —
readable by the next cycle's decomposition **without a human remembering it exists**. The three
options as they survive are in `spec/undecided-measurements.md` §From the rows spec, from
`git show 3a83be30:spec/0.41.0.md` §2.

**From *An audit-side `nonblocking_open`*.** Decided: **emit it**, and invert the guards that pin the
key's absence — **under `audit_converge_on: blocking` only**. Under that setting a clean round can
carry `warning` and `info` rows, so the audit phase has the accepted-open state the review-side field
was built to make visible; the count exists and only the breakdown is missing.
`engine.RoleStreak`'s `Open` (`internal/engine/rolestreaks.go`) is severity-blind and reaches the
payloads through `auditSignalFields` in `internal/cli/audit_record.go`. Four places pin the key's
absence by name and a fifth states it in a comment; those five are what the decision inverts, and they
are cited by symbol or phrase in `spec/undecided-measurements.md` §From the rows spec.

## A `wontfix` with no evidence

Moved here with §3 from `a-findings-exits-agree-measurements.md`, where it was measured.

Measured at `5058fc99` on a fresh one-finding recorded round, `tp review <round file> --resolve 0
wontfix` with no evidence argument prints `resolved finding 0 as wontfix`, exits **0**, and writes
`"evidence": ""` into the row. `reviewFindingResolvedAway` then requires non-empty evidence, so the
row stays in the surviving set and `consecutive_clean` stays at 0: accepted, reported as resolved, and
silently ignored — the failure mode the operator cannot see.

## The fence that does not exist

Moved here with §3 from `a-findings-exits-agree-measurements.md`, where it was measured.

An earlier draft of the rows spec argued that the agent-safety property survives because *"recording
a disposition is a user-approved decision under `TP_UNATTENDED=1`"*. **That fence does not exist.**
Measured at HEAD:

- a search for `Unattended` in `internal/cli/audit_resolve.go` returns **0** matches; the call sites
  of `engine.Unattended()` in `internal/cli` are `unattended.go`, `config_extract.go`, `set.go`,
  `set_project.go`, `set_local.go`, `importcmd.go`, `done.go` and `close.go` — `audit_resolve.go` is
  not among them;
- `TP_UNATTENDED=1 tp audit raw.ndjson --resolve 0 wontfix "accepted for now"` exits **0** and writes
  `resolved.status: "wontfix"` into the file;
- `TP_UNATTENDED=1 tp audit raw.ndjson --resolve 0 wontfix ""` also exits **0**, writing
  `resolved.evidence: ""`.

So an unattended agent can today write the exact row §2 would make non-gating, with an empty reason.
The `Unattended` search was repeated at the head this file was split at and still returns 0.
