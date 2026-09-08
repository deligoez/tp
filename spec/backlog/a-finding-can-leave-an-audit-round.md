# tp — A finding can leave an audit round

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-finding-can-leave-an-audit-round-measurements.md` beside it; this file stands
without them. It is the audit half of the hotfix spec formerly numbered `1.0.2`; the other half — the count, the
refusals, the fence, the predicates and the emitted wording — is `a-findings-exits-agree.md`.

Class: **loop** — it changes which rows survive into the audit grade, so it is reviewed by the
mechanism it changes. Budget it at the loop-class median in `CLAUDE.md`'s *What a cycle costs* table.

## 1. The decision

**Context.** A finding leaves a round either as a spec change or as a `--resolve` disposition; there
is no third way out, and `--status` says which happened. On the audit side the second exit does not
exist. A `FAIL` resolved `wontfix` with evidence, written into the round's own **recorded** file,
changes nothing: `clean` is stamped at record and `auditRoundOpenByRole` reads `AuditRowIsPass` alone,
never `resolved`. The same disposition on the review side clears the round, because
`engine.ReviewRoundClean` recomputes live. The probe showing both is in the sidecar under *The
accepted finding that changed nothing*. So the only exits are repairing the finding, destroying the
record, or re-recording the same rows under a different `audit_converge_on` — a third exit that
reaches only non-`error` severities, is human-only under `TP_UNATTENDED`, and costs a round of budget
(sidecar, *The third exit*). A cycle that has decided to ship over a known, justified finding cannot
say so. `CLAUDE.md`'s own shipping rule — *record the out-of-surface findings with justification, name
the version that takes them, and ship* — is one the tool cannot express, and two releases have shipped
through that divergence by hand.

**Decision.** `tp audit --resolve`, after writing the disposition into the recorded round file,
re-stamps that round's `state.json` entry: when every non-`PASS` row carries a `wontfix` or
`duplicate` disposition with non-empty evidence, `clean` becomes true; otherwise the stamp stands.
`fixed` does not clear a round.

**Consequences.** The re-stamp reverses a written refusal in `spec/0.37.0.md` §2 and says so (§2).
`--resolve` acquires a precondition on its positional (§2.1). No recorded field is added. Under
`blocking`, a partial disposition does not clear the round (§2.1). `open`, `role_streaks`, `--check`
and `next_action` must read the same surviving set — that is `a-findings-exits-agree.md` §5, and this
spec's rows assert only the stamp and the two signals beside it.

**Alternatives.** Design B recovers the policy in force from the stored stamp and re-grades the
surviving rows at read time. It is correct on every case built against it, and it is rejected because
it leaves the recovery in a reader: a round would be re-graded by being read, which is the thing
`spec/0.37.0.md` §2 fenced. Both designs, with transcripts, are in the sidecar under *The third
design, found twice*.

## 2. An accepted audit finding can clear the round

**The surviving set is computed live; the policy is not.** `AuditRoundClean` reads the round's recorded
rows, drops every row closed `wontfix` or `duplicate` **with evidence**, and grades what remains under
the policy that was in force when the round was recorded. `fixed` does not close a round — a repair is
a claim about the code and the next round is what tests it.

**This reverses a stated refusal, and says so.** `spec/0.37.0.md` §2 names `--resolve` in the
alternative it rejects; what justifies reversing it is that §2's argument is about a *policy switch*,
not about a disposition — a disposition removes a row the operator wrote evidence for, and cannot
retroactively re-grade a round nobody touched. The argument in full is in the sidecar under *Why the
reversal is not a gap being filled*.

### 2.1 Which design, and what it costs

**Two designs satisfy this without adding a recorded field.** Both were built and run — the sidecar's
*The third design, found twice* has both, with their transcripts.

- **A — re-stamp at `--resolve`.** After writing the disposition into the recorded round file,
  `--resolve` re-stamps that round's entry: every non-`PASS` row disposed ⇒ `Clean=true`, otherwise the
  stamp stands. `spec/0.37.0.md` §2 already anticipates an operator write changing a stamp.
- **B — recover the policy from the stamp.** `all` implies `blocking`, so over the recorded rows the
  stored `clean` bool identifies the policy in force wherever the two disagree, and the remaining case
  grades under `all`.

**This release takes A.** It writes at the moment the operator acts, so no reader recomputes anything
and no round is ever re-graded by being read; B recovers correctly but leaves the recovery in a reader,
which is the thing §2 fenced.

**Both are conservative in exactly one case, and A's is the one that matters here:** under `blocking`,
a partial disposition does not clear the round. Every non-`PASS` row must carry one, so a round whose
`error` finding is accepted and whose `warning` finding is left open still reports not clean, where
`blocking` alone would have called it clean. The exit is `--resolve-all` with a shared
justification, which is the channel `skills/tp/SKILL.md` documents. That is a cost this release accepts
rather than a defect: the sentence it must deliver is *the acceptance channel works*, not *`blocking`'s
accepted-open semantics extend to `--resolve`*.

**`--resolve` must derive the base from the path** when its positional is a recorded round file
(`spec/.tp-review/<base>/audit-round-N.ndjson`), and must not re-stamp when handed any other file. This
turns "resolve into the file `--record` wrote" from advice into a precondition;
`a-findings-exits-agree.md` §6 gives `--record` the `file` key that makes the precondition usable.

**No new recorded field.** The bar the hotfix set — a fix that is measured, carries no semantic change
beyond the repair, and needs no new recorded field — holds on the last clause; this section is the one
place it moved on the second.

**The accepted row stays in the round and stays visible.** It is still recorded, still emitted by
`tp audit --merge` with its `resolved` block intact, and still a finding in the round's count. It stops
gating convergence; it does not stop existing.

## 3. Non-Goals

1. **No new recorded field, no new flag, no config.** §2.1 takes the design that needs none.
2. **One convergence change, scoped to one signal.** §2 changes which rows survive into the audit
   grade, and nothing else: not the panel, not the `spec_hash`, not the review side. The stopping rule
   is stated up front: if the finding count does not fall after a cut, the problem is the loop and not
   the document.
3. **No retroactive re-grading.** A stamp changes only when `--resolve` writes into that round's
   recorded file; reading never changes one, and a round nobody touches keeps the stamp it was
   recorded with, under the policy that was in force then.
4. **No new disposition value.** `fixed`, `wontfix` and `duplicate` are the three `tp audit --resolve`
   accepts, and this release adds none.
5. **No judgement of `resolved.evidence`'s content, and no fence on the write.** This spec reads the
   evidence at the stamp and requires it to be non-empty. Refusing an empty evidence string at the
   write, and refusing `--resolve` under `TP_UNATTENDED=1`, is `a-findings-exits-agree.md` §4 — and
   that fence is what keeps §2 from being an escape hatch an agent can write for itself, so this spec
   must not ship ahead of it.
6. **The review side is unchanged.** `engine.ReviewRoundClean` already drops disposed rows live.

## 4. Tests

Every row derives from a numbered decision and names a mutant that must fail it. **Every row here is
written, not yet watched**: none has been run red against `HEAD`. Row 1 is the acceptance and must be
seen failing first.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a one-`FAIL` audit round, resolved `wontfix` with evidence in its **recorded** file, takes `consecutive_clean` 0 → 1 and `role_streaks[].open` 1 → 0, with `audit_converge_on` pinned to `all` in the fixture | the shipped stamp, which returns `clean: false` and `open: 1` |
| 1b | §2 *severity* | the same assertion with the row's `severity: "error"` and `audit_converge_on: blocking` | grade the acceptance from severity, which reaches an advisory row and not this one |
| 2 | §2 *fixed* | the same row resolved `fixed` does **not** clear the round | close on any disposition, which would let a repair certify itself instead of leaving the next round to test it |
| 3 | §2 *evidence* | `wontfix` with an empty or absent evidence string does not clear the round | drop the evidence requirement, making acceptance free |
| 4 | §2 *policy* | a round recorded under `all` still grades under `all` after the project is set to `blocking`, and the reverse — **asserted with a `warning`-severity finding** | read the policy live. **The severity is the assertion**: `AuditRowsClean` grades `error` and unrecognised severities as blocking under both policies, so on an `error` fixture the live-read mutant returns the same answer and survives |
| 5 | §2.1 *partial* | under `blocking`, a round with its `error` finding accepted and its `warning` finding open still stamps not clean, and `--resolve-all` with a shared justification clears it | clear on the blocking rows alone, which is B's conservative case and not A's |
| 6 | §2.1 *precondition* | `--resolve` handed a findings file outside `spec/.tp-review/<base>/` writes the disposition and re-stamps nothing | derive the base from the current directory's active spec, which re-stamps a round the file does not belong to |
| 7 | §2.1 *visible* | the accepted row still appears in `tp audit --merge` output with its `resolved` block | drop it from the round, which is the record destruction this release exists to remove |
