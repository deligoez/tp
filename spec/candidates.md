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
| **the ten findings carried out of v1.0.0's audit** | deleted — each was verified present in the release that took it before its entry was removed. They are `spec/1.38.0.md`, `spec/1.54.0.md`, `spec/1.55.0.md`, `spec/1.56.0.md` and `spec/1.57.0.md` |
| **its ground-related material**, plus three defects the grounding programme measured in `tp ground` itself | **`spec/1.0.1.md`** — the ground command's own friction |
| **the refuted candidates** — the unexecutable-split rule, the contradictory-comparator rule, the example-table rule, the corpus-replay gate as a procedure, the identifier pass, and `broken-cross-ref` extended across files | **`spec/undecided.md`**, `## Refuted` |
| **the undecided rows with no release** — the divisible round, the test-file fence, class families, the evidence contract, a registered check that outlives its release, the write-deny fence's reach | **`spec/undecided.md`**, `## Undecided` |
| **`t.Parallel()` in the test suite** | **`spec/1.46.0.md`** — the `internal/cli` half is applied; `internal/engine` stays open because that is the package gremlins mutates |
| **a durable home for an accepted finding**, **making `severity` checkable**, **an audit-side `nonblocking_open`** | **`spec/1.50.0.md`** |
| **a prior-round section for `tp review`** | **`spec/1.52.0.md`** |
| **the `forward-spec-ref` prototype**, the one lint rule that survived | **`spec/1.58.0.md`** |
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
