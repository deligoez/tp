# tp — A rewritten spec starts a new epoch

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `reconcile-measurements.md` beside it, and this file stands without them. The slug
is the file's earlier subject — a `--reconcile` note that cleared nothing — and stays because
citations use it; the subject changed on 2026-09-11. It absorbs the emit-time hash from
`round-records-the-text-it-read`, which is now a forwarding stub.

Class: **tool** — it changes what an emission carries from earlier rounds and which text a round's
recorded hash names; no convergence rule changes, and the one answer that moves — `stale`, and
`converged` with it — moves on a round recorded after the text it read was edited, which the existing
rule then reports correctly.

## 1. The decision

**Context.** When a spec is rewritten, its recorded rounds describe a document that no longer exists,
and tp has no way to say so. A field report (WB-3155) rewrote a spec after eight review rounds, and
three things it met reproduce at `HEAD` (sidecar, *Field report WB-3155, verified 2026-09-11*):

- `tp resume` raises the `spec-stale` escalation with a message that says *reconcile it* and names no
  command, while its `next_action` says to claim the next ready task. An exit exists: one review round
  emitted and recorded against the current text clears `stale`.
- The next review round's prompts carry every earlier round's findings under *"DO NOT re-report"* —
  every one of them about the old text.
- The only tool at hand makes both worse. Resolving each earlier round file `wontfix` leaves the rows
  in the prompt, now tagged `[WONTFIX]`, and turns those rounds clean — so one round against the new
  text converges, on a streak most of whose rounds read a document that no longer exists.

Underneath all three: a round's recorded hash is computed when the round is recorded, not when it was
emitted. A round emitted from the old text and recorded after the rewrite carries the new text's hash,
and clears `spec-stale` without having read the text it certifies.

**Decision.** Three changes:

1. The `spec-stale` blocker names what clears it (§2).
2. `tp review <spec> --rewritten "<evidence>"` starts an epoch: rounds recorded before it stop feeding
   any list of earlier findings an emission carries, in review and in audit; nothing recorded
   changes; `--status` reports the epoch (§3).
3. A round's `spec_hash` is the hash of its own snapshot — the text its prompts were emitted from (§4).

**Consequences.** Only a round that read the current text clears staleness. A rewrite costs one call
instead of one resolve per round file, and writes no disposition. No convergence rule changes; what a
rewrite of an already converged spec still permits is Non-Goal 1, with its register entry.

**Alternatives.**

- **Void the rounds** — the field report's proposal: one call that marks every earlier finding
  `wontfix`, clears `spec-stale` and leaves a note on `--status`. Rejected three times over. A
  `wontfix` asserts that someone read the finding and chose not to fix it, which is false for every
  row it would be written into, and it would stand in the record as that decision. Disposing the rows
  makes their rounds clean, which manufactures convergence — measured with `--resolve-all`, the
  hand-driven form of the same call. And clearing `spec-stale` without a round that read the new text
  is the fabricated-round exit the earlier draft of this spec was written against (sidecar, *The
  zero-byte round*).
- **The earlier draft's `tp review <spec> --reconcile --note "<text>"`**, an uncounted note beside a
  stale round. Dropped: it cleared nothing by design, so the blocker could not name it as its exit;
  `--harness-note` already stores free text on a recorded round; and its one motivating instance was a
  zero-byte round recorded to clear staleness — a round nobody ran — for which the answer is to name
  the honest exit, as §2 does, rather than a note beside the stale one. The note survives as the
  epoch's required evidence.

## 2. `spec-stale` names what clears it

**The decision: the blocker carries the command that clears it, as data and in its message** — emit a
review round with `tp review <spec>`, run its roles, and record their findings with
`tp review <spec> --record <file>`. It names `--rewritten` beside that, as the call to make first when
the spec was rewritten rather than edited: tp cannot tell an edit from a rewrite, and the operator can.

**When a round is already in flight, the blocker names both ways past it.** Once
`emitting-does-not-lose-a-round` ships, emitting over an unrecorded round whose spec has since changed
is refused, so the bare emission is not an exit then. The blocker names recording the in-flight round
and then emitting, or discarding it with that emission's `--force` — and says which of the two leaves
the in-flight round's grading on record.

**Only a round that read the current text clears it.** After §4, a round emitted before the edit and
recorded after it leaves `stale` true, so the named command is the one exit, and naming it is honest.
The epoch does not clear staleness: it records why the old rounds are about a different document; the
round that reads the new one is still owed.

## 3. A rewrite starts an epoch

**The call.** `tp review <spec> --rewritten "<evidence>"` appends an epoch to the spec's round state:
its evidence, when it was declared, and the last recorded round of each phase it follows. **The
evidence is required and must be non-empty**: the epoch's whole content is the operator's statement
of why the earlier rounds read a different document. tp does not judge the statement; it requires one.
The call is uncounted — no round, no streak, no exit code beyond usage errors.

**What reads it.** Every list of earlier rounds' findings that an emission builds reads only rounds
recorded after the latest epoch: review's carried findings, open, accepted and resolved alike; the
regression prompt's previously fixed findings; audit's Prior Round block. A round recorded after the
epoch is read exactly as today.

**What it does not touch.** No round file, no round entry and no disposition changes. The rounds
before it stay on disk and in the state, and convergence is computed over every recorded round as
today. That is sufficient in the field's case without writing anything: a round before the epoch
that holds open findings breaks the clean streak by itself, so the streak starts over at the first
clean round against the new text.

**`--status` reports it.** `tp review <spec> --status` and `tp audit <spec> --status` carry each epoch
— the rounds it follows and its evidence, of the form *rounds 1–8 precede a rewrite: \<evidence\>* —
under a key, so a reader of a history full of stale-looking rounds finds out why beside them.

**It is not fenced under `TP_UNATTENDED`.** It removes nothing from convergence and writes no
disposition. What it stops is tp telling reviewers what earlier rounds found, so the worst an
unattended unit can do with it is have a finding reported again.

### 3.1 Both loop phases, not ground

**The epoch covers review and audit, and one call declares it for both.** It belongs to the text, not
to a phase: an audit round before a rewrite graded code against checklist items derived from sections
that no longer exist, and its Prior Round block would ask the next auditor to re-check them. In the
common case — a rewrite before implementation — no audit round exists yet and the coverage costs
nothing.

**Ground is out.** Its carry is keyed on each unit's own text, so a rewritten sentence is asked afresh
already, and a sentence the rewrite kept byte for byte keeps the verdict that sentence earned.

## 4. A round's `spec_hash` is the hash of its snapshot

**The decision: at record time, a round's `spec_hash` is the sha256 of the round's own snapshot**
rather than of the spec file as it stands when the round is recorded, in both review and audit. The
snapshot is already written at emission, already per round and per phase; no new artifact, field or
write. `stale` still compares the latest round's hash with the spec as it stands; what changes is that
the round's side names the text the round was given.

**Why here.** It is what makes §2's named exit the only one, and what lets a reader tell which
recorded rounds read a text that no longer exists. Once `emitting-does-not-lose-a-round` refuses a
re-emission over an unrecorded round whose text changed, a round has one snapshot and it is the text
every role was given; until then it is the last text emitted for that round, and the sidecar of
`round-records-the-text-it-read` measures the difference.

**A round whose snapshot is absent hashes the spec path, as today, and does not fail.** That is the
live case of a `--record` with no preceding emission.

## 5. Non-Goals

1. **The epoch does not reset the clean streak, and no convergence rule changes.** The streak spanning
   a rewrite is real: a converged spec, rewritten, converges again after one round against the new
   text (sidecar, *A converged spec, rewritten*). It is the declared-event case of a question already
   parked as `spec/undecided.md`'s *Resetting the clean streak when consecutive rounds read different
   text*. Taking it here would make this a loop-class release touching every reader of convergence;
   the epoch's record is what a later release would key such a reset on.
2. **No streak reset when consecutive rounds carry different hashes** — the former §3 of
   `round-records-the-text-it-read`. It is parked in the same register entry, which records why it is
   not worth a cycle: a single historical instance in the recorded audit corpus, and no new mismatch
   on the review side since v0.35.0.
3. **The escalation stands.** The blocker's class, the phases it fires in and `tp resume`'s
   `next_action` are unchanged. Whether `next_action` should defer to an escalation is a question about
   every escalate blocker, not this one.
4. **Nothing recorded is rewritten or deleted, and no disposition is written**, by any part of this
   release.
5. **No automatic epoch.** tp does not infer a rewrite from a diff; the operator declares it.
6. **No repair of rounds already recorded with a record-time hash.** Rewriting them would fabricate a
   claim about what those rounds read.
7. **No `spec_moved_mid_round`.** The earlier draft counted re-emissions that changed a round's
   snapshot; `emitting-does-not-lose-a-round` refuses that re-emission instead, and took the section.

## 6. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Rows 2, 3, 5, 10 and
12 read a value that exists at `HEAD`, and the sidecar quotes it; the other rows' subjects do not
exist yet, so their two counts belong to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | at the implement phase, with the spec edited after the last recorded review round, the `spec-stale` blocker's data names the clearing command — asserted on the key, not on the message | the shipped blocker, whose data carries only the spec |
| 2 | §2 *clears* | running exactly the named command — emit, then record the round's findings, which in a roleless fixture is a file with no rows — removes the blocker | name a command that does not clear it, such as `--rewritten` alone, which leaves `stale` true |
| 2b | §2 *in flight* | a round emitted, the spec then edited, the round unrecorded: the blocker names recording that round first or discarding it with the emission's `--force`, and each named sequence removes the blocker. *Deferred* until `emitting-does-not-lose-a-round` ships | name the bare emission, which that release refuses at exit 3 |
| 3 | §3 | after `--rewritten`, no prompt of the next review emission contains a marker string from a finding recorded before the epoch, and a finding recorded after the epoch appears in the emission after it | read every recorded round, as `HEAD` does — its next prompts carry both pre-rewrite findings |
| 4 | §3 *nothing recorded* | every round file and every round entry is byte-identical before and after `--rewritten` | implement the epoch as the void — a `wontfix` into every earlier row, which turns those rounds clean |
| 5 | §3 *convergence* | on two clean rounds, a rewrite and one more clean round, `consecutive_clean`, `converged` and `--status --check`'s exit are the same with and without an epoch — a characterisation of Non-Goal 1, whose test's doc comment names the register entry as what retires it | reset the streak at the epoch, changing convergence in a release that says it does not |
| 6 | §3 *evidence* | `--rewritten` with empty or absent evidence is a usage error and writes nothing | accept it, recording that something changed — which the hashes already say |
| 7 | §3 *status* | two epochs produce two entries in `--status`, each naming the rounds it follows and its evidence | keep only the latest, so a second rewrite erases the account of the first |
| 8 | §3.1 *audit* | with an audit round recorded before the epoch, the next audit emission carries no Prior Round block; an audit round recorded after it is carried as today | apply the epoch to the review phase only |
| 9 | §3.1 *ground* | on a rewrite that kept one sentence byte-identical, a ground emission carries the same units with and without an epoch | let the epoch clear ground's carry, re-asking a sentence whose verdict still holds |
| 10 | §4 | emit, edit the spec, record: the recorded `spec_hash` equals the round's snapshot hash, and `--status` reports `stale: true` | hash the spec path, as `HEAD` does — it records the edited file's hash and reports `stale: false` |
| 11 | §4 *both phases* | over every round of a fixture holding review and audit rounds — the audit count asserted non-zero rather than chosen — each `spec_hash` equals its snapshot's hash | move the review record path alone, which a single-round review test cannot see |
| 12 | §4 *fallback* | a round with no snapshot on disk records the spec path's hash and exits 0 | fail on a missing snapshot, which breaks a `--record` with no preceding emission |
| 13 | §4 *history* | a round recorded before this release keeps its stored `spec_hash` byte for byte after upgrade | recompute stored hashes on read, rewriting what past rounds are understood to have read |

**Row 5 pins a hole on purpose.** A green test asserting that a rewritten spec re-converges after one
round is the shape most likely to be tidied away as a bug in the test, so its name and doc comment
say it is deliberate and name the parked question whose answer would turn it red.
