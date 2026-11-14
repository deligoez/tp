# tp — A converged payload says what it left open

**Parked.** Class: **decision**. Un-park it when the first project sets `audit_converge_on: blocking`;
tp itself does not. `review_converge_on` already defaults to `blocking`, and review already emits
`nonblocking_open` under it; `audit_converge_on` defaults to `all`. The decisions were taken at the
2026-09-08 decision pass; the measurements are in `spec/undecided-measurements.md` §From the rows
spec.

## 1. Context

Under `blocking` a round converges with non-blocking rows still open, and with blocking rows an
operator accepted. The payload should say so, so a driver reading only the payload knows what the
convergence left behind. Today it says so on one side only, and one class of audit row blocks by
accident.

## 2. Decision

1. **An audit-side `nonblocking_open`.** Review already emits it (`internal/engine/reviewclean.go`).
   The audit emits it too under `audit_converge_on: blocking`, and the guards that pin its absence on
   audit are inverted.
2. **A count of accepted blocking rows, on both sides.** One counter on the converged payload, so an
   accepted finding is not read as no finding. The review side waits with the audit side: accepting a
   critical or high review finding is already an operator decision with its own escalation record.
3. **Audit rows written with review severity words.** On 2026-11-14, 67 recorded audit rows across
   tp's history carried `low`/`medium`/`high` instead of `error`/`warning`/`info`.
   `AuditSeverityBucket` (`internal/engine/auditclean.go`) reads them as unrecognised, which blocks.
   Either the record sink names such a row with a warning, or the review vocabulary is mapped onto the
   audit one. Which is left to the note when it is un-parked.

## 3. Alternatives

- **Building it now.** Not taken: no project sets `audit_converge_on: blocking`, so every part is
  surface with no reader.
- **Severity checked as a refusal at the record sink.** Not taken at the 2026-09-08 pass: severity
  stays self-declared, and a warning names the row without losing it.

## 4. Non-goals

- Changing what `all`, the audit default, converges on.
- A durable list of accepted findings across specs (closed in `spec/undecided.md`).
