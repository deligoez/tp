# tp — The round carries the text it read — folded

2026-09-11. This spec left the backlog order; a forwarding stub, because shipped artifacts cite it.

| what moved | where |
|---|---|
| §2 — a round's `spec_hash` is the sha256 of its own snapshot, not of the spec file at record time | `reconcile` §4 |
| the `spec_moved_mid_round` count its sidecar handed to reconcile | `emitting-does-not-lose-a-round`, which refuses the re-emission instead of counting it |

Dropped: §3, the audit streak reset when consecutive rounds carry different hashes — parked in
`spec/undecided.md` under *Resetting the clean streak when consecutive rounds read different text*,
because it has one historical instance and has been dormant since v0.35.0.

`CLAUDE.md` says this file *closes it forward by hashing at emit*: that decision now lives in
`reconcile` §4, which likewise repairs none of the rounds already recorded.

Its `-measurements.md` sidecar and its `.tp-review/round-records-the-text-it-read/` rounds stay as
history.
