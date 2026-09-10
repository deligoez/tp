# tp — An invalid task file is reported — folded

Folded on 2026-09-11 into `an-unreadable-file-is-named.md`, which widened to *tp says when it could
not read*.

| what moved | where |
|---|---|
| §2, an unreadable or unparseable task file reported by `tp lint` | `an-unreadable-file-is-named` §4 — at `warning` severity, no longer `error`: lint reads the task file only for advisory findings, and every command that needs it already refuses the same file at exit 3 |
| §3 Non-Goal 1 (no schema validation in lint) | `an-unreadable-file-is-named` §7 Non-Goal 5 |
| §4 rows 1, 1b, 1c | `an-unreadable-file-is-named` §8 rows 6 and 6b |
| sidecar, `--verify`'s missing zero-parse refusal | `an-unreadable-file-is-named` §3 site 4 |

Dropped: nothing of §2. The sidecar's `validate --project` and `--report` items were not carried —
neither is a read failure — and stay there as history.

Its `-measurements.md` sidecar and any `.tp-review/an-invalid-task-file-is-reported/` rounds stay as
history.
