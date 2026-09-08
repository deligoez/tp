# tp — A finding can leave an audit round

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-finding-can-leave-an-audit-round-measurements.md` beside it, and this file
stands without them. It is the audit half of the hotfix spec formerly numbered `1.0.2`; the other
half — the count, the refusals, the predicates and the emitted wording — is
`a-findings-exits-agree.md`.

Class: **loop** — it changes which rows survive into the audit grade, so it is reviewed by the
mechanism it changes. Budget it at the loop-class median in `CLAUDE.md`'s *What a cycle costs*.

## 1. The decision

**Context.** A finding leaves a round either as a change to what it is about, or as a `--resolve`
disposition; there is no third way out, and `--status` says which happened. On the audit side the
second exit does not exist. Write a `wontfix` with evidence into a round's own **recorded** file and
run `tp audit <spec> --status`: the round is unchanged. The same disposition on the review side
clears its round. The probe running both, one command each, is in the sidecar under *The accepted
finding that changed nothing*.

So an audit finding the operator has decided to accept has three exits, and each costs something it
should not: repair it after all, destroy the record, or re-record the same rows under a different
`audit_converge_on` — which reaches only non-`error` severities, is human-only under
`TP_UNATTENDED`, and spends a round of budget (sidecar, *The third exit*).

**What that costs is measured, not supposed.** `CLAUDE.md`'s own shipping rule — *record the
out-of-surface findings with justification, name the version that takes them, and ship* — is a rule
the tool cannot express, and two releases have shipped through that gap by hand. The most recent is
this repository's own `v1.1.0`, whose audit needed two cap raises because nothing a round found
could be accepted rather than repaired; its dispositions were written as prose in the release notes
because there was nowhere else to put them.

**Decision.** `tp audit --resolve`, after writing the disposition into the recorded round file,
re-stamps that round's recorded verdict: when every non-`PASS` row carries a `wontfix` or
`duplicate` disposition with non-empty evidence, the round becomes clean; otherwise the stamp
stands. `fixed` does not clear a round.

**Consequences.** This reverses a refusal `spec/0.37.0.md` §2 wrote down, and §2 below says so rather
than leaving a reader to find the contradiction. `--resolve` acquires a precondition on its
positional (§2.1). No new recorded field. Under `audit_converge_on: blocking`, a partial disposition
does not clear the round (§2.1). Whether `open`, the role streaks, `--check` and `next_action` all
read the same surviving set is `a-findings-exits-agree.md` §5, not this spec; this spec's rows
assert the stamp and the two signals beside it.

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
is the next one. This is the rule the review side already applies, and it is the reason a row
arriving at `--record` already marked `fixed` is refused there.

**This reverses a stated refusal, and says so.** `spec/0.37.0.md` §2 names `--resolve` inside the
alternative it rejects. What justifies reversing it is that §2's argument is about a *policy
switch* — a setting changing how already-recorded rows are graded — and not about a disposition. A
disposition removes a row the operator wrote a reason for; it cannot retroactively re-grade a round
nobody touched. The argument in full is in the sidecar under *Why the reversal is not a gap being
filled*.

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

**`--resolve` must derive which round to re-stamp from the path it was given**, and must re-stamp
nothing when handed any other file. This turns *resolve into the file `--record` wrote* from advice
into a precondition. `a-findings-exits-agree.md` §6 gives `--record` the key that names the file it
wrote, which is what makes the precondition usable rather than a trap.

## 3. The disposition write is fenced

**A second invocation that writes state it should refuse: a `wontfix` with no evidence.** Measured on a
fresh one-finding recorded round, `tp review <round file> --resolve 0 wontfix` with no evidence
argument accepts the write, exits 0, and writes an empty evidence string into the row; the record then
requires non-empty evidence, so the row stays open and `consecutive_clean` stays at 0 — accepted,
reported as resolved, and silently ignored, the failure mode the operator cannot see. `spec/1.1.0.md`
cited this case as already refused; the citation was false and has been removed, and the refusal
belongs here, in the release that owns the disposition channel. The transcript is in the sidecar
under *A `wontfix` with no evidence*.

**And no fence stops an unattended agent from writing the disposition.** The audit resolve path does
not ask whether it is running unattended, and `tp audit <round file> --resolve` under
`TP_UNATTENDED=1` exits 0 — with a reason and with an empty one (sidecar, *The fence that does not
exist*). Once §2 makes a disposition clear a round, that write is exactly the escape hatch
`skills/tp/SKILL.md`'s *"a disposition is not an escape hatch from the gate"* exists to deny, while
removing the only thing that currently stops it.

**Therefore this release carries the fence.** Two sinks, both new work:

1. `tp audit --resolve` / `--resolve-all` become user-approved decisions under `TP_UNATTENDED=1`,
   refused at exit 2 with an escalation hint, like every other decision `CLAUDE.md` reserves for the
   operator.
2. **`resolved.evidence` must be non-empty** — both at the write, which accepts `""` today, and at
   record time, where §2's predicate reads it. An acceptance without a stated reason is
   indistinguishable from a deletion. tp does not judge the reason; it requires one.

**So a disposition closes a finding only when it carries non-empty `evidence` and only for
`wontfix`/`duplicate`.** A rule enforced by making a recorded decision meaningless is enforced in the
wrong place — but it has to be enforced somewhere, and this release is where.

## 4. Non-Goals

1. **No new recorded field, no new flag, no config.** §2.1 takes the design that needs none.
2. **One convergence change, scoped to one signal.** §2 changes which rows survive into the audit
   grade and nothing else: not the panel, not the `spec_hash`, not the review side.
3. **No retroactive re-grading.** A stamp changes only when `--resolve` writes into that round's
   recorded file. Reading never changes one.
4. **No new disposition value.** `fixed`, `wontfix` and `duplicate` are the three `--resolve`
   accepts, and this release adds none.
5. **The review side is unchanged.** It already drops disposed rows when the round is read.
6. **A durable home for an accepted finding is deferred**, with its decision already taken and
   recorded. The 2026-09-08 pass decided a repository-level accepted-findings file surfaced as an
   open count until a task claims it (sidecar, *Decided at the 2026-09-08 decision pass*). It is
   deferred here because it is a new recorded artifact and a new key on two reporting surfaces,
   which is the shape `CLAUDE.md` names as belonging to the next version rather than to the release
   that found the need. **Reopen condition:** the first cycle that accepts a finding and then cannot
   tell, at decomposition, which release owes it.
7. **An audit-side non-blocking open count is deferred**, same pass, same reason. It is meaningful
   only under `audit_converge_on: blocking`, which is opt-in and human-only, and inverting the
   guards that pin the key's absence is work whose subject is that setting rather than this
   channel. **Reopen condition:** a cycle that runs `blocking` and cannot see what its clean round
   accepted.

## 5. Tests

Every row derives from a numbered decision and names a mutant that must fail it. **Every row here is
written, not yet watched**: none has been run red against `HEAD`, and row 1 is the acceptance, so it
is the one that must be seen failing first. **Only row 1 quotes a pair of counts**, and the rest
omitting one is deliberate rather than an oversight — their subject does not exist at `HEAD`, so the
value at `HEAD` and the value under the mutant are confirmable only once the code lands. Step 0.5 defers that
pair to the implementing task's acceptance; it does not belong to a grading round.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a one-`FAIL` audit round, resolved `wontfix` with evidence in its **recorded** file, takes the clean streak 0 → 1 and the role's open count 1 → 0, with `audit_converge_on` pinned to `all` in the fixture | the shipped stamp, which leaves the round not clean and the open count at 1 |
| 2 | §2 *severity* | the same assertion with the row's `severity: "error"` and `audit_converge_on: blocking` | grade the acceptance from severity, which reaches an advisory row and not this one |
| 3 | §2 *fixed* | the same row resolved `fixed` does **not** clear the round | close on any disposition, which would let a repair certify itself instead of leaving the next round to test it |
| 4 | §2 *evidence* | `wontfix` with an empty or absent evidence string does not clear the round | drop the evidence requirement, making acceptance free |
| 5 | §2 *policy* | a round recorded under `all` still grades under `all` after the project is set to `blocking`, and the reverse — **asserted with a `warning`-severity finding** | read the policy when the round is read. **The severity is the assertion**: an `error` row is blocking under both policies, so on an `error` fixture the live-read mutant returns the same answer and survives |
| 6 | §2.1 *partial* | under `blocking`, a round with its `error` finding accepted and its `warning` finding open still stamps not clean, and `--resolve-all` with a shared justification clears it | clear on the blocking rows alone, which is the rejected design's conservative case and not this one's |
| 7 | §2.1 *precondition* | `--resolve` handed a findings file that is not a recorded round writes the disposition and re-stamps nothing | derive the round from the current directory's active spec, which re-stamps a round the file does not belong to |
| 8 | §2 *visible* | the accepted row still appears in `tp audit --merge` output with its `resolved` block | drop it from the round, which is the record destruction this release exists to remove |
| 9 | §3 *write* | `tp audit <round file> --resolve <n> wontfix ""`, and the same with the evidence argument absent, is refused at exit 2 and writes nothing | accept the status alone — `HEAD`, which writes an empty evidence string |
| 10 | §3 *record* | a `wontfix` row whose evidence is empty or missing counts as open in the round's open count and does not clear §2's stamp | drop the evidence requirement at record, making an acceptance indistinguishable from a deletion |
| 11 | §3 *fence* | `TP_UNATTENDED=1 tp audit <round file> --resolve 0 wontfix "reason"` exits 2 with an escalation hint, and `--resolve-all` likewise; both exit 0 without the variable | leave `--resolve` unfenced — `HEAD`, which accepts it at exit 0 and makes §2 an agent-writable escape hatch |
