# tp — A rewrite names what it dropped

A backlog spec, named by slug; its priority number and its release number are assigned later. Its
measurements are in `a-rewrite-names-what-it-dropped-measurements.md` beside it, and this file stands
without them. It is **prototype-first**: §2 is its first task, and it names the result that drops the
release.

Class: **tool** — it adds a read-only listing to `tp ground`; the listing records nothing and gates
nothing.

## 1. The decision

**Context.** Three things read a spec, and none of them can see a sentence that is no longer in it.
`tp ground` grades the units the current text has. `tp review` tells every role the spec is complete
and authoritative. `tp lint` checks form. `tp review --diff-from` matches sections by their exact
heading: a modified section is shown with its current text only, and a removed or renamed section is
shown by its heading alone. So a rule dropped from a section that survives, or from one that was
renamed, reaches no reviewer. A field report (WB-3155) rewrote a spec and then, comparing the old
version by hand, found rules that had gone silently — one of them a user-visible behaviour change a
PR description had to state. Part of what first looked lost had moved into the spec's decisions
sidecar. This repository's own Step 0.5 rewrite of a backlog spec dropped a dependency sentence the
same way, and the next commit on that file put the dependency back in another form (sidecar, *Field
report WB-3155* and *The prototype*).

**Decision.** `tp ground <spec> --units --against <ref>` lists the floor units of the spec as it
stood at `<ref>` whose text is in no unit of the current spec or of a counterpart file. It is
read-only: it emits no round and writes nothing. Whether a listed unit still has a counterpart in
other words is for the agent to judge. tp lists and orders, and never takes a unit out of the
listing.

1. **`<ref>`** is anything `git show` accepts before `:<path>`. A bare ref reads the spec's own path
   at that ref. For a renamed spec, `<ref>:<path>` names the old path.
2. **Counterpart files.** The `<base>-measurements.md` beside the spec is a counterpart when it
   exists. A repeatable flag names more, so a project that keeps its decisions trail somewhere else
   can name that file.
3. **Order.** Each listed unit carries the highest word overlap it has with any sentence of the
   current spec or of a counterpart, and the listing is sorted on that value, lowest first. So a
   sentence that was dropped sorts ahead of one that was only reworded. The overlap orders the
   listing; it never filters it.

**Consequences.** A rewrite can be checked against its predecessor in one call, before the ground
round that would otherwise grade only what survived. How long the listing is depends on how much the
rewrite changed. A reworded sentence has a new text hash, so a rewording pass lists most of the old
floor. A rewrite that changes the spec's scope lists nearly all of it. The order is what makes the
first case readable. In the second case the agent is confirming a scope change it made on purpose.
Nothing reads the listing: no gate, no `--check`, no convergence signal.

**Alternatives.**
- *A new command, as the report proposed.* Rejected: the floor, its units and their text hashes
  already belong to `tp ground`, and `--units` already prints them.
- *Matching by meaning.* Rejected: it turns a listing the agent reads into a verdict the tool
  delivers, and that verdict is one more claim nobody has run.
- *A threshold on the overlap.* Rejected: any cutoff hides the dropped rule that happens to share
  its words with a kept one.
- *Giving `--diff-from` the text of removed sections.* Rejected as the answer: that shows a removed
  section, but a sentence dropped inside a surviving section still does not appear.

## 2. Prototype first, and what drops the release

The first task builds the listing and nothing else. It runs the listing on the rewrites named in the
sidecar's *The prototype*, plus at least one rewrite in the field's shape — a body cut down to its
decisions, with the forensics moved to a sidecar. For at least two of them it reads every listed unit
and marks each one *dropped*, *reworded* or *moved*.

**The release is dropped if, on any rewrite read in full, a unit marked *dropped* sorts into the
lower half of the listing.** Finding that unit would then mean reading most of the listing, which is
the same comparison the field author made by hand, only as long as the old floor. The sidecar's
prototype is the first measurement against this condition. It is not a pass: it computes the listing
from `tp ground --units` output rather than running the built flag, and only two of its rewrites were
read in full.

## 3. Non-Goals

1. **No semantic matching and no threshold.** Word overlap orders the listing and does nothing else.
2. **No new round type, no recorded state, no gate, no `--check`.**
3. **No change to `--diff-from`.**
4. **The listing covers the floor at `<ref>`** — the units a ground round would have graded. What the
   floor cut is the subject of `the-floor-names-what-it-cut`.
5. **No judgement of intent.** A drop the author meant is listed like any other drop; saying which
   ones were meant is the agent's job.

## 4. Tests

Every row derives from a numbered decision and names a mutant that must fail it. The flag does not
exist at `HEAD` — `tp ground <spec> --units --against HEAD` exits 2 with `unknown flag: --against` —
so each row's value under its mutant is left to the implementing task's acceptance. The fixture for
every row is a git repository with two commits of one small spec. Version 1 holds sentences A, B and
C; from version 1 to version 2, A is kept verbatim, B is reworded and C is dropped, and version 2 adds
one sentence ahead of all three, so unit ids shift.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §1 | `--against HEAD~1` lists exactly B and C | compare units by id instead of by text, which lists A because its id moved |
| 2 | §1.2 | with C moved verbatim into `<base>-measurements.md`, the listing is B alone | ignore the sidecar |
| 3 | §1.2 *flag* | with C moved into the second of two files named by the repeatable flag, the listing is B alone | read only the first value of the flag |
| 4 | §1.3 | with B before C in version 1, the listing puts C first | sort by unit id at `<ref>` |
| 5 | §1.3 *no filter* | a dropped sentence whose words all reappear, spread over other sentences, is still listed | drop units above an overlap cutoff |
| 6 | §1.1 | on a spec renamed since `<ref>`, `--against <ref>:<old path>` lists against the old path, and a bare `<ref>` exits non-zero naming the path it could not read | ignore the path half of `<ref>:<path>` |
| 7 | §1 *read-only* | with a `.tp-review/` directory present, every file under it is byte-identical afterwards and none is created | produce the listing through the emission path, which writes a snapshot |
