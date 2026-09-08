# tp v1.44.0 — The binary check

> **This file is decisions.** The predicate this release was originally specified with — *compare the
> running binary's version against the spec being developed* — was prototyped while writing this file
> and **refuted in both directions**. §2 records that, because the refuted form is the one an
> implementer would otherwise write.

## 1. Overview

**The rule being mechanized:** *dogfood the in-progress binary, never the lagging release.*
`CLAUDE.md` has carried it since **v0.24.0** — `git log -S 'Dogfood the in-progress binary' -- CLAUDE.md`
gives one commit, `3e1a3f0a`, whose first containing tag is v0.24.0 and which `git show v0.23.0:CLAUDE.md`
does not have. It is violated by anyone who forgets to rebuild, and the
violation is silent — a stale binary produces confident output about behaviour it does not have.

tp emits **one advisory per invocation** when the binary running inside a tp repository is not the one
the working tree would build. Never an error, never an exit code, silenced by `--quiet`.

## 2. The version comparison is the wrong predicate

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

**The two version strings differ, and they differ in exactly the revision.** A string comparison of
the second column reports them different, and the third column can be read straight off the second.
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

## 3. The predicate

Inside a git repository that contains tp's own module, four states, decided in order:

| state | condition | advisory |
|---|---|---|
| **release** | no `vcs.revision`, and `Main.Version` is a real version | names that version and the rebuild command — this is a `go install`ed binary, not a build of this tree |
| **unattributable** | no `vcs.revision`, and `Main.Version` is `(devel)` | **nothing** |
| **stale** | `vcs.revision` present and ≠ `HEAD` | names both revisions and the rebuild command |
| **current** | `vcs.revision` == `HEAD` | **nothing** |

**A missing `vcs.revision` does not by itself mean a release**, which is why the second row exists.
Probed three ways in a copy of the tree outside the repository, reading each with
`go version -m <binary>`:

| build | `vcs.revision` | `Main.Version` | `tp --version` |
|---|---|---|---|
| `go install github.com/deligoez/tp/cmd/tp@v1.0.0` | absent | `v1.0.0` | `v1.0.0` |
| `go build -buildvcs=false ./cmd/tp`, inside the checkout | absent | `(devel)` | `dev` |
| `go build ./cmd/tp` in a copy with `.git` removed | absent | `(devel)` | `dev` |

The last two **are** builds of this tree, so the release advisory would assert the opposite of the
truth about them, and there is no installed version for it to name — `root.go:56` rejects `(devel)`,
which is why they report `dev`. `Main.Version` is the second signal that separates the three, and it
sits in the same `BuildInfo` as the revision. tp cannot tell whether such a build is current or
stale, so the honest advisory for that state is none.

**Both revisions are truncated by tp for display.** `vcs.revision` is the full 40-character sha
(`build vcs.revision=65d0b09f280084b9b6b4bb4588ef217c12701419` on a probed binary), not a short one.

**`vcs.modified` is not a trigger.** A dirty working tree is the normal state of development; the
binary is still the one this tree builds, up to uncommitted edits. A build from a dirty tree records
`vcs.modified=true` beside a `vcs.revision` equal to `HEAD`, so the flag is separable from staleness
and triggering on it would fire on every invocation from a tree with uncommitted edits.

**Commit distance is not computed, and the reason is that it changes no advice** — the rebuild
command is the same one commit behind or eleven. The cost argument is weaker than it looks and should
not be leaned on: the predicate above already needs `HEAD` on every invocation, and tp has no
in-process git (`grep -rn 'exec.Command("git"' --include='*.go' internal/ cmd/ | grep -v _test.go |
grep -v testdata | wc -l` returns 16, every one a subprocess), so a distance costs a *second* call,
not the first.

**A repository that is not tp sees nothing.** The gate compares the module path from `BuildInfo`
against the module path in the repository's own `go.mod`. Only the first side is already in hand: tp
reads no `go.mod` anywhere today (`grep -rn 'go\.mod' --include='*.go' internal/ cmd/ | grep -v
_test.go` returns nothing), so the repository side is a new input rather than a reuse. It is cheap
and exact, and it is still not a heuristic.

## 4. Not the plugin's version check, and the two must not be merged

`hooks/session-start.sh` compares `tp --version` against `.claude-plugin/plugin.json`'s version, runs
in `SessionStart`, and **fails** (exit 2, via `fail()`). This one compares the binary's *commit*
against the tree's `HEAD`, runs on any invocation inside the repository, and **advises**.

**They disagree on the case that matters, and that is why both exist.** A freshly installed release
developing the next version's spec passes the hook and trips this advisory. Reproduced on this
machine: `.claude-plugin/plugin.json` declares `1.0.0`, the installed binary reports `tp version
v1.0.0`, and the hook's own `version_below` puts `1.0.0` not below `1.0.0`, so the hook is silent —
while `go version -m $(command -v tp)` prints the module line and `-buildmode=exe` and no `vcs`
setting at all, which is §3's release state.

**This release introduces no version comparison, and that is a simplification rather than a saving.**
§3 compares two revision strings for equality, so nothing here is ordered. It cannot be claimed as
avoiding a second implementation of the shell's logic, because the repository already holds two
implementations of it:

- `hooks/session-start.sh` — `#!/bin/sh`, not bash. The comparator is `version_below`;
  `version_number` normalises a reported version to its dotted numbers and `version_part` extracts
  one component, and neither compares.
- `internal/cli/plugin_test.go` — `comparePluginVersions` with its helper `pluginVersionPart`
  re-implement the same dotted-numeric algorithm in Go, and three shipped tests use it
  (`grep -n 'comparePluginVersions' internal/cli/plugin_test.go` returns the definition plus three
  call sites).

## 5. Non-Goals

1. **Never an error.** No exit code changes, in any command, under any condition.
2. **No rebuild.** tp names the command; running it is the operator's.
3. **No merge with the plugin's version check.** §4 states why; merging makes one of the two wrong for
   its own case. This is the only no-merge clause here, and it is about the hook. The sibling
   advisory — the untracked task file — carries its own refusal to merge with *this* check, on its
   own side (`grep -rn 'No merge with the binary check' spec/` finds it there and nowhere here), so
   the pair is refused from one direction only and this clause does not speak for it.
4. **No version comparison anywhere in this release.** §3 compares two revision strings for equality.
   This is a simplification of the release, not a saving against a duplicate implementation: §4
   records that the repository already holds two version comparators.
5. **No advisory outside tp's own repository.** The generalisation to any project developing a tool
   with tp is real and is not taken here: it needs a way to know which binary a project is dogfooding,
   which tp does not have and this release does not invent.
6. **Not once per session — once per invocation.** tp holds no session identity
   (`grep -rniE 'sessionID|session_id|TP_SESSION' --include='*.go' internal/ cmd/ | grep -v _test.go`
   returns nothing), and inventing one to deduplicate an advisory is a larger mechanism than the
   advisory.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a binary whose `vcs.revision` equals `HEAD` emits **nothing**, while a spec numbered above the pseudo-version that binary reports is under development | the version predicate, which fires here — this is the false positive, and it must be watched failing first |
| 2 | §2 | two binaries whose reported versions reduce to the **same** comparable number but carry different `vcs.revision` values are distinguished: one silent, one advising | a predicate comparing the numeric prefix — what the repository's shipped comparator produces, reducing both of §2's binaries to `0.37.1`. The revision is present in both strings; the prefix comparison discards it |
| 3 | §3 *release* | `BuildInfo` with no `vcs.revision` **and** a real `Main.Version` advises, naming that version; the two builds reaching the same absence with `Main.Version` `(devel)` emit nothing. All three fixtures are in the test, not the first alone | dropping the `Main.Version` condition, which makes the advisory assert "not a build of this tree" about two builds of this tree — and, separately, treating a missing revision as a match, which silences the case §4 says the hook cannot catch |
| 4 | §3 *dirty* | `vcs.modified` true with a matching revision emits nothing | trigger on dirty — a build from a working tree records `vcs.modified=true` alongside a `vcs.revision` equal to `HEAD`, so the trigger fires on every invocation from a tree with uncommitted edits |
| 5 | §3 *order* | the release state is decided before the stale state, so a missing revision never reads as a mismatch against `HEAD` | compare first and check presence second, producing a stale advisory whose embedded-revision half is empty |
| 6 | §5.1 | the advisory changes no exit code — asserted across a command set, not one command | assert on `tp audit` alone, which leaves every sibling free to regress |
| 7 | §5.6 | two invocations in the same shell each emit the advisory | deduplicate via a file or environment marker, which is the session identity §5.6 declines to invent |

**Row 1 is the acceptance, and its fixture is §2's first transcript** — a binary built from `HEAD`
inside this repository while a spec numbered above the pseudo-version that build reports is under
development. That is the exact configuration the original predicate flags and the rule asks for. The
qualifier is load-bearing: because Go increments the patch, a spec numbered at or below the last
release is *below* what a `HEAD` build reports and would not trip the refuted predicate at all.

**Row 2's fixture is buildable and its cheaper form is not.** The two binaries do not report the same
version string — Go embeds the revision, so no two commits can. They report the same *comparable
number*, which is what a version predicate sees.
configuration the original predicate would have flagged, and the one the rule asks for.
