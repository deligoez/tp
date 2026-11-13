# tp — An audit FAIL no later round re-grades blocks convergence

A backlog decision for 1.18.0. The operator chose the rule (the same as review's, 1.16.0); the
audit translation was settled with a second opinion from the tp-smart session. The reproduction and
the measurement over tp's own histories are in `spec/1.18.0-measurements.md`.

Class: **decision**.

## 1. Context

Audit convergence counts the trailing clean rounds, each graded on its own rows. A round's Prior
Round block asks each role to re-grade the immediately prior round's non-PASS rows, but nothing holds
the loop open when it does not: record a FAIL in round 1, record rounds 2 and 3 empty, and
`tp audit <spec> --status --check` exits 0 while round 3's prompt no longer names the FAIL. CLAUDE.md
rule 13 says an audit ships with no FAIL in any round.

## 2. Decision

1. **The carried set blocks.** An audit is not converged, by clean rounds or by its cap, while a
   carried row blocks. The carried set holds, per `(role, item_id)` since the newest epoch, the row of
   its latest round; inside one round the worst row under a key counts (`--merge` keeps disagreeing
   rows). A carried row blocks when it is not PASS, not accepted (wontfix or duplicate with
   evidence), and blocking under the latest recorded round's `audit_converge_on`. It is graded at the
   highest severity among its rows since its last resolving disposition.
2. **Only a re-grade clears it.** A later PASS under the same key clears the row, with or without a
   `fixed` disposition; `fixed` alone clears nothing, since an empty round is exactly the "graded 0
   of N" this note closes. A PASS clears a positional id (`table-`, `list-`, `finding-`) only when
   its round read the same spec text as the row's round (equal `spec_hash`); `task-` and `file-` ids
   name their item whatever the text.
3. **Every later prompt asks for it.** The Prior Round block lists every carried row whose latest
   grade is not PASS, from whichever round since the epoch, each with its round; the acceptance carry
   reads the same latest row. A row nobody is asked to re-grade must not block in silence.
4. **Where it shows.** `--status` and `--check`, `next_action` and `tp resume` name the carried
   blocking rows and how each clears: a later round's PASS, an acceptance with evidence
   (`accept-finding`, the operator's), or an epoch. When no role of the next round can re-grade a row
   (its role retired or disabled), the surface says so and names the operator's exits.
5. **An epoch past a blocking audit row is the operator's decision.** `--rewritten` whose cut holds an
   unresolved blocking review or audit row is `rewrite-past-blocking`; one decision, its text
   reworded from "review finding" to "finding".

## 3. Consequences

- Shipped specs whose audit converged past a FAIL read as not converged after the upgrade; as in
  1.16.0 they are left so, measured and listed, tp's own included.
- An operator switching `audit_converge_on` to `blocking` releases carried warning and info rows on
  the next status read: the policy is the latest round's, as the cap's grading already is.
- A spec repair inside one epoch makes a positional row unclearable by PASS until the operator
  accepts it or starts an epoch. A known limit: rows carry no item text, so tp cannot tell which
  item a shifted id now names, and errs toward review.
- Accepting an audit row is already `accept-finding`; this adds occasions for it, not a decision.

## 4. Alternatives

- **Any later recorded round verifies a fix** (review's rule). Rejected: an empty audit round is the
  defect itself; the audit prompt asks for a re-grade, so the re-grade is the evidence.
- **Hold every FAIL of any round forever** (rule 13 read literally). Rejected by the operator: a FAIL
  re-graded PASS after a repair is the loop working.

## 5. Non-goals

- Emitting a skipped role only to re-grade its carried rows; the rows block and the surface says so.
- Dispositioning old histories.
