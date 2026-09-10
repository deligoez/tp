# reconcile — measurements

Supplemental material for `reconcile.md`; the spec stands without it.

## Rewritten on 2026-09-11

The spec's subject changed from *`--reconcile`* to *a rewritten spec starts a new epoch*. The earlier
draft proposed `tp review <spec> --reconcile --note "<text>"`: an uncounted row beside a stale round,
which deliberately cleared nothing. The backlog survey of 2026-09-11 found it low value on three
counts — its one motivating instance was a single zero-byte round (below); `--harness-note` already
stores free text on a recorded round; and because the note did not clear staleness, a `spec-stale`
blocker naming it would have pointed at a command that does not clear the blocker. A field report
(WB-3155) then supplied the case the new subject answers. The earlier text is
`git show 18032abe:spec/backlog/reconcile.md`.

What moved with the rewrite: the earlier draft's §2.1 (`spec_moved_mid_round`, a count of re-emissions
that changed a round's snapshot) went to `emitting-does-not-lose-a-round`, which refuses that
re-emission instead of counting it. `round-records-the-text-it-read` §2 (the emit-time hash) came here
as §4; its §3 (a streak reset keyed on hash changes) was parked in `spec/undecided.md`. That spec's
sidecar keeps its corpus census, the fallback measurement and the overwrite measurement, unchanged.

The sections from *The rows the invariant does not cover* to the end are the earlier draft's and stay
as its history.

## Field report WB-3155, verified 2026-09-11

Every reproduction below ran against a binary built from `18032abe`, in a fresh git repository outside
this one: a two-section spec, `tp init`, one task added and committed, then two review rounds each
recorded with one `high` finding carrying a unique marker string in its text, then the spec replaced
by a document sharing no sentence with the first.

**#15 — "`spec-stale` has no reconciliation path."** The reporter: `tp resume` says *"the spec
changed after the last recorded review round; reconcile it before continuing"*, and names no command.
**Verdict: CONFIRMED.** On the fixture, `tp resume spec.md` exits 0 with `phase: implement`, a
`spec-stale` blocker of class `escalate` whose message is exactly that sentence and whose data is
`{"spec": "spec.md"}` alone, and `next_action.summary` *"claim the next ready task t1"*. Source:
`internal/engine/resumeblockers.go:93-100`, where the blocker is built with that message and a data map
holding only the spec path. The exit does exist: `tp review spec.md` then
`tp review spec.md --record <empty file>` leaves `consecutive_clean: 1`, `stale: false`, and the next
`tp resume` carries no `spec-stale` blocker. The reporter's sharper form — that a rewritten spec's
earlier rounds graded a document that no longer exists, and tp holds every round's `spec_hash` and
so could know — is #18.

**#18 — "no way to invalidate a rewritten spec's old rounds."** The reporter rewrote a spec after
eight review rounds, then ran `tp review <round file> --resolve-all wontfix "<evidence>"` once per
round file by hand, and proposed `tp review <spec> --void-rounds <evidence>`: mark every recorded
round's findings `wontfix`, clear `spec-stale`, leave a line on `--status`. **Verdict: CONFIRMED that
no verb exists; the proposal is rejected.** Measured on the fixture:

- **The old rounds do not block convergence.** One round emitted and recorded against the rewritten
  text gives `consecutive_clean: 1`, `stale: false` — the open findings of the earlier rounds break
  the streak and nothing more.
- **They do pollute the next prompt.** The round-3 emission's role prompts contain both markers under
  *"UNRESOLVED findings from previous rounds — DO NOT re-report:"*. Source: the carry-forward loop in
  `internal/cli/review.go` (lines 2007–2021 at `18032abe`) reads the rows of every recorded round;
  the header is at `review.go:1356`.
- **The reporter's workaround makes both worse.** On a copy of the fixture, `--resolve-all wontfix`
  into each of the two round files (exit 0 each) moves `consecutive_clean` from 0 to 2 before any new
  round. The round-3 emission still carries both markers, now as `[WONTFIX]` rows under the same
  header. After one clean round against the rewritten text: `consecutive_clean: 3`,
  `converged: true`, `tp review spec.md --status --check` exit **0** — convergence on a streak two of
  whose three rounds read the deleted document. The reporter saw the same jump (0 → 8) on eight
  rounds.

Not reproduced: the reporter's statement that the eighth round file needed `--force` because 43 of
its rows were already dispositioned `fixed`. It is consistent with `--force` being what overwrites an
existing disposition (*The rows the invariant does not cover*, below), and nothing above rests on it.

## Re-verified 2026-09-11

**The record-time hash clears `stale` for a round that never read the current text.** Fresh
repository, one-section spec, one round emitted and recorded clean; the spec rewritten (`stale: true`);
a second round emitted from that text; the spec edited again; the second round recorded. Result:
`stale: false`, `consecutive_clean: 2`, and the second round's recorded `spec_hash` equals the sha256
of the spec as last edited and not that of `snapshot-round-2.md`. This is the spec's §4 and row 10.

**A round with no snapshot.** A fresh repository, `tp init`, then `tp review spec.md --record` of an
empty file with no emission before it: exit 0, no snapshot file in the state directory, and the
recorded `spec_hash` equals the sha256 of `spec.md`. Row 12.

### A converged spec, rewritten

Fresh repository, one-section spec, two rounds emitted and recorded clean: `consecutive_clean: 2`,
`converged: true`. The spec replaced by a document sharing no sentence with it: `converged: false`,
`stale: true`. One more round emitted and recorded clean: `consecutive_clean: 3`, `converged: true`,
`--status --check` exit 0. Two of the three rounds in that streak read the replaced document. This is
Non-Goal 1's hole and row 5's `HEAD` value; the epoch leaves it as it is.

## The zero-byte round

Moved from the earlier draft's §1, where it was the motivating instance.

Measured on a fixture outside this repository: record a clean round 1, edit the spec, and
`tp review spec.md --status` reports `stale: true`. Emit a further round and `--record` an **empty**
findings file: `stale` goes to `false`, `converged` to `true`, `--status --check` exits **0**, and
round 1's entry is byte-identical — `state.json` only gained a row. The round that bought all that is
one nobody ran: `findings: 0`, `clean: true`, from a zero-byte file.

**This repository did exactly that.** Commit `52290c32` — *"chore(tp): record the round that cleared
the staleness"*, 2026-08-13 — adds `spec/.tp-review/0.35.0/review-round-21.ndjson` at **zero bytes**
and appends one `clean: true` entry (`git show --stat 52290c32`, and
`git cat-file -s 52290c32:spec/.tp-review/0.35.0/review-round-21.ndjson` returns `0`).

## The rows the invariant does not cover

The round's recorded findings file is rewritten in place by `tp review --resolve` / `--resolve-all`,
whose `--force` overwrites a disposition already there. 1,566 rows across 85 recorded round files
carried a post-hoc `resolved` object when this was written:

```
python3 -c "import json,glob;g=glob.glob('spec/.tp-review/*/review-round-*.ndjson')+glob.glob('spec/.tp-review/*/audit-round-*.ndjson');r=[(f,l) for f in g for l in open(f) if l.strip() and isinstance(json.loads(l).get('resolved'),dict)];print(len(r),len({f for f,_ in r}))"
```

## Why that cycle proceeded to audit

**That cycle proceeded to audit with the blocker standing, and the reasoning is worth keeping**: after
implementation is complete the audit subsumes what a re-review could ask, because `spec-coverage`
derives its checklist from the repaired text and tests the code against it. That the checklist came from
the *repaired* text is checkable — `shasum -a 256 spec/.tp-review/0.35.0/snapshot-audit-round-1.md`
equals the `spec_hash` that round recorded. The cost avoided was real, and it is structural rather than
anecdotal: `review_clean_rounds` resolves to 2 (`tp config --resolved`), a `fixed` disposition implies a
spec change and forces a re-review (`skills/tp/SKILL.md:19`), and the panel is five —
`python3 -c "import json,glob;print(sorted({json.loads(l)['role'] for f in glob.glob('spec/.tp-review/0.37.0/review-round-*.ndjson') for l in open(f) if l.strip() and json.loads(l).get('role')}))"`
returns architect, ax-economist, implementer, regression and tester. So settling a two-section repair
costs **at least ten role-rounds over the whole spec**. It is stated as a floor rather than a cycle's
round count because no review cycle in this repository has eleven rounds
(`python3 -c "import glob,collections;print(sorted(collections.Counter(f.split('/')[2] for f in glob.glob('spec/.tp-review/*/review-round-*.ndjson')).values()))"`)
— every `eleven rounds` in the tree names an **audit**, a different panel with a scope-blind
convergence cause rather than a localised spec repair.

## The audit side's instance

v0.35.0's own audit ran nine rounds over **seven distinct** `spec_hash` values:

```
python3 -c "import json;print([r['spec_hash'][:13] for r in json.load(open('spec/.tp-review/0.35.0/state.json'))['audit_rounds']])"
```

## Where the storage decision came from

An earlier draft made the unknown-key round-trip a hard prerequisite: `ReviewState` is a typed struct
and `SaveReviewState` marshals it, so a `reconciliations` array *injected* into `state.json` is gone
after the next `--record` — `go test ./internal/cli -count=1 -run
TestAGroundRoundLeavesAnExistingStateJSONByteIdentical` seeds a top-level key `ReviewState` does not
know, runs a record over it, and asserts the key is gone. That measurement is correct and the
conclusion drawn from it was not: a list that is a *typed field* of `ReviewState` survives every save
the current binary makes, and only a binary that predates the field drops it. The same holds for the
epoch list the rewritten spec records.
