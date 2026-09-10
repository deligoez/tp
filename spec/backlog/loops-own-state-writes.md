# The loop's own state writes — folded

2026-09-11. This file is a forwarding stub; shipped artifacts and recorded rounds cite this path.

| what moved | where |
|---|---|
| §2 the round's findings file is written atomically, at all three writers, with §2.1–§2.2 | `a-findings-exits-agree` §6.3 |
| §2's `tp audit --merge -o` symlink and mode divergence (sidecar, *Routed here*) | `a-findings-exits-agree` §6.3 |
| §5 rows 1, 1b, 1c, 2, 3 | `a-findings-exits-agree` §10 rows 15–16 |
| the `LockFilePath` symlink item (sidecar, *Routed here at the 2026-09-08 re-verification*) | stays recorded in `loops-own-state-writes-measurements.md`, sourced from `spec/0.35.0-candidates.md` item 9; no release takes it yet |

**Dropped:** §3 (the gate sees an empty watched directory) and its rows 4–6 — the gate watches
state, state is files, a directory-only change carries none, and its one live instance is gone. The
duplicated `runResult` test type and the `assert.ErrorIs` restoration, both sidecar items — test-only
hygiene with no behaviour to protect.

Its `-measurements.md` sidecar and its `.tp-review/loops-own-state-writes/` rounds stay as history.
