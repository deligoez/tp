# Archived backlog

On 2026-09-11 the backlog was cut to the decision candidates still in this directory
(`checklist-covers-what-changed`, `reconcile`; `gate-sequence` followed after v1.2.0). Reproduced defects live in
`BUGS.md` and are fixed test-first; everything else was removed. Each row names the files and the
last commit that touched them: read one with `git show <sha>:spec/backlog/<file>`. The ground and
review rounds under `.tp-review/` stay, as rows of the corpus derivations. On 2026-11-14 the decision
notes shipped from v1.3.0 to v1.18.0 followed; code comments that cite them resolve through these rows.

| slug | files | sha | outcome |
|---|---|---|---|
| `audit-records-what-was-graded` | `audit-records-what-was-graded-measurements.md`, `audit-records-what-was-graded.md` | `87aef25f` | landed in this release through the fix track; its header names the seven commits |
| `a-task-file-write-names-its-target` | `a-task-file-write-names-its-target-measurements.md`, `a-task-file-write-names-its-target.md` | `675c9829` | landed: `2cc2e544` (writes name their file, the pointer is announced), `d73eb8f5` (`tp unclaim`), `85956dbc` (batch `commit` string or array); one bullet is one criterion → `BUGS.md` |
| `the-floor-names-what-it-cut` | `the-floor-names-what-it-cut-measurements.md`, `the-floor-names-what-it-cut.md` | `997eeed8` | the splitter landed at `dd2a0d4f`; naming cut spans was not taken (no case beyond the splitter) |
| `emitting-does-not-lose-a-round` | `emitting-does-not-lose-a-round-measurements.md`, `emitting-does-not-lose-a-round.md` | `8d74c196` | landed at `8dff8b4a` (re-emit over a changed round exits 3; `--force` discards, fenced unattended); the scratch filename → `BUGS.md` |
| `a-findings-exits-agree` | `a-findings-exits-agree-measurements.md`, `a-findings-exits-agree.md` | `ee538cff` | one open-finding predicate landed at `895f83a0`, a refusal writing nothing at `1a2cee9e`; the carry header and the `--round` flag → `BUGS.md` |
| `a-finding-can-leave-an-audit-round` | `a-finding-can-leave-an-audit-round-measurements.md`, `a-finding-can-leave-an-audit-round.md` | `da2737b9` | landed: an evidenced wontfix/duplicate clears its audit round (`7c4befca`), fenced unattended; the Prior Round carry of dispositions → `BUGS.md` |
| `next-action-and-check-tell-the-truth` | `next-action-and-check-tell-the-truth-measurements.md`, `next-action-and-check-tell-the-truth.md` | `5a2446d7` | landed: ground `--check` fails on FAIL (`f1ca2f84`, `bd864c3a`), a check that could not run keeps its class (`6f069d96`); the no-task-file case and `--status` next_action → `BUGS.md`; ground `next_action` was not taken |
| `hooks-fence-the-target` | `hooks-fence-the-target-measurements.md`, `hooks-fence-the-target.md` | `9d71707e` | the write-deny hook judges a batch op by op (`fcec0da0`) and the role write-allow hook the same; the spawn-time test-file permission and `test_globs` layering were not taken (no reproduction) |
| `refusals-that-name-nothing` | `refusals-that-name-nothing-measurements.md`, `refusals-that-name-nothing.md` | `1000ab5c` | its reproduced refusals (`--merge` blaming format, the ground pairing refusal) → `BUGS.md`; the rest dropped |
| `round-knows-its-panel` | `round-knows-its-panel-measurements.md`, `round-knows-its-panel.md` | `4760daf1` | → `BUGS.md` (`--check` converges with roles that never ran) |
| `an-unreadable-file-is-named` | `an-unreadable-file-is-named-measurements.md`, `an-unreadable-file-is-named.md` | `76a343bc` | its reproduced items (the `tp.lens` typo, the spec cut inside a rune) → `BUGS.md`; the rest dropped, no field instance |
| `brief-carries-the-forcing-sentences` | `brief-carries-the-forcing-sentences-measurements.md`, `brief-carries-the-forcing-sentences.md` | `adf50471` | dropped: it adds prompt text to the loop this release shrinks, on one trial |
| `what-the-record-does-not-say` | `what-the-record-does-not-say-measurements.md`, `what-the-record-does-not-say.md` | `e7843978` | dropped: one field instance, and it adds loop surface |
| `a-round-can-be-driven-from-the-envelope` | `a-round-can-be-driven-from-the-envelope-measurements.md`, `a-round-can-be-driven-from-the-envelope.md` | `6d378ba8` | dropped: its cost was mechanism only, with no field case |
| `a-rewrite-names-what-it-dropped` | `a-rewrite-names-what-it-dropped-measurements.md`, `a-rewrite-names-what-it-dropped.md` | `08006d68` | dropped: unproven that a listing tells loss from rewording; a decision note if the loss recurs |
| `red-gate-procedure` | `red-gate-procedure-measurements.md`, `red-gate-procedure.md` | `2db6aa0c` | dropped: a doc note with no defect behind it |
| `mutation-run-check` | `mutation-run-check-measurements.md`, `mutation-run-check.md` | `2db6aa0c` | dropped: `CLAUDE.md`'s mutation rule covers the run |
| `repair-locality` | `repair-locality-measurements.md`, `repair-locality.md` | `78bdda35` | dropped: the number saturates and gates nothing; its script source is in the sidecar |
| `12-repair-locality` | `12-repair-locality.md` | `2974fe44` | forwarding stub for `repair-locality`, dropped with it |
| `decomposition-is-built-before-import` | `decomposition-is-built-before-import-measurements.md`, `decomposition-is-built-before-import.md` | `dd1fe2fe` | forwarding stub; its rule shipped as `skills/tp/SKILL.md` Step 2 |
| `floor-anchors-need-fixtures` | `floor-anchors-need-fixtures-measurements.md`, `floor-anchors-need-fixtures.md` | `997eeed8` | forwarding stub for `the-floor-names-what-it-cut` |
| `scratch-name-is-unique-per-spec` | `scratch-name-is-unique-per-spec-measurements.md`, `scratch-name-is-unique-per-spec.md` | `8d74c196` | forwarding stub; the defect → `BUGS.md` |
| `loops-own-state-writes` | `loops-own-state-writes-measurements.md`, `loops-own-state-writes.md` | `ee538cff` | forwarding stub for `a-findings-exits-agree` |
| `round-records-the-text-it-read` | `round-records-the-text-it-read-measurements.md`, `round-records-the-text-it-read.md` | `b7977073` | forwarding stub; its emit-time hash went to `reconcile`, which stays |
| `what-the-carry-can-promise` | `what-the-carry-can-promise-measurements.md`, `what-the-carry-can-promise.md` | `e7843978` | forwarding stub for `what-the-record-does-not-say` |
| `record-diagnoses-every-bad-row` | `record-diagnoses-every-bad-row-measurements.md`, `record-diagnoses-every-bad-row.md` | `1000ab5c` | forwarding stub for `refusals-that-name-nothing` |
| `the-guard-pins-the-whole-listing` | `the-guard-pins-the-whole-listing-measurements.md`, `the-guard-pins-the-whole-listing.md` | `1000ab5c` | forwarding stub for `refusals-that-name-nothing` |
| `guards-read-what-production-reads` | `guards-read-what-production-reads-measurements.md`, `guards-read-what-production-reads.md` | `1000ab5c` | forwarding stub; dropped on 2026-09-08 |
| `an-invalid-task-file-is-reported` | `an-invalid-task-file-is-reported-measurements.md`, `an-invalid-task-file-is-reported.md` | `76a343bc` | forwarding stub for `an-unreadable-file-is-named` |
| `context-is-cut-on-a-rune-boundary` | `context-is-cut-on-a-rune-boundary-measurements.md`, `context-is-cut-on-a-rune-boundary.md` | `76a343bc` | forwarding stub for `an-unreadable-file-is-named` |
| `two-advisories` | `two-advisories-measurements.md`, `two-advisories.md` | `ae1ef2fd` | forwarding stub, split on 2026-09-11 |
| `ground-command-friction` | `ground-command-friction-measurements.md`, `ground-command-friction.md` | `b73ce3f3` | forwarding stub, split on 2026-09-08 into the ground specs |
| `README` | `README.md` | `7b9f6444` | the roadmap; replaced by `BUGS.md` and the decision candidates in this directory |
| `gate-sequence` | `gate-sequence-measurements.md`, `gate-sequence.md` | `7b9f6444` | archived after v1.2.0: its field half shipped there (the unrunnable-gate warning at init/import, the per-spec `quality_gate` setter); what remains, a `tp gate` convenience verb and an internal executor merge, has no measured cost |
| `epics` | `epics-measurements.md`, `epics.md` | `0af63ab6` | archived on 2026-11-14: shipped as the epic layer in v1.3.0 (`spec/1.3.0.md`, `spec/1.3.0-measurements.md`) |
| `epics-progress-field` | `epics-progress-field.md` | `ed0390d6` | archived on 2026-11-14: answered in v1.6.0 (`spec/1.6.0.md`) |
| `epic-review-at-scale` | `epic-review-at-scale.md` | `c04556f2` | archived on 2026-11-14: shipped in v1.7.0 (`spec/1.7.0.md`) |
| `dashboard` | `dashboard.md` | `48d79e6d` | archived on 2026-11-14: shipped as `tp dashboard` in v1.8.0 (`spec/1.8.0.md`) |
| `lifecycle-timestamps` | `lifecycle-timestamps.md` | `25efcf79` | archived on 2026-11-14: shipped in v1.8.0 (`spec/1.8.0.md`) |
| `epic-scope` | `epic-scope.md` | `5cff39ff` | archived on 2026-11-14: `tp epic scope` and the `gate-scope` role shipped in v1.9.0 (`spec/1.9.0-measurements.md`) |
| `deferrable-required` | `deferrable-required.md` | `e93bf55b` | archived on 2026-11-14: shipped in v1.10.0 (`spec/1.10.0-measurements.md`) |
| `epic-reemit-guard` | `epic-reemit-guard.md` | `f461f7cd` | archived on 2026-11-14: shipped in v1.10.0 (`spec/1.10.0-measurements.md`) |
| `epic-role-attribution` | `epic-role-attribution.md` | `44a79bd5` | archived on 2026-11-14: `--record-inbox` shipped in v1.10.0 (`spec/1.10.0-measurements.md`) |
| `epic-description` | `epic-description.md` | `b76c0979` | archived on 2026-11-14: shipped in v1.11.0 (`spec/1.11.0-measurements.md`) |
| `run-regression-unit` | `run-regression-unit.md` | `f5c7ba0a` | archived on 2026-11-14: shipped in v1.12.0 (`spec/1.12.0-measurements.md`) |
| `hand-recorded-staleness` | `hand-recorded-staleness.md` | `c0942f84` | archived on 2026-11-14: shipped in v1.12.0 (`spec/1.12.0-measurements.md`) |
| `ground-scratch-in-state-dir` | `ground-scratch-in-state-dir.md` | `8d90c0e1` | archived on 2026-11-14: shipped in v1.12.0 (`spec/1.12.0-measurements.md`) |
| `role-files-in-state-dir` | `role-files-in-state-dir.md` | `eaba1a58` | archived on 2026-11-14: shipped in v1.12.0 (`spec/1.12.0-measurements.md`) |
| `project-layer-from-task-file` | `project-layer-from-task-file.md` | `4e008d58` | archived on 2026-11-14: shipped in v1.12.0 (`spec/1.12.0-measurements.md`) |
| `dashboard-ux` | `dashboard-ux.md` | `0cbef316` | archived on 2026-11-14: shipped in v1.13.0, field fixes in v1.14.2 and v1.17.1 (`spec/1.13.0-measurements.md`, `spec/1.14.1-measurements.md`, `spec/1.17.1-measurements.md`) |
| `reconcile` | `reconcile-measurements.md`, `reconcile.md` | `4d31e859` | archived on 2026-11-14: a rewritten spec starting an epoch shipped in v1.14.0 (`spec/1.14.0-measurements.md`); `reconcile-measurements.md` last changed at `b7977073` |
| `carried-rows-block` | `carried-rows-block.md` | `26e2d985` | archived on 2026-11-14: shipped in v1.16.0 (`spec/1.16.0-measurements.md`) |
| `dashboard-graph` | `dashboard-graph.md` | `1786ff7f` | archived on 2026-11-14: shipped in v1.17.0 (`spec/1.17.0-measurements.md`) |
| `audit-carried-fail-blocks` | `audit-carried-fail-blocks.md` | `a60ccdd3` | archived on 2026-11-14: shipped in v1.18.0 (`spec/1.18.0-measurements.md`) |
