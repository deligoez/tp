# tp — `--reconcile`

Class: tool

> **This file is decisions.** The motivating instance is dated and measured from inside this
> repository's own v0.35.0 cycle (§1.1): an operator reached a fork where the correct action and the
> cheap mechanical one pointed in opposite directions, and took the cheap one.

## 1. Overview

A round records the hash of the text it read. When the spec is then repaired — which is what a review
round is *for* — the round reads stale, and **every mechanical way to clear that costs the record
something.** Overwriting the hash a round read destroys it. The cheaper exit destroys nothing and is
worse for a different reason: it **fabricates a round.**

Measured on a fixture outside this repository, against the freshly built binary. Record a clean round 1,
edit the spec, and `tp review spec.md --status` reports `stale: true`. Emit a further round and
`--record` an **empty** findings file: `stale` goes to `false`, `converged` to `true`,
`--status --check` exits **0**, and round 1's entry is **byte-identical** — `state.json` only gained a
row. The round that bought all that is one nobody ran: `findings: 0`, `clean: true`, from a zero-byte
file.

**This repository did exactly that.** Commit `52290c32` — *"chore(tp): record the round that cleared
the staleness"*, 2026-08-13 — adds `spec/.tp-review/0.35.0/review-round-21.ndjson` at **zero bytes**
and appends one `clean: true` entry (`git show --stat 52290c32`, and
`git cat-file -s 52290c32:spec/.tp-review/0.35.0/review-round-21.ndjson` returns `0`). That is the
v0.35.0 cycle clearing its own staleness a week before the episode §1.1 describes, by the cheap exit,
and it is the strongest argument this release has: **what is missing is not a way to clear staleness
but an honest one.**

**The invariant this protects, stated as narrowly as it holds: a recorded round's entry in `state.json`
is a frozen fact.** It is *not* true of the round's recorded findings file, which tp rewrites in place
through `tp review --resolve` / `--resolve-all` (`internal/cli/review_resolve.go`), whose `--force`
overwrites a disposition already there; how many recorded rows carry such a post-hoc `resolved` object
is derived in `reconcile-measurements.md` under "The rows the invariant does not cover". This release
stores its rows in `state.json`, which is the half that holds. Any surface that makes destroying or
fabricating a recorded round the path of least resistance is a defect in tp, not in the operator.

`tp review <spec> --reconcile --note "<text>"` records a reconciliation entry. The hash the round read
is preserved; the note stating why the spec moved is added as **its own row**, overwriting nothing.
Uncounted, gating nothing — accounting, not convergence.

With the emit-time hash release (`spec/backlog/round-records-the-text-it-read.md`) shipped, "the hash
the round read" is exact; without it, it is the record-time hash, and the row says which.

### 1.1 The instance

**This repository's v0.35.0 cycle.** Mid-implementation, verifying the converged spec against shipped
code found gaps; the spec was repaired, and the repair went through the uncounted regression pass, which
found more. **The two counts are recorded nowhere and are stated here as unrecorded** — an uncounted
pass writes no artifact by design, and `git grep` for either returns this file alone. The dating is
recorded and does check out: review round 21 was recorded 2026-08-13, five commits land on
`spec/0.35.0.md` on 2026-08-20 with two task-file commits beside them, bracketed by task closures, and
audit round 1 opened 2026-08-30 against a different hash
(`git log --date=iso-strict --format='%h %ad %s' -- spec/0.35.0.md`). The blocker then had no honest
exit — `--reconcile` does not exist (`tp review --help` lists `--merge`, `--resolve`/`--resolve-all`,
`--verify` and `--report`), and the two exits that do exist are the ones §1 names.

That cycle proceeded to audit with the blocker standing; the reasoning, and what a re-review would
have cost instead, are in the sidecar under "Why that cycle proceeded to audit".

**What was missing was not a way to re-review. It was a way to record why the spec moved without lying
— about what the rounds read, or about a round having happened.**

## 2. The entry

`--reconcile` appends a reconciliation row to the spec's state, carrying the note, the round it
reconciles, the hash that round read, the hash now, and a timestamp.

**It is a row, not a field on the round.** A field would be one note per round and would invite the
next repair to overwrite it — the same shape one level down. Rows accumulate, so a spec repaired three
times carries three.

**The rows are a typed field on the state**: a `reconciliations` list on `engine.ReviewState`
(`internal/engine/reviewstate.go:49-53`), marshalled by `SaveReviewState` (`:275`) like the two round
lists beside it, so they need neither an unknown-key round-trip nor the emit-time hash release. A
binary older than this release saving the file would drop the list — a downgrade hazard, not a design
blocker.

**`--note` is required and must be non-empty.** A reconciliation with no stated reason records that
something changed, which the hashes already say. The note is the entire contribution.

**It records; it does not clear.** Staleness stays true — the spec *has* moved. `--reconcile` makes
the movement explicable, not invisible. An operator reading a stale round now finds out why beside it.

**Uncounted, and it touches no counter.** Not a round, not a clean streak, no effect on `--check`, no
exit code beyond usage errors.

**A `TP_ROUND` re-record never touches the list.** Recording is idempotent on `TP_ROUND`
(`engine.RecordTargetRound`, `internal/engine/recordround.go`): a record unit that retries rewrites
its own round's entry in place rather than appending one. That rewrite replaces one element of
`review_rounds` and nothing else; the reconciliation list before and after it is byte-identical.

### 2.1 A re-emission that changes the text is counted

When `tp review` or `tp audit` writes a snapshot for a round that already has one, and the bytes
differ, the round's entry records that it happened: **`spec_moved_mid_round`**, a count of such
re-emissions, absent on rounds where it never occurred. The signal lives here because this release is
its consumer — a reconciliation row is the honest account of a spec that moved, and the counter says
which rounds moved under it. Counted rather than flagged: a boolean cannot distinguish one repair from
six, and the count costs the same. Nothing warns, blocks, resets a streak or recommends on it.

## 3. How it composes with the streak reset

The release that resets the audit streak when the spec hash changes makes the loop **re-earn** its
streak; this one **explains** the movement for a reader, preserving the hash the round read.

**Neither substitutes for the other and neither is a prerequisite.** A reset with no explanation tells
an operator to redo work without saying why; an explanation with no reset lets a stale claim stand. Two
different readers — the loop and the person.

## 4. Non-Goals

1. **No overwrite of anything, ever.** Not the hash, not the findings, not a prior reconciliation.
   That is the defect, not the feature.
2. **No convergence effect.** It does not clear staleness, reset a streak, advance a round or change
   an exit code.
3. **No automatic reconciliation.** tp does not infer why a spec moved; the note is the operator's.
4. **No `--reconcile` on the audit phase in this release — a scope cut, not an absence of instances.**
   `ReviewRound` is the type of both `review_rounds` and `audit_rounds`
   (`internal/engine/reviewstate.go:17`), so an audit round carries a `spec_hash` and goes stale under
   the same rule; and `spec-stale` fires **only** at `PhaseImplement` or `PhaseAudit`
   (`internal/engine/resumeblockers.go:94`), never during review — so even §1.1's review-round instance
   was met on the audit side of the loop. v0.35.0's own audit went stale repeatedly across its rounds
   (derivation in the sidecar under "The audit side's instance"), so the release that takes the audit
   side inherits a measured instance rather than a guessed shape.
5. **No repair of rounds already cleared by overwriting.** Those records are gone; this stops the next
   one.
6. **No gate, warning or `next_action` on `spec_moved_mid_round`.** §2.1 produces the number and stops.

## 5. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | after `--reconcile`, the reconciled round's `spec_hash` is **byte-identical** to what it was before | write the current hash onto the round, which is exactly the overwrite this release exists to prevent |
| 2 | §2 *rows* | three reconciliations of one spec produce three rows, none replacing another | store it as a field on the round, keeping only the last |
| 3 | §2 *note* | an empty or missing `--note` is a usage error | accept it, recording that something changed — which the two hashes already say |
| 4 | §2 *uncounted* | round count, clean streak, `converged` and `--status --check`'s exit are identical before and after | let it touch a counter, making a note a way to advance the loop |
| 5 | §2 *staleness* | the spec still reads stale after reconciling | clear staleness, which hides a real movement behind an explanation of it |
| 6 | §1 | the recorded row names both hashes — the one the round read and the one now — and they differ | record one, leaving a reader unable to see what moved |
| 7 | §2 *re-record* | with two reconciliation rows recorded, a `--record` under `TP_ROUND=N` for an already-recorded `N` rewrites round `N`'s entry and leaves the `reconciliations` list byte-identical | the rewrite path truncates the list — rebuilding the state from the round lists alone, which is what a rewrite that reconstructs `ReviewState` from scratch does |
| 8 | §2.1 | emit, edit, emit, record — `spec_moved_mid_round` is 1; a third differing emission makes it 2; a round emitted once carries no key | store a boolean, which reports six repairs in one round identically to one |

**Row 1 is the acceptance and row 5 is the one an implementer will want to skip.** Making staleness go
away is the operator's felt need in §1.1's instance, where it is measured: commit `52290c32` cleared it
with a zero-byte round. Satisfying that need directly is how this release would become the defect it
was written against.

**Row 7 is the ground round-2 FAIL.** The re-record path already exists and already rewrites a round
entry; a list that lives beside the round lists is one `st = &ReviewState{...}` away from being dropped
by a retry, so the row pins the retry rather than the happy path.
