# Candidates — split, and forwarded

**This file holds no content. It is a forwarding note, and it exists only so the references to it do
not break.**

It used to hold three kinds of thing at once — refuted candidates, undecided needs, and findings
carried out of an audit — and by the time `tp ground` had run over every pending spec, most of it was
either closed or about a different subject than the file's own name suggested. It was split on those
seams.

## Where everything went

| what it held | where it is now |
|---|---|
| **the ten findings carried out of v1.0.0's audit** | deleted — each was verified present in the release that took it before its entry was removed. They are, by today's names, `spec/backlog/checklist-covers-what-changed.md`, `spec/backlog/refusals-that-name-nothing.md` (which took both guard specs), `spec/backlog/ground-command-friction.md` and `spec/backlog/what-the-carry-can-promise.md` |
| **its ground-related material**, plus three defects the grounding programme measured in `tp ground` itself | **`spec/backlog/ground-command-friction.md`** — the ground command's own friction (the file was numbered `1.0.1` before that number shipped as a different release) |
| **the refuted candidates** — the unexecutable-split rule, the contradictory-comparator rule, the example-table rule, the corpus-replay gate as a procedure, the identifier pass, and `broken-cross-ref` extended across files | **`spec/undecided.md`**, `## Refuted` |
| **the undecided rows with no release** — the divisible round, the test-file fence, class families, the evidence contract, a registered check that outlives its release, the write-deny fence's reach | **`spec/undecided.md`**, `## Undecided` |
| **`t.Parallel()` in the test suite** | `spec/undecided.md`, *`t.Parallel()` in the engine package* — the `internal/cli` half is applied; the mutation check itself is `spec/backlog/mutation-run-check.md` |
| **a durable home for an accepted finding**, **making `severity` checkable**, **an audit-side `nonblocking_open`** | **`spec/backlog/a-finding-can-leave-an-audit-round.md`** (the accepted finding) and **`spec/backlog/round-knows-its-panel.md`** (role-scoped convergence); the audit-side `nonblocking_open` is in the latter's sidecar as a dropped counter |
| **a prior-round section for `tp review`** | **`spec/undecided.md`**, *A prior-round section for `tp review`* |
| **the `forward-spec-ref` prototype**, the one lint rule that survived | dropped on 2026-09-08 — its population vanished with the version numbers; **`spec/undecided.md`**, `## Refuted` |
| **the pre-registered forced-commitment trial**, which had run | `CLAUDE.md`, with the result |

**Coverage was checked before this file was emptied**, not asserted: every bolded table row and every
`###` heading of the previous revision — 24 distinct items — was located in a target file. Re-derive
against `git show HEAD~1:spec/candidates.md` if the split ever needs auditing.

## Why the stub stays rather than the file being deleted

29 references to this filename survive across the tree, in 11 files, and **six of them are in shipped
artifacts** — `spec/0.34.0.md`, `spec/0.34.0.tasks.json`, `spec/0.36.0.md` and
`spec/0.35.0-candidates.md` — which are the record of what was shipped and are not edited after the
fact. Deleting the file would break every one of those; editing them to chase a rename is the
renumbering cost this repository has already paid three times. A stub costs one small file and keeps
both rules.

Derive the count rather than trusting it: search the tracked, non-`.tp-review` files for the bare
filename, excluding `0.35.0-candidates.md` and `1.0.0-corrections.md`, which are different files.

**Note for a reader arriving from an old reference.** A citation into this file names a section that
no longer exists here. The table above says which file took that section's subject; the subject is
what to follow, never the section number — the sections were not preserved across the split, and a
number that resolves is not thereby correct.
