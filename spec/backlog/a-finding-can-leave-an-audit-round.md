# tp — A finding can leave an audit round

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-finding-can-leave-an-audit-round-measurements.md` beside it, and this file
stands without them. Its review-side sibling — one predicate for a closed finding, the refusals, the
emitted wording — is `a-findings-exits-agree.md`.

Class: **loop** — it changes which rows survive into the audit grade, so it is reviewed by the
mechanism it changes. Budget it at the loop-class median in `CLAUDE.md`'s *What a cycle costs*.

## 1. The decision

**Context.** A finding leaves a round either as a change to what it is about, or as a `--resolve`
disposition; there is no third way out, and `--status` says which happened. On the audit side the
second exit does not exist. Write a `wontfix` with evidence into a round's own **recorded** file and
run `tp audit <spec> --status`: nothing it reports changes. The same disposition on the review side
clears its round. The probe running both, one command each, is in the sidecar under *The accepted
finding that changed nothing*; a field report (WB-3155) reproduced it after dispositioning a whole
round.

So an audit finding the operator has decided to accept has three exits, and each costs something it
should not: repair it after all, destroy the record, or re-record the same rows under a different
`audit_converge_on` — which reaches only non-`error` severities, is human-only under
`TP_UNATTENDED`, and spends a round of budget (sidecar, *The third exit*). And an acceptance the
next round cannot see does not last: the next round's auditor is shown the row without its
disposition and re-opens it.

**What that costs is measured, not supposed.** `CLAUDE.md`'s own shipping rule — *record the
out-of-surface findings with justification, name the version that takes them, and ship* — is a rule
the tool cannot express. This repository's own `v1.1.0` audit needed cap raises because nothing a
round found could be accepted rather than repaired, and its dispositions were written as prose in
the release notes because there was nowhere else to put them.

**Decision.** `tp audit --resolve`, after writing the disposition into the recorded round file,
re-stamps that round's recorded verdict: when every non-`PASS` row carries a `wontfix` or
`duplicate` disposition with non-empty evidence, the round becomes clean; otherwise the stamp
stands. `fixed` does not clear a round. The disposition write is fenced (§3). The next round's
auditor sees each prior row's disposition and evidence (§4). Every audit count reads the same
surviving set, and `--status` says how many rows carry a disposition (§5).

**Consequences.** This reverses a refusal `spec/0.37.0.md` §2 wrote down; its argument was about a
policy switch re-grading recorded rows, not about a disposition, and the sidecar gives it in full
under *Why the reversal is not a gap being filled*. `--resolve` acquires a precondition on its
positional (§2.1). No new recorded field. Under `audit_converge_on: blocking`, a partial disposition
does not clear the round (§2.1). The Prior Round block changes shape, so
`brief-carries-the-forcing-sentences`, which edits the same block, ships after this spec.

**Alternatives.** A second design recovers the policy in force from the stored stamp and re-grades
the surviving rows when the round is read. It is correct on every case built against it, and it is
rejected because it leaves the recovery in a reader — a round would be re-graded by being read,
which is the thing `spec/0.37.0.md` §2 fenced. Both designs, with their transcripts and the one
enumerable case each is conservative on, are in the sidecar under *The third design, found twice*.

## 2. An accepted audit finding can clear the round

**The surviving set is computed when the disposition is written; the policy is not re-read.** The
round's recorded rows are read, every row closed `wontfix` or `duplicate` **with evidence** is
dropped, and what remains is graded under the policy that was in force when the round was recorded.
A round nobody touches keeps the verdict it was recorded with.

**`fixed` does not close a round.** A repair is a claim about the code, and the round that tests it
is the next one. This is the rule the review side already applies, and the reason a row arriving at
`--record` already marked `fixed` is refused there.

**The accepted row stays in the round and stays visible.** It is still recorded, still emitted by
`tp audit --merge` with its `resolved` block intact, and still counted among the round's findings.
It stops gating convergence; it does not stop existing.

### 2.1 What the design costs, and the precondition it adds

**It is conservative in one enumerable case.** Under `audit_converge_on: blocking`, every non-`PASS`
row must carry a disposition for the round to clear — so a round whose `error` finding is accepted
and whose `warning` finding is left open still reports not clean, where `blocking` alone would have
called it clean. The exit is `--resolve-all` with a shared justification, the channel
`skills/tp/SKILL.md` already documents. That is a cost this release accepts rather than a defect:
the sentence it must deliver is *the acceptance channel works*, not *`blocking`'s accepted-open
semantics extend to `--resolve`*.

**`--resolve` derives which round to re-stamp from the path it was given, and re-stamps nothing
when handed any other file.** This turns *resolve into the file `--record` wrote* from advice into a
precondition. The write into another file is not refused: resolving into the merge output **before**
`--record` is a legitimate path, and the disposition is recorded with the round.
`a-findings-exits-agree.md` §6.1 gives `--record` the key that names the file it wrote;
`audit-records-what-was-graded` makes a resolve into a file that is not a recorded round say so, and
offers a re-record `next_step` only inside a driver round — under this spec the re-stamp is the
resolve's own, so outside one no re-record is needed.

## 3. The disposition write is fenced

**A `wontfix` with no evidence is accepted today and then ignored.** On a fresh one-finding
recorded round, `--resolve 0 wontfix` with no evidence argument exits 0 and writes an empty
evidence string; the record requires non-empty evidence, so the row stays open — accepted, reported
as resolved, and silently ignored (sidecar, *A `wontfix` with no evidence*).

**And no fence stops an unattended agent from writing the disposition.** `tp audit <round file>
--resolve` under `TP_UNATTENDED=1` exits 0, with a reason and with an empty one (sidecar, *The fence
that does not exist*). Once §2 makes a disposition clear a round, that write is exactly the escape
hatch `skills/tp/SKILL.md`'s *"a disposition is not an escape hatch from the gate"* exists to deny.

**Therefore this release carries the fence.** Two sinks:

1. **An acceptance is a user-approved decision under `TP_UNATTENDED=1`**: `tp audit --resolve` /
   `--resolve-all` writing `wontfix` or `duplicate` is refused at exit 2 with an escalation hint, like
   every other decision `CLAUDE.md` reserves for the operator. **`fixed` is not fenced**: it clears no
   round (§2), so it is no escape hatch, and it is the durable write of `tp run`'s audit-fix unit —
   every child of a run carries `TP_UNATTENDED=1`, so fencing it would escalate every audit finding a
   run repairs.
2. **`resolved.evidence` must be non-empty** — at the write, which accepts `""` today, and at record
   time, where §2's predicate reads it. tp does not judge the reason; it requires one.

## 4. The next round sees the disposition

**Each row in the next round's Prior Round block carries the prior row's disposition and its
evidence**, beside the `status` and `changed_since` it carries today, and is framed by them:

| prior row | framing |
|---|---|
| accepted — `wontfix` or `duplicate` | *accepted: re-open only if `changed_since` is true* |
| `fixed` | *fixed: verify the repair held* |
| no disposition | today's *context to re-check, not a verdict to repeat* |

**Why.** An acceptance that clears a round but is invisible to the next round's auditor gets
re-opened, and the re-opened row blocks that round — so without this section §2's clearance lasts
exactly one round. A `fixed` row shown without its evidence cannot be checked against the repair it
claims. Today the block is byte-identical before and after a round's dispositions are written.

## 5. Every audit count reads the surviving set

`open` in `role_streaks`, each role's streak, `spec_coverage_clean_rounds` and `--status --check`
derive from the set §2 defines: a row closed `wontfix` or `duplicate` with evidence is not open. Two
predicates are two things to drift, and the drift is invisible because both produce plausible
integers — the sidecar's *The third exit* shows a live tree moving `clean` while `role_streaks` stayed
open.

**`--status` reports `dispositioned: k/n` on every recorded audit round** — `n` the round's
findings, `k` those carrying any disposition, `fixed` included — the same key
`a-findings-exits-agree.md` §3 puts on the review side. Today `--status` is byte-identical before and
after a round's dispositions, which `skills/tp/SKILL.md` documents as intended; this release changes
that documented behaviour, and the SKILL passage with it.

**`next_action` follows the same set**: when every non-`PASS` row in the latest round carries a
disposition, it says the round is disposed and the next step is to re-audit.

## 6. Non-Goals

1. **No new recorded field, no new flag, no config.** §2.1 takes the design that needs none;
   `dispositioned` is computed on read.
2. **One convergence change, scoped to one signal.** §2 changes which rows survive into the audit
   grade and nothing else: not the panel, not the `spec_hash`, not the review side.
3. **No retroactive re-grading.** A stamp changes only when `--resolve` writes into that round's
   recorded file. Reading never changes one.
4. **No new disposition value.** `fixed`, `wontfix` and `duplicate` are the three `--resolve`
   accepts, and this release adds none.
5. **The review side is unchanged.** It already drops disposed rows when the round is read.
6. **A durable home for an accepted finding is deferred**, with its decision already taken and
   recorded (sidecar, *Decided at the 2026-09-08 decision pass*). It is a new recorded artifact and
   a new key on two reporting surfaces, the shape `CLAUDE.md` names as belonging to the next version.
   **Reopen condition:** the first cycle that accepts a finding and then cannot tell, at
   decomposition, which release owes it.
7. **An audit-side non-blocking open count is deferred**, same pass, same reason. It is meaningful
   only under `audit_converge_on: blocking`, which is opt-in and human-only. **Reopen condition:** a
   cycle that runs `blocking` and cannot see what its clean round accepted.

## 7. Tests

Every row derives from a numbered decision and names a mutant that must fail it. Every row is
written, not yet watched; row 1 is the acceptance and must be seen failing against `HEAD` first.
Only row 1 quotes a pair of counts: the others' subjects do not exist at `HEAD`, so their pairs
belong to the implementing task's acceptance.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a one-`FAIL` audit round, resolved `wontfix` with evidence in its **recorded** file, takes the clean streak 0 → 1 and the role's open count 1 → 0, with `audit_converge_on` pinned to `all` in the fixture | the shipped stamp, which leaves the round not clean and the open count at 1 |
| 2 | §2 *severity* | the same assertion with the row's `severity: "error"` and `audit_converge_on: blocking` | grade the acceptance from severity, which reaches an advisory row and not this one |
| 3 | §2 *fixed* | the same row resolved `fixed` does **not** clear the round | close on any disposition, which would let a repair certify itself instead of leaving the next round to test it |
| 4 | §2 *evidence* | `wontfix` with an empty or absent evidence string does not clear the round | drop the evidence requirement, making acceptance free |
| 5 | §2 *policy* | a round recorded under `all` still grades under `all` after the project is set to `blocking`, and the reverse — **asserted with a `warning`-severity finding** | read the policy when the round is read; on an `error` fixture the live-read mutant returns the same answer and survives, so the severity is the assertion |
| 6 | §2.1 *partial* | under `blocking`, a round with its `error` finding accepted and its `warning` finding open still stamps not clean, and `--resolve-all` with a shared justification clears it | clear on the blocking rows alone, which is the rejected design's conservative case and not this one's |
| 7 | §2.1 *precondition* | `--resolve` handed a findings file that is not a recorded round re-stamps nothing: `state.json` is byte-identical before and after (the notice it prints is `audit-records-what-was-graded`'s row) | derive the round from the current directory's active spec, which re-stamps a round the file does not belong to |
| 8 | §2 *visible* | the accepted row still appears in `tp audit --merge` output with its `resolved` block | drop it from the round, which is the record destruction this release exists to remove |
| 9 | §3 *write* | `tp audit <round file> --resolve <n> wontfix ""`, and the same with the evidence argument absent, is refused at exit 2 and writes nothing | accept the status alone — `HEAD`, which writes an empty evidence string |
| 10 | §3 *record* | a `wontfix` row whose evidence is empty or missing counts as open and does not clear §2's stamp | drop the evidence requirement at record, making an acceptance indistinguishable from a deletion |
| 11 | §3 *fence* | `TP_UNATTENDED=1 tp audit <round file> --resolve 0 wontfix "reason"` exits 2 with an escalation hint, and `--resolve-all wontfix` likewise; both exit 0 without the variable | leave `--resolve` unfenced — `HEAD`, which makes §2 an agent-writable escape hatch |
| 11b | §3 *fixed passes* | `TP_UNATTENDED=1 tp audit <round file> --resolve 0 fixed "repair sha"` exits 0 and writes the disposition, and the round's stamp is unchanged | fence every status, which escalates each finding a `tp run` audit-fix unit repairs |
| 12 | §4 | after a round's rows are resolved one `wontfix`, one `fixed`, one left open, the next round's Prior Round block carries each row's disposition and evidence under the framing §4's table gives, and differs from the block emitted before the dispositions | the shipped block, byte-identical before and after |
| 13 | §4 *changed_since* | an accepted row's framing tells the auditor to re-open it only when `changed_since` is true, and the row still carries `changed_since` | drop `changed_since` once a disposition is shown, which leaves the auditor no way to tell an acceptance that still holds from one the code has overtaken |
| 14 | §5 | on a round with one disposed and one open finding, `open` is 1, the role's streak is 0, `--check` exits 1 and `clean` is false — each named, not "agree" | give `open` its own predicate, which moves `clean` while `role_streaks` stays open |
| 15 | §5 *dispositioned* | `--status` reads `dispositioned: 0/1` on a one-finding round, and `1/1` after a `--resolve` into its recorded file | leave `--status` byte-identical — `HEAD` |
| 16 | §5 *next_action* | with every non-`PASS` row disposed, `next_action` names re-auditing rather than addressing findings | leave the string static |
