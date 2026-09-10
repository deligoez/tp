# tp — tp says when it could not read

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `an-unreadable-file-is-named-measurements.md` beside it, and this file stands
without them. The slug keeps its first subject, the review ranking's unreadable file; on 2026-09-11
the spec widened to the class and absorbed `an-invalid-task-file-is-reported.md`,
`context-is-cut-on-a-rune-boundary.md` and the raw-stderr sweep recorded in
`two-advisories-measurements.md`. The first two are now forwarding stubs.

Class: **tool** — every decision adds a report where a read failure is dropped today; no round is
graded differently and no convergence gate moves.

## 1. The decision

**Context.** tp reads inputs it cannot always read — a frontmatter key, a file, a subtree, a line, a
task file, a findings line — and at each site below it holds the failure and drops it: exit 0, empty
stderr, and a payload a caller cannot tell from a clean run. Every site was re-run at `HEAD`; the
reproductions are in the sidecar under *Re-verified at `18032abe`*.

The strongest instance: **a mistyped `tp.lens` key is reported by `tp lint` and ignored in silence
by `tp review`.** On a spec whose frontmatter carries `lens: implementr: [<question>]`, `tp lint`
prints one `frontmatter` warning naming the unknown key; `tp review` on the same spec exits 0 with
empty stderr, and no role's prompt carries the question. The round runs under the defaults the typo
left in place, and the operator who wrote the question believes it was asked. Spelled correctly, the
same command carries the question and prints a deprecation notice — so the channel exists, and the
typo is what silences it. A malformed `tp.audit_roles` focus does the same to `tp audit`, and a
frontmatter block that fails to parse as YAML does the same to both.

**Decision.** One rule, taken at the 2026-09-08 decision pass and recorded in
`two-advisories-measurements.md` under *Decided at the 2026-09-08 decision pass*: **when a command
drops an input it could not read, its payload says so always, and a notice on stderr says so
additionally.** The payload survives `--quiet` and is what a driver reads; the notice is for whoever
watches the terminal. Each section below applies that rule to one site.

**Consequences.** The emission payloads of `tp review` and `tp audit` gain keys (§2, §3). `tp lint`
gains one finding (§4). A set of stderr advisories moves to the notice channel, so `--quiet` now
silences them (§5). Two truncations move to a character boundary (§6). Two exit codes change, both
toward a refusal the same input already meets through a sibling command (§3).

**Alternatives.** *Refuse instead of report* — make a mistyped key or an unreadable doc exit
non-zero. Not taken: a round under the defaults is a valid round, and a reviewer handed a partial doc
tree has a partial view, not a broken input; whether to repair and re-emit is the operator's call.
*A notice alone*, which this spec's earlier body chose for the ranking drop, is what the decision
pass rejected: under `--quiet` it is indistinguishable from a clean run.

## 2. The spec's own frontmatter

`tp review` and `tp audit` report every frontmatter warning and error `tp lint` reports for the same
spec: in the emission payload under a `frontmatter_findings` key carrying lint's messages, and as one
notice each. The round is still emitted under whatever the frontmatter did resolve; what changes is
that the emission says what it could not use.

## 3. A file, a subtree, a line, a findings file

1. **The review ranking.** Under `--perspective documentation` or `test` with `--spec-inline` or
   `--diff-from`, a candidate file that cannot be read is dropped from the ranking with no word, and
   `reviewed_files` falls by one.
2. **The doc-tree walk.** An unreadable subdirectory under `--docs-path` is skipped: exit 0, empty
   stderr, and the `Documentation structure:` tree handed to the role lacks it — while a missing docs
   root refuses at exit 3. The coarse failure refuses; the partial one is indistinguishable from a
   smaller tree.
3. **The heading parse behind `--verify`.** A spec carrying one line longer than the line scanner
   accepts makes `tp lint` and `tp review <spec>` refuse at exit 3, while
   `tp review <spec> --verify --findings <file>` exits 0 and tells the verifier the spec has
   `0 sections`.
4. **The findings file behind `--verify`.** When every line of the findings file is malformed,
   `tp review --merge` refuses at exit 1, and `--verify` on the same file exits 0 with a prompt that
   opens *"Previous review rounds produced 0 findings"*. Both print the same per-line warnings; the
   difference is the refusal, and `--verify`'s payload carries no count that separates an empty file
   from a dropped one.

**Decision.** Sites 1 and 2 put each path they could not read, with its error, into the emission
payload under an `unreadable` key, and write one notice per path. A file dropped at the ranking is not
reported again by the content read that follows it: one anomaly, one report. `reviewed_files` keeps
counting what the prompt carries, and `unreadable` makes the difference legible. Site 3 refuses at
exit 3 as the plain emission does, and the refusal of all three commands names the over-long line,
where today's hint says to check the spec path. Site 4 refuses an input from which no line parsed,
as `--merge` does, and its payload carries the per-input `parsed`/`skipped` counts `--merge` already
emits.

## 4. The task file `tp lint` reads

`tp lint` reads the spec's task file only for its advisory `acceptance-quality` findings. An
unparseable task file and an unreadable one each yield no finding, empty stderr and exit 0 — the
same as a task file with nothing to report.

**Decision.** Either failure becomes one `task-file-invalid` finding at **`warning`** severity naming
the task file and the error, and the acceptance walk is skipped. Warning rather than error: lint's use
of the task file is advisory, and every command that needs the task file already refuses the same file
at exit 3, so an error here would add a second refusal and nothing a caller lacks. A missing task file
stays silent — it is the ordinary state before decomposition. The finding *is* lint's payload, so the
rule is met without a notice; `tp lint` has no advisory stderr channel and does not gain one.

## 5. Advisories `--quiet` cannot silence

A set of advisory lines is written straight to stderr rather than through the notice channel, so
`--quiet` does not silence them. The example that shows the cost: `tp review --merge --quiet` over a
file with one malformed line still prints `warning: skipping malformed line (invalid JSON)` — while its
payload already carries the same fact as `inputs[].skipped`.

**Decision.** Every advisory — a stderr line that does not end the command — goes through the notice
channel, which `--quiet` silences and JSON mode does not. Where the advisory reports an input dropped
from the command's result, the payload carries it too; for `--merge` it already does. An error envelope
written to stderr before a non-zero exit is not an advisory and is untouched. The site inventory, and
the counting rule that decides its size, are in the sidecar under *The raw-stderr sweep*.

## 6. A cut lands on a character boundary

1. **A lint finding's `context`** is capped at 80 bytes by a byte slice in `duplicate-line` and
   `duplicate-paragraph`. A multi-byte rune across the boundary is halved, and the emitted `context`
   ends in `U+FFFD` — text in no document — and is longer than the cap once encoded. **Decision:** the
   cap stays 80 bytes, and the cut lands on the last rune boundary at or before it.
2. **The review's finding identity** — category, location and the first 80 characters of the finding
   text, which decides what a later round carries forward as the same finding and what the report
   tracks across rounds — takes the first 80 *bytes*, where its own documentation promises
   characters. Two findings that differ inside their first 80 characters collapse into one when the
   difference lies past byte 80. **Decision:** the window is 80 characters, as documented.

The two caps differ on purpose: `context` is a size bound on output, the identity is a comparison
window.

## 7. Non-Goals

Non-Goals 1–3 keep the numbers they had before the 2026-09-08 split, because shipped files cite them.

1. **The audit's file selection is untouched.** The ranking in §3 is the review phase's; the audit's
   per-role file cap and what it selects are `checklist-covers-what-changed`'s. The two share the
   words "file selection" and nothing else.
2. **The ranking's read and the content read are not merged** into one pass. A file readable at one
   and not at the other stays possible; merging them is a refactor with its own cap and ordering
   decisions.
3. **`reviewed_files` is not redefined.** Whether an unreadable file was "reviewed" is a contract
   question; §3's key answers the operator's question without opening it.
4. **Nothing new is refused except §3's sites 3 and 4**, each of which meets a refusal today through a
   sibling command on the same input.
5. **`tp lint` does not validate the task file's schema.** A file that parses and breaks
   `tp validate`'s rules is `tp validate`'s to report.
6. **An unknown key directly under `tp:`** — `review_role:` for `review_roles:` — is reported by
   neither `tp lint` nor `tp review` today (sidecar). That is a missing lint rule, not a dropped
   report, and it is not this release. **Reopen condition:** a cycle that loses a round to one.
7. **The `context` cap is not raised and not made a character count.**

## 8. Tests

Every row derives from a numbered decision and names the mutant that must fail it. Rows whose
subject is a key or finding that does not exist at `HEAD` quote the `HEAD` observable the fix
changes; the count under the finished code is the implementing task's acceptance, per Step 0.5. Rows
1–1d keep the numbers shipped files cite.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §3 *ranking* | three candidate files, one unreadable, spec text carrying `## ` headings, under `--spec-inline`: `unreadable` has 1 entry and stderr 1 notice. `HEAD`: `reviewed_files` 2, stderr empty | the shipped bare skip |
| 1b | §3 *one report* | across the ranking and the content read, one unreadable file yields one notice and one `unreadable` entry | report at both sites |
| 1c | §3 *fixture* | the probe asserts the locked file is unreadable **before** it counts anything | run as a user who can read a `chmod 000` file, under which nothing is dropped and the test passes for the wrong reason |
| 1d | §3 *branch* | the probe asserts the spec content carries a `## ` heading before it counts anything | a fixture without headings, under which the ranking returns every file unread, the drop site is never entered, and the test passes with nothing dropped |
| 2 | §2 | a spec with `lens: implementr: [q]`: `tp review` exits 0, its payload's `frontmatter_findings` holds lint's message, and stderr holds one notice. `HEAD`: 0 stderr bytes, no key | the shipped emission, which never reads the frontmatter's warnings |
| 2b | §2 *control* | the same spec with `implementer` spelled correctly carries the question in the implementer prompt and no `frontmatter_findings` entry | report every frontmatter key, which turns every spec with a frontmatter into a warning |
| 2c | §2 *audit* | a spec whose `tp.audit_roles.<active id>.focus` is not a list: `tp audit` carries lint's message on both channels. `HEAD`: 0 stderr bytes. The fixture's role id is asserted active first | a fixture naming a role the corpus lacks, which draws the shipped "matches no active role" notice and passes for the wrong reason |
| 3 | §3 *walk* | `--docs-path` over `a.md` and an unreadable `sub/`: `unreadable` names `sub` and stderr carries one notice. `HEAD`: exit 0, stderr empty, tree without `sub` | the shipped walk, which returns nil on the entry error |
| 4 | §3 *long line* | a spec carrying one 70,000-character line: `--verify --findings f` exits 3, as `tp review <spec>` does, and the refusal names that line's number. `HEAD`: exit 0, `0 sections` | the shipped discarded parse error |
| 5 | §3 *verify input* | a findings file whose every line is malformed: `--verify` exits non-zero as `--merge` does, and a file with one good line carries `parsed` 1 / `skipped` 1. `HEAD`: exit 0, `0 findings` | the shipped verify read, which appends only what parsed |
| 6 | §4 | an unparseable task file and an unreadable one each yield one `task-file-invalid` finding at `warning` and lint exits 0. `HEAD`: 0 findings | the shipped `return nil` on each path |
| 6b | §4 *control* | a valid task file with one short acceptance yields one `acceptance-quality` finding and no `task-file-invalid`; a spec with no task file yields neither | report the missing file too, which turns every undecomposed spec's lint noisy |
| 7 | §5 | `tp review --merge --quiet` over a file with one malformed line writes 0 stderr bytes and still carries `inputs[].skipped` 1. `HEAD`: one warning line | the shipped raw stderr write |
| 8 | §6 *context* | a duplicated line of 78 ASCII characters and an em dash yields a `context` that is valid UTF-8 and at most 80 bytes. `HEAD`: 84 bytes, ending in two `U+FFFD` | the shipped byte slice |
| 8b | §6 *fixture* | the fixture's 80th byte is asserted to fall inside a rune before the assertion runs | an 80-ASCII fixture, on which byte and rune cuts agree |
| 9 | §6 *identity* | two findings of 30 `ş` + 20 `x` then different tails, recorded as round 1: round 2 carries 2 previous findings. `HEAD`: 1. Control: an 80-`q` prefix with different tails still carries 1 | the shipped byte window |
