# tp v1.51.0 — `--reconcile`

> **This file is decisions.** Two instances motivate it: one from a field report, held by its author and
> unreachable from this tree, and one dated and measured from inside this repository's own v0.35.0
> cycle. Both operators reached the same fork — the correct action and the cheap mechanical one pointed
> in opposite directions.

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
overwrites a disposition already there — 1,566 rows across 85 recorded round files carry a post-hoc
`resolved` object today
(`python3 -c "import json,glob;g=glob.glob('spec/.tp-review/*/review-round-*.ndjson')+glob.glob('spec/.tp-review/*/audit-round-*.ndjson');r=[(f,l) for f in g for l in open(f) if l.strip() and isinstance(json.loads(l).get('resolved'),dict)];print(len(r),len({f for f,_ in r}))"`).
This release stores its rows in `state.json`, which is the half that holds. Any surface that makes
destroying or fabricating a recorded round the path of least resistance is a defect in tp, not in the
operator.

`tp review <spec> --reconcile --note "<text>"` records a reconciliation entry. The hash the round read
is preserved; the note stating why the spec moved is added as **its own row**, overwriting nothing.
Uncounted, gating nothing — accounting, not convergence.

**This release needs the emit-time hash.** Reconciling against a hash that was itself re-read at record
time explains a movement the record cannot locate; the release that pins the hash to the round's own
snapshot is what makes "the hash the round read" a true description.

### 1.1 Two instances

**A Rust NLP project — testimony, and nothing in this tree can check it.** After nine commits, every
one closing an item the rounds themselves had opened, the user hand-authored a `spec_hash_at_round`
field with a note beside it rather than corrupt the record. They built the feature by hand because the
tool offered only the lie. `git grep spec_hash_at_round` returns this file and no other, and the report
lives with its author and that project; every load-bearing particular is held there. It stays because it
is the only instance of an operator who reached the fork and **chose the record** — the second instance
is what happens when they do not.

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

**What was missing was not a way to re-review. It was a way to record why the spec moved without lying
— about what the rounds read, or about a round having happened.**

## 2. The entry

`--reconcile` appends a reconciliation row to the spec's state, carrying the note, the round it
reconciles, the hash that round read, the hash now, and a timestamp.

**It is a row, not a field on the round.** A field would be one note per round and would invite the
next repair to overwrite it — the same shape one level down. Rows accumulate, so a spec repaired three
times carries three.

**The rows depend on an unshipped prerequisite, and that is a dependency rather than a property they
already have.** Today an unknown key does not survive a write: `ReviewState` is three fields
(`internal/engine/reviewstate.go:49-53`) and `SaveReviewState` marshals that struct (`:275`), so a
`reconciliations` array injected into `state.json` is **gone after the next `--record`**. The shipped
control measures exactly this —
`go test ./internal/cli -count=1 -run TestAGroundRoundLeavesAnExistingStateJSONByteIdentical` seeds a
top-level key `ReviewState` does not know, runs a record over it, and asserts the key is gone
(`internal/cli/ground_state_untouched_test.go:101-110`). The unknown-key round-trip that fixes it is the
ship-signal release's §2.1 and is **unshipped**, so a binary older than that fix drops these rows however
this release stores them. A release whose premise is *a recorded fact is never overwritten* cannot ship
into a store that erases it, which is what makes the round-trip a hard prerequisite rather than a
convenience.

**`--note` is required and must be non-empty.** A reconciliation with no stated reason records that
something changed, which the hashes already say. The note is the entire contribution.

**It records; it does not clear.** Staleness stays true — the spec *has* moved. `--reconcile` makes
the movement explicable, not invisible. An operator reading a stale round now finds out why beside it.

**Uncounted, and it touches no counter.** Not a round, not a clean streak, no effect on `--check`, no
exit code beyond usage errors.

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
   (`internal/engine/reviewstate.go:15-23`), so an audit round carries a `spec_hash` and goes stale under
   the same rule; and `spec-stale` fires **only** at `PhaseImplement` or `PhaseAudit`
   (`internal/engine/resumeblockers.go:94`), never during review — so even §1.1's review-round instances
   were met on the audit side of the loop. v0.35.0's own audit ran nine rounds over **seven distinct**
   `spec_hash` values
   (`python3 -c "import json;print([r['spec_hash'][:13] for r in json.load(open('spec/.tp-review/0.35.0/state.json'))['audit_rounds']])"`),
   so the release that takes the audit side inherits a measured instance rather than a guessed shape.
5. **No repair of rounds already cleared by overwriting.** Those records are gone; this stops the next
   one.

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

**Row 1 is the acceptance and row 5 is the one an implementer will want to skip.** Making staleness go
away is the operator's felt need in **one** of §1.1's two instances — v0.35.0's, where it is measured:
commit `52290c32` cleared it with a zero-byte round. It is not the felt need in the other, where the
operator declined the clearance and hand-built the record instead. Satisfying the first is how this
release would become the defect it was written against.
