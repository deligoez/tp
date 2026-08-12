# tp v0.34.0 — Trustworthy writes, and less of everything else

## 1. Overview

This release has two jobs and one governing rule.

The first job is repair. tp sells one thing above all others: that the state it holds is durable and
true. Two write paths break that promise quietly, and a third defect means the quality gate cannot
see a whole class of bug in the tool that guards it.

| Defect | Today | Consequence |
|---|---|---|
| `tp import` writes the task file outside the write lock | Atomic but not mutually exclusive | A concurrent `import` + `add` lost the add's task in 1 of 20 runs, both processes exiting 0 |
| `tp add --bulk` / `tp set --bulk` drop lines past a 64KB cap | Warning on stderr, `exit 0` | A 3-row file with one 70KB line silently discards that row and every row after it, reporting success |
| The quality gate does not run `-race` | Green on a racy tree | The repo's only data race was invisible to the gate that is supposed to catch it |

The second job is subtraction. Thirteen releases of feature work have left duplicated logic,
overlapping documents, and tests that assert nothing a mutation would break. None of it fails a
gate, and all of it costs every future reader — human or agent — context they have to spend before
they can act.

The rule that governs both jobs is stated in §2 and applies to every task in this release. It is
scheduled **before** the unattended run of v0.35.0 for two reasons: every defect above is one an
unattended loop multiplies, and every line removed here is a line the loop never has to read.

## 2. Subtraction is the governing rule

**This rule binds decomposition, not the tool.** Nothing in tp parses closing evidence and this
release adds nothing that will. The rule is applied when the task list is reviewed, before
implementation starts, and it is the operator's call.

A task whose acceptance is a removal names what it removes and what still covers it. A task that
only adds belongs here when a section of this spec mandates the addition, and its acceptance cites
that section.

An agent asked to improve a codebase reliably adds: a helper, a flag, a wrapper, a test. Each
addition is locally defensible and the sum is the drift this release exists to reverse.

### 2.1 The removal contract

Nothing is removed on the grounds that it looks unused. Every removal states, in the closing
evidence, what now covers the thing removed:

| Removing | The evidence names |
|---|---|
| Code | That it had no caller, or the single remaining implementation that absorbed it |
| A test | The surviving test that fails when the original defect is reintroduced |
| Documentation | The one place the fact now lives |

The middle row is the one that needs a method rather than an assertion, and §7 gives it.

### 2.2 What is not the metric

Line count is not the metric. Test count is not the metric. A release measured on "fewer tests"
produces an agent that deletes the tests that were doing the work, because those are the ones with
the most lines and the most maintenance friction.

The metric is **discrimination**: does this artifact distinguish a correct state from an incorrect
one? Code that cannot be reached does not. A test that passes against a broken implementation does
not. A paragraph repeated in four documents discriminates once and misleads three times. Anything
that fails that question goes, and anything that passes it stays no matter how long it is.

More tests are welcome. Tests that discriminate nothing are not.

### 2.3 The release records what it removed

The release notes carry a before-and-after count: source lines, test count, exported symbols,
documentation lines, and the number of documented facts that had more than one home. Not as a target
to hit — as a recorded fact, so the next cycle can see whether this worked or whether the codebase
simply grew in a different direction.

## 3. `tp import` writes under the lock

Two task-file write paths run outside `WithFileLock`. `importcmd.go` reads the target twice and
writes it through `WriteTaskFile` with no lock at all. `init.go` guards with a stat and then writes,
and `tp add --spec` reaches that same path by calling `runInit` before taking its own lock. v0.33.0
made these writes atomic, which removes torn reads but not lost updates. Atomicity is not mutual
exclusion.

Both read-modify-writes move inside `WithFileLock`, on the same terms as every other write command,
honouring the resolved `lock_timeout_seconds`. These are the only two remaining unlocked task-file
writes; after this section there are none.

A lock held past the timeout fails the same way every other write command already fails it: **exit 4
(`ExitState`)**, with `LockTimeoutError`'s hint naming the lock path and the elapsed wait. The
success path gains no flag and no output change.

## 4. The bulk readers honour the shared line cap

Every reader in the review and audit family was swept in v0.33.0 to abort with exit 3 at a shared
1MB cap. `tp add --bulk` and `tp set --bulk` were left out because each pins its own
warn-and-continue contract in its own tests. The exemption was recorded at the call sites with
`line-cap:` markers, so this is a deferral being collected, not a newly found bug.

Both bulk readers adopt the shared cap and the shared failure mode: a line over the cap aborts the
whole operation with exit 3, naming the file and the line number, and writes nothing. Partial
success that reports total success is the failure being removed; a bulk operation either applies
every row or applies none. The `line-cap:` markers come out with the exemption, and so does the
second reader implementation if the two turn out to be the same code twice.

## 5. The quality gate sees data races

tp's own gate is `go test ./... && golangci-lint run`. Go's race detector is off by default, so a
data race in tp cannot fail the gate. The v0.33.0 audit found one — in a test written during that
audit to guard against a different defect — and the gate had been green over it the whole time.

The project gate in `.tp/config.json` becomes `go test -race ./... && golangci-lint run`, and
`skills/tp/SKILL.md` recommends `-race` for any Go project using tp. Measured on this tree, `-race`
costs between a tenth and a half again of the suite's wall time.

A task-level `quality_gate` wins over the project default, so changing the project layer alone does
not change what this release's own closes run. **The outcome required is that the gate resolved for
this release's task file runs `-race`, and that a test fails if that resolved value changes at all.**
Which override to remove and by what route are settled against the code at decomposition; four
routes have been proposed in review and three were disproven by measurement, which is the evidence
that this is not a question prose can close.

Two constraints belong here, because both are policy rather than mechanism:

1. The guard compares the resolved value against the literal gate string. Comparing it against the
   project value passes when the project gate weakens, and a substring check for `-race` passes when
   the rest of the gate changes.
2. `CLAUDE.md` and `skills/tp/SKILL.md` currently instruct `tp init --quality-gate "…"`. Both recipes
   drop the flag, so a later cycle's `tp init` does not re-create an override for any task file.
   Only the recipes change; §8.3 governs where the flag stays documented.

## 6. The code sweep

### 6.1 Duplication

`golangci-lint` runs `unused`, `unparam` and `gocritic`, so an orphaned symbol or an unused
parameter already fails the gate. Nothing detects the same logic written twice in two files, which
is the form duplication actually takes here: two readers with one contract, two prompt builders
differing in one string, two resolution paths for one config value.

`dupl` is enabled in `.golangci.yml` over production and test files alike, at its own default
threshold, which is not raised. Every clone it reports is either merged or exempted with its reason
recorded at the site; a clone whose merge would require a new abstraction is exempted, because
Non-Goal 7 forbids inventing one. Which clones fall which way is discovered by running the linter,
not decided here.

`.golangci.yml` is part of the shared quality gate, which runs at every close, so enabling `dupl`
and bringing the tree clean under it are **one task**.

### 6.2 Redundant abstraction

Indirection that exists because a second caller was expected and never arrived takes three forms
here: a helper with one call site, an interface with one implementation, and a parameter every
caller passes the same value for. Each is inlined unless a second caller exists today and can be
named.

This is the class the v0.32.0 lesson warned about from the other end — a repair that introduces a new
abstraction becomes the next round's audit surface. The same abstraction, once nothing came of it,
becomes permanent reading cost.

### 6.3 Dead branches

Error branches for states the type system already excludes, `default` arms after an exhaustive
switch, and nil checks on values constructed non-nil in the same function are removed rather than
tested. A branch that cannot execute is not defensive; it is a claim the reader has to verify.

## 7. The test sweep

The v0.33.0 audit found three tests written as guards that were not guards: one failed against its
own target defect 1 run in 10, one passed against a deliberately nonsense implementation because a
different mechanism satisfied its only assertion, and one was the repo's sole data race. Each was
caught only because an independent role re-derived the claim by mutation. That is the instrument
this sweep uses.

### 7.1 The mutation criterion

A test earns its place if a targeted mutation of the code it covers makes it fail. The procedure is
manual and narrow — break the thing the test names, run that test, watch it fail, restore. It is not
a full mutation-testing run over the suite; that is v0.36.0's optional gate composition, and it is
too slow to be the mechanism here.

Scope is a fixed list, not a description — this repository holds of the order of a thousand test
functions and "written to prevent a defect from returning" describes all of them or none. The
exact figure is deliberately not stated: this release removes tests, so a number written here would
be falsified by the work it describes, and the only obvious repair would be to edit it. The release's **first task** writes
the list to `spec/0.34.0-guard-tests.md`, one `file:function` per line, over the test functions
**added** between the `v0.32.0` and `v0.33.0` tags. That window is the evidence-driven one — three of
the guards written during it were found not to guard — and two tags bound it without any judgement
about which commit was an audit round. The exact rule and the resulting count are that task's
output, recorded in the file itself; it is of the order of a hundred functions, which makes triage
ordering part of that task rather than an afterthought.

One hazard belongs here rather than in acceptance, because it fails silently: this repository
configures an external git diff driver, so a derivation that does not pass `--no-ext-diff` returns
an empty list and the task closes green over it. An empty guard list is a failed derivation, never a
result.

The observer for that rule is the first task's own acceptance, and it has to be: registered
`workflow.checks` run in the review phase only, while the list is written afterwards, so review
would only ever see the check reporting `PENDING`. **That task's acceptance is therefore to run
`scripts/check-measured-claims.py` to exit 0 with no `PENDING` line**, which is reachable only once
the list is written, matches the derivation as a set, and the placeholder declaration is deleted.

The check is deliberately **not** registered. It was, for eight rounds, and the registration could
never verify the list — the artifact does not exist during review — while suppressing an entire
finding class for every reviewer. It did guard the window, rule and count claims in that time, so
the trade was real rather than free; it was still the wrong one, and the first round after
unregistering surfaced three stale measured claims the suppression had hidden.

The sweep is capped at that list plus any test a removal decision depends on.

A guard that survives its own mutation unchanged is either rewritten to assert the thing it claims
to, or removed with the surviving coverage named.

### 7.2 Redundant assertions

Several behaviours are asserted by many tests because each round of a past audit added one. Where
several tests assert the same property through the same path, one is kept — the one whose failure
message identifies the defect best — and the rest go. Where they assert the same property through
*different* paths, all stay: that is coverage, not duplication.

### 7.3 Implementation-detail assertions

Tests that pin internal structure — an exact log string, a private helper's shape, a map's iteration
order — fail on refactors that break nothing. They are converted to assert the observable contract
or removed. This is the class that makes the codebase expensive to change, which is the cost this
release is paying down.

### 7.4 What the sweep must not touch

The equality tests introduced in §8, the config guard of §5, and any test whose failure would let a
silent-write defect return. When in doubt the test stays; the contract in §2.1 requires naming the
surviving coverage, and "I could not name one" is the answer that keeps a test.

## 8. The documentation sweep

### 8.1 One canonical home per fact

`README.md`, `skills/tp/SKILL.md`, `CLAUDE.md` and the reference each describe overlapping parts of
the same tool, and they have drifted against each other. Four documents stating a fact means three
of them will eventually be wrong, and the reader cannot tell which.

The canonical home is **the document the actor performing that task already has loaded**, so
pointers run from the rarely-loaded document to the always-loaded one and never the reverse.
Ownership assigned by topic alone would move a fact an agent needs mid-cycle out of SKILL.md, which
is loaded, into README.md, which is not — trading one drift for one extra read per use.

| Document | Owns | Loaded by |
|---|---|---|
| `README.md` | What tp is, install, a one-line-per-command index for discovery | A human, once |
| `skills/tp/SKILL.md` | Everything needed mid-cycle: command forms, the flag inventory, workflows, decomposition, close recipes | An agent, every cycle |
| `CLAUDE.md` | This repository's own conventions and its self-development loop | An agent, every cycle, in this repo only |
| `skills/tp/REFERENCE.md` | Exhaustive field, exit-code and schema detail | Either, on demand |

The command index and the command forms are the one permitted overlap, because the two audiences
differ: a human scanning README needs to know a command exists, an agent needs its exact form. The
checked inventory of §8.3 lives in SKILL.md.

Anything duplicated across two homes is cut from the non-owner. `CLAUDE.md` in particular has
accreted rules that are not about this repository at all — those either move to SKILL.md, where
every user gets them, or are named in v0.36.0 as rules to mechanize.

One `CLAUDE.md` rule gains a qualifier rather than a home: the instruction to register a mechanical
check for a class seen in three rounds becomes conditional on the phase, since a check registered
where the thing it measures does not yet exist suppresses a finding class for every reviewer and
verifies nothing. §7.1 records the instance; this is where the rule itself is corrected.

### 8.2 The lint rule table is complete and stays complete

`README.md` documents fewer lint rules than tp can emit, and the two lists have drifted far enough
that this spec cannot state either one reliably. Whether the short list was curation or drift is no
longer worth deciding: the table lists every rule tp can emit, one line each.

The outcome required is that **a test fails when a rule tp can emit is undocumented.** Review has
already measured what that can and cannot be: rules are emitted from more than one package, so no
single enumeration makes absence imply non-emission; guaranteeing it by typing is the refactor
Non-Goal 7 excludes; and observing emissions over this repository's own specs surfaces only a
subset, so observation cannot source the set either. No fraction is stated for that subset — two
rounds put one in this sentence and both were wrong.

What remains achievable, and is therefore what this section asks for, is a collected list of rule
identifiers checked against the documented table, with the collection's own blind spot recorded
beside it. A test that compares two honestly-collected sets is worth more than a construction that
cannot be built, and stating the blind spot is what keeps the first from being mistaken for the
second.

### 8.3 Flags absent from every document

`--reason-file` and `--stdin` on `done` and `close`, `--docs-path` and `--test-path` (required with
their perspectives), `--base` on `audit`, and `--commit-strategy` on `init` appear in no document.
They are added to their canonical home, and the flag inventory gains the same kind of test: it fails
when a flag the CLI registers is undocumented, and it fails when the inventory documents a flag the
CLI does not register.

The inventory records what exists, not what to recommend. A flag whose recipe §5 drops stays in the
inventory, because the CLI still registers it and the test above fails if it does not. This is the
one place that governs the question; §5 states the recipe change and nothing more.

## 9. Exit-code consistency

### 9.1 An unknown command is a usage error

`tp <unknown-command>` exits 1 while `tp audit --bogus-flag` correctly exits 2. The documented scheme
says usage errors are 2, so tp is internally inconsistent and a driver cannot classify the failure.
Unknown commands and unknown subcommands exit 2.

### 9.2 The validation hint channel stops handing out task-file advice

`root.go` reports every validation error with the code-1 default hint, `run 'tp validate' to audit
the task file`. A malformed `--record` NDJSON therefore names an unrelated command and an unrelated
file. The message itself already names the real file and line, so nothing is unrecoverable — it
costs a round trip and it teaches the reader to distrust hints.

Validation errors raised by commands whose object is not the task file carry a hint naming their own
object. `TestFileErrorsCarryAHint` covers `ExitFile` sites today; it is extended to enumerate
`ExitValidation` sites the same way, so a new site cannot join the class silently.

### 9.3 `lint` names the spec, not the task file

`tp lint` takes a spec and emits the code-3 task-file default hint on a missing file — the same
defect the v0.33.0 sweep fixed across the review and audit family. It was recorded in
`taskFileCommands` with an honest reason rather than as a clean exemption. It adopts
`specFileMissingHint` and leaves the list.

Leaving that list makes `TestFileErrorsCarryAHint` enumerate every `ExitFile` site in `lint.go`, so
every hintless site there gains a hint in the same task — otherwise the gate closes red on the one
that was missed. How many such sites there are is counted when the task runs, not here.

`tp init` is not part of this: `runInit` never stats the spec, so it has no missing-spec path to
repair. Giving it one would be a new behaviour, which §11 excludes.

## 10. Two practices adopted from field feedback

Both are documentation only. They come from a field report reviewing tp's workflow against
Böckeler's *TDD inside the agent loop* experiment, and they need no code because tp already has the
machinery.

### 10.1 Example tables are the acceptance shape for behavioural tasks

A spec table of `input → expected output` rows is already first-class: `tp lint` extracts it,
decomposition must map every row to a task's acceptance, and `tp validate` checks the coverage. That
makes an example table authored during the review loop — with the human in the interview, and
adversarially reviewed by the panel — a spec-derived oracle that physically predates the
implementation.

This matters because tp orders test work after implementation, which is the right call on the
evidence but leaves the unit writing a test free to derive expected values by reading or running the
code under test. A tautological test passes, adds coverage and satisfies the gate, and nothing in
the pipeline can tell it from a real one. Ordering is not the defence; **the source of the expected
value is.** When the value comes from a table the review panel converged on, the test transcribes it
rather than inventing it.

The decomposition guidance in `skills/tp/SKILL.md` states this as the recommended shape for
behavioural acceptance, with the secondary benefit named: writing a concrete expected value forces
resolution of ambiguity that prose hides, which is exactly the friction that belongs in the
interview step.

### 10.2 Measuring whether a second model earns its cost

`overlap_report` already reports `unique` and `shared` cluster counts per role, and flags
`trim_candidate` when a role contributes nothing unique. Nothing restricts that instrument to the
lens axis. Running the same reviewer role in one round on one model and in another round on a second
model, then reading `unique`, answers a question most workflows can only guess at: does model
diversity add findings, or only redundant ones?

The recipe is documented in the reference, not in `skills/tp/SKILL.md`: its actor is the operator
deciding whether to pay for a second model, not the agent running a cycle, so §8.1's placement rule
puts it in the on-demand document. No feature changes; the instrument exists and the axis was simply
never named.

## 11. Non-Goals

1. **No behaviour changes outside the ones enumerated here.** This release intentionally changes
   observable behaviour in §3, §4, §5, §9.1, §9.2 and §9.3, and nowhere else. Everything in the
   three sweeps — §6, §7 and §8 — is behaviour-neutral, and a sweep that also improves things is a
   sweep nobody can review.
2. **`scope` on audit rows and `audit_converge_on`.** Still deferred for the reason v0.33.0 gave,
   and still carried in `spec/0.34.0-candidates.md`.
3. **No new commands or flags.** Every change is a fix, a removal, a hint, a test or a document.
4. **Mutation testing as a gate, the test-file fence, and `quality_gate` as a sequence.** All three
   belong to v0.36.0. §7 uses mutation as a manual instrument, not as tooling tp ships.
5. **The unattended run.** v0.35.0.
6. **No historical spec edits.** `spec/0.23.0.md` §8 says "tp never silently rebuilds the index",
   which v0.33.0 made false for the snapshot-only case. Historical specs are records; the living
   docs were corrected and that is where correctness is maintained.
7. **No rewrite disguised as a sweep.** If removing a duplication requires designing a new
   abstraction to hold the merged form, it is not a removal — it is v0.36.0's problem or a later
   version's.

## 12. What decomposition owns

Earlier drafts carried a numbered test list here, restating each section's requirement in different
words. Across four review rounds the largest single class of blocking finding came from that
restatement: the list and its section drifted, and each round's measurement overturned the previous
round's prose. The list is gone.

The sections above are of two kinds. §3, §4, §5, §8.2, §8.3 and §9 state an outcome that must become
observably true; the assertion proving it — the exact recipe, the exact exit code per entry point,
the exact set on each side of a comparison — is authored as task acceptance during decomposition,
where running the code settles it. §6.2, §6.3, §7.2, §7.3 and §8.1 state a judgement to apply rather
than a value to assert; their acceptance shape is §2.1's removal contract, not a test. This is tp's
own `over-specification` class applied to tp's own spec: a detail whose correctness can only be
established against code belongs in acceptance, not here.

Two rules govern that authoring, because both are policy rather than mechanism:

1. **An assertion accompanying a behaviour change must fail on today's tree.** One that already
   passes proves nothing about the change it accompanies; verify by running it before the fix. This
   binds the outcome sections only — a removal under §2.1 names a test that passes today, which is
   the opposite requirement and the correct one there.
2. **A test and the change it covers close in the same task.** The gate runs at every close, so a
   split leaves the earlier close red.

### 12.1 Existing tests this change invalidates

The bulk readers' warn-and-continue tests assert the behaviour being removed and move in the same
task as the change. `TestFileErrorsCarryAHint` gains cases rather than changing shape. The tests
removed under §7 are enumerated in their own tasks, each naming the surviving coverage, and are
never bundled with a behaviour change. The gate change makes the suite slower everywhere, which is a
cost, not a failure.
