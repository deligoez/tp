# tp — A finding the next round is told not to re-report blocks convergence

A backlog decision for 1.16.0. The operator chose the scope; the rule was settled with a second
opinion from the tp-smart session. The reproduction and the measurement over tp's own histories are
in `spec/1.16.0-measurements.md`.

Class: **decision**.

## 1. Context

Review convergence counts the trailing clean rounds, each judged on its own rows. The next round's
prompts list every unresolved earlier finding under "UNRESOLVED findings from previous rounds — DO
NOT re-report". So a blocking finding nobody fixed or accepted is not raised again, the following
rounds come back empty, and `tp review <spec> --status --check` exits 0: the finding has left
without a fix and without the acceptance CLAUDE.md rule 9 reserves for the operator.

## 2. Decision

1. **The carried set blocks.** A review spec is not converged while the set of findings the next
   emission would carry as unresolved holds a blocking row. One set, computed once, read by both the
   prompt and the convergence check: a row is either carried — roles are told to stay silent about
   it, so it blocks until it is resolved — or cut by an epoch (1.14.0), and then roles are free to
   raise it again, so it need not block.
2. **Unresolved and blocking keep their meanings.** Unresolved is the carry's own: not `fixed`, and
   not `wontfix`/`duplicate` with evidence. Blocking is `review_converge_on`'s.
3. **A row's latest disposition counts.** The carried set takes, for each finding, its most recent
   occurrence, so a row resolved in a later round is not held open by an earlier unresolved copy. A
   finding is identified as the carry identifies it today — its category, location and the first 80
   characters of its text — so two different findings at one location stay two rows, and resolving
   one never hides the other.
4. **Where it shows.** `--status` and `--check`, `next_action` and `tp resume` name the carried
   blocking rows as what stands between the spec and convergence, and how each clears: resolve it
   with `tp review <spec> --resolve`, or start an epoch if the text was rewritten.
5. **An epoch that would cut a blocking row is the operator's decision.** With carried rows blocking,
   an epoch is a way past an unresolved finding: edit the spec, start one, and the row stops
   blocking unless a role raises it again. So `--rewritten` whose cut holds an unresolved blocking
   row is the operator's decision `rewrite-past-blocking` (exit 2 under `tp run`, a member of the
   escalation set, and named in CLAUDE.md rule 9). An epoch whose cut holds no unresolved blocking row
   stays an ordinary mode, as 1.14.0 shipped it.

Verified by tests, not only by design: the rows an emission prints under the do-not-re-report heading
and the rows `--check` names as blocking are compared over a history holding every disposition; two
distinct findings at one location, one resolved and one not, leave the unresolved one blocking.

## 3. Consequences

- Histories that converged with an unresolved blocking row in a carried round read as not converged
  after the upgrade. In tp's own repository that is four shipped specs; the operator chose to leave
  them so rather than backfill dispositions nobody checked. What `tp resume`, the session-start hook,
  the dashboard and `tp run` then report for such a spec is run before the release and recorded in
  the measurements; if any of them starts proposing review work on a shipped spec every session, the
  operator sees that before the release.
- Resolving a finding without a change stays the operator's decision; this rule only stops the
  silence from standing in for it.

## 4. Alternatives

- **Block only on rounds over the current text.** Rejected: one edited character takes an old round
  off the current text while it stays carried, and the hole reopens.
- **Stop telling roles not to re-report unresolved rows.** Rejected: the outcome then depends on
  whether a role happens to notice the problem again; the record should decide.
- **Apply the rule only to rounds recorded after the upgrade.** Rejected: a version-shaped exception
  in a convergence rule.

## 5. Non-goals

- The audit phase. Its prompt asks a role to re-grade a prior non-PASS row rather than stay silent,
  but empty rounds can still converge past it; that is a separate note with its own measurement.
- Dispositioning old histories.
