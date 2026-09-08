# two-advisories — measurements

Supplemental material for `two-advisories.md`; the spec stands without it.

## The version comparison is the wrong predicate

The original specification compared the running binary's version against *the newest spec version
under development*. Measured on this repository, that predicate is wrong in both directions. Every
figure below is given with the command that derives it.

**What Go reports is the last tag with the patch incremented**, followed by a pseudo-version suffix
carrying the commit time and the commit itself:

```
$ git describe --tags --abbrev=0
v1.0.0
$ git tag -l v1.0.1 | wc -l
0
$ go build -o tp ./cmd/tp && ./tp --version
tp version v1.0.1-0.20260904005015-c9f19088623a+dirty
$ git rev-parse --short=12 HEAD
c9f19088623a
```

The number a correct build reports is therefore *above* the newest release rather than at it, and
that bounds the false positive: it fires on a spec numbered above the incremented patch, and not on
one numbered at or below the last tag.

**False positive — it fires on the correct setup.** The repository's only shipped comparator,
`hooks/session-start.sh`'s `version_below`, puts `1.0.1` below `1.44.0`: the advisory fires on
exactly the binary the rule asks for, built seconds earlier from `HEAD`. The string cannot rescue it,
because the timestamp inside a pseudo-version is the **commit** time — `go version -m` on the binary
above prints `vcs.time=2026-09-04T00:50:15Z`, which is that commit's — so how recently the binary was
built is not readable from the version at all.

**False negative — the information is present and the predicate discards it.** Two binaries built in
a clone outside the repository, at two commits this repository holds:

| built at | `tp --version` | `vcs.revision` |
|---|---|---|
| `65d0b09f2800` | `v0.37.1-0.20260902095645-65d0b09f2800` | `65d0b09f280084b9b6b4bb4588ef217c12701419` |
| `10dee489714a` | `v0.37.1-0.20260902102453-10dee489714a` | `10dee489714a8522e6f3609c82c9dd76187c1501` |

**The two commits are eleven apart**: `git rev-list --count 65d0b09f2800..10dee489714a` returns 11,
and `--first-parent` returns 11 as well. Each row of the table is produced by
`git checkout <rev> && go build ./cmd/tp` in the clone, and its third column by `go version -m` on
the result.

**The two version strings differ, and one of the fields they differ in is the revision.** Ground
round 2 deleted a sharper sentence that used to stand here — *"they differ in exactly the revision"*
and *"the third column can be read straight off the second"* — and both halves were false against the
table directly above: the strings also differ in the pseudo-version's **timestamp** field, and the
second column carries a twelve-character prefix while the third carries the full forty-character sha
that §3 insists on. What the table needs is only that the strings differ and that the revision is one
of the reasons.

The predicate is blind not because the information is absent but because `version_number` strips
everything from the first `-`, reducing both to `0.37.1`. That is the correction the refutation rests
on: a version predicate here is not short of data, it is comparing a prefix that throws the
discriminator away.

**The discriminator is the commit, and it is already embedded.** `internal/cli/root.go:56` already
calls `debug.ReadBuildInfo()` for `info.Main.Version`; the same `BuildInfo` carries `vcs.revision` and
`vcs.modified` in `Settings`, put there by `buildvcs`, which is on by default. Two qualifications on
reusing that call: it sits inside `if version == "dev"` (`root.go:55`), so a binary whose version was
set by ldflags never reaches it and the advisory needs its own unguarded call; and the production
build `CLAUDE.md` documents, `go build -ldflags="-s -w"`, keeps the settings — probed, all four
`vcs.*` entries survive the strip.

## What is testimony

> **This file is decisions.** Its central decision was rejected in its first form by the operator the
> defect happened to. **That account is testimony and nothing here corroborates it** — no earlier
> draft exists in history to compare against, and the incident appears nowhere else in `spec/`,
> `skills/` or `CLAUDE.md`. §2.1 quotes the refutation because the rejected form is the one an
> implementer would otherwise write, and then gives the argument that does **not** rest on the
> testimony — a relation between two sentences, checkable by reading them. §5 is the ledger of what
> is reported and what is checked.

Three of the untracked-task-file advisory's premises are the operator's retrospective self-report
about a repository this cycle cannot reach and a decision that cannot be rerun. Each is marked where
it is used, and none is corroborated by any artifact here:

1. **the loss itself** — the project, its naming convention, its `.gitignore` line, the deletion
   and the fourteen tasks;
2. **the attribution of the blockquote** to the operator who lost the file, and that it was a
   response to a first draft — no draft-and-response pair exists in history;
3. **"that session ran tp dozens of times"**, the premise under the habituation argument.

They are recorded as testimony, not measurement, and they are what motivated the release rather than
what it rests on.

**What does not depend on any of them.** The orthogonality argument is a claim about two
propositions, checkable by reading them. The trigger is a fact about `DetectPhase`. The predicate is
a fact about `git ls-files`. Every test row is a fact about a mechanism a test can run.

The stronger assertion, that the reframed sentence *would* have prevented the deletion, is
unfalsifiable and is nowhere relied on. A test can show the advisory fires with the right numbers at
the right moment; no test can show it would have been obeyed.

## Routed here at the 2026-09-08 re-verification

One item from the candidates files lands on this spec's subject. It is recorded in this sidecar; the
spec body is not edited.

- **The raw-stderr advisory sweep.** A run of advisories is written straight to `os.Stderr` rather
  than through `output.Notice`, so `--quiet` cannot silence them and no test can intercept them at
  the helper. Ten files carry them: `internal/cli/review_merge.go`, `audit_merge.go`,
  `review_verify.go`, `config.go`, `done.go`, `commit.go`, `commitstrategy.go`, `changewarn.go`, and
  `internal/engine/configresolve.go`, `discover.go`. **The counting rule decides the number, so it is
  written down rather than the number alone:** `fmt.Fprint*(os.Stderr` over exactly those ten files
  returns **18** call sites at `dd89c566` — 3, 4, 1, 1, 2, 1, 2, 1, 2, 1 in the order listed. A bare
  `os.Stderr` grep over the same ten returns more, because `cmd.Stderr = os.Stderr` and stream
  restores match it; the re-verification survey reported **14** under a rule it did not state, and 14
  is not reproducible by either command. Re-run the `fmt.Fprint*` form rather than quoting any of the
  three figures. Source: `spec/0.33.0-candidates.md` item 3 and `spec/0.35.0-candidates.md` item 15,
  whose own "roughly twenty" is the right order of magnitude.

## Decided at the 2026-09-08 decision pass

From `spec/undecided.md`, *Which channel a degraded scan reports on*.

**Decided: a payload field always; the notice is additional.** A payload field survives `--quiet` and
a driver reads payloads, which a notice on stderr gives neither. Concretely, `runStateOf` in
`internal/cli/run_status.go` gains a **`lock_unreadable`** payload field: today it reports an
unprobeable run lock as a notice only, returning `in_flight` with no payload field, so under `--quiet`
it is indistinguishable from a live run. `internal/cli/validate_project.go` already reports on both
channels — an `output.Notice` and a `skipped` entry — and is the precedent the rule generalises.

**This also answers the channel question *The evidence contract* was holding**, which is why that
entry could be closed rather than left to state a general principle first. Routed from
`spec/0.35.0-candidates.md` item 17, which measured the second half by making `.tp/locks` a regular
file and running `tp run --status --quiet`.
