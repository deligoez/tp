# tp v1.44.0 — The binary check

> **This file is decisions.** The predicate this release was originally specified with — *compare the
> running binary's version against the spec being developed* — was prototyped while writing this file
> and **refuted in both directions**. §2 records that, because the refuted form is the one an
> implementer would otherwise write.

## 1. Overview

**The rule being mechanized:** *dogfood the in-progress binary, never the lagging release.*
`CLAUDE.md` has carried it since v0.28.0, it is violated by anyone who forgets to rebuild, and the
violation is silent — a stale binary produces confident output about behaviour it does not have.

tp emits **one advisory per invocation** when the binary running inside a tp repository is not the one
the working tree would build. Never an error, never an exit code, silenced by `--quiet`.

## 2. The version comparison is the wrong predicate

The original specification compared the running binary's version against *the newest spec version
under development*. Measured on this repository, that predicate is wrong in both directions.

**False positive — it fires on the correct setup.** A binary freshly built from `HEAD` reports the
*last tag's* version, because Go derives `Main.Version` from the most recent tag plus a pseudo-version
suffix. Developing `spec/1.38.0.md` with a binary built one minute ago from `HEAD` gives:

```
$ tp --version
tp version v0.37.1-0.20260902095645-65d0b09f2800+dirty
```

`v0.37.1 < 1.38.0`, so the advisory fires — on exactly the binary the rule asks for.

**False negative — it cannot see the case it exists for.** The same two binaries, seven commits apart,
report indistinguishable versions. Measured live while writing this file:

| | reports | embedded commit |
|---|---|---|
| the binary in use, built earlier this session | `v0.37.1-0.2026…-65d0b09f2800+dirty` | `65d0b09f2800` |
| `git rev-parse --short=12 HEAD` | — | **`10dee489714a`** |

The binary being dogfooded was **seven commits stale and its version string said nothing about it.**
That is the failure the rule is about, and the version predicate is blind to it.

**The discriminator is the commit, and it is already embedded.** `internal/cli/root.go:56` already
calls `debug.ReadBuildInfo()` for `info.Main.Version`; the same `BuildInfo` carries `vcs.revision` and
`vcs.modified` in `Settings`, put there by `buildvcs`, which is on by default.

## 3. The predicate

Inside a git repository that contains tp's own module, three states, decided in order:

| state | condition | advisory |
|---|---|---|
| **release** | no `vcs.revision` in `BuildInfo` | names the installed version and the rebuild command — this is a `go install`ed binary, not a build of this tree |
| **stale** | `vcs.revision` present and ≠ `HEAD` | names both short revisions and the rebuild command |
| **current** | `vcs.revision` == `HEAD` | **nothing** |

**`vcs.modified` is not a trigger.** A dirty working tree is the normal state of development; the
binary is still the one this tree builds, up to uncommitted edits. Advising on it would fire on
essentially every invocation and train the reader to ignore the channel.

**Commit distance is not computed.** `HEAD` and the embedded revision either match or they do not.
Counting commits between them means a `git` call on every invocation to produce a number that changes
no advice.

**A repository that is not tp sees nothing.** The check is gated on the module path from `BuildInfo`
matching the repository's own module, which is a fact tp already holds and needs no heuristic.

## 4. Not the plugin's version check, and the two must not be merged

`hooks/session-start.sh` compares `tp --version` against `.claude-plugin/plugin.json`'s version, runs
in `SessionStart`, and **fails**. This one compares the binary's *commit* against the tree's `HEAD`,
runs on any invocation inside the repository, and **advises**.

**They disagree on the case that matters, and that is why both exist.** A freshly installed release
developing the next version's spec passes the hook — its version is at or above the manifest's — and
trips this advisory, because it carries no `vcs.revision` at all.

**The shell comparison is not reused, and this release does not port it.**
`hooks/session-start.sh`'s `version_number`/`version_part` are the only version-comparison logic in
the repository and they live in bash. This release needs no version comparison — §3 compares two
revision strings for equality — so it introduces no second implementation to drift from the first.
That is the main reason §2's refutation is a simplification rather than a cost.

## 5. Non-Goals

1. **Never an error.** No exit code changes, in any command, under any condition.
2. **No rebuild.** tp names the command; running it is the operator's.
3. **No merge with the plugin's version check.** §4 states why; merging makes one of the two wrong for
   its own case.
4. **No version comparison anywhere in this release.** §2 refuted the predicate and §4 records that
   dropping it avoids a second implementation of the shell's logic.
5. **No advisory outside tp's own repository.** The generalisation to any project developing a tool
   with tp is real and is not taken here: it needs a way to know which binary a project is dogfooding,
   which tp does not have and this release does not invent.
6. **Not once per session — once per invocation.** tp holds no session identity, and inventing one to
   deduplicate an advisory is a larger mechanism than the advisory.

## 6. Tests

Every row derives from a numbered decision, names the artifact it depends on, and names a mutant that
must fail it.

| # | from | assertion | the mutant that must fail it |
|---|---|---|---|
| 1 | §2 | a binary whose `vcs.revision` equals `HEAD` emits **nothing**, even when every task file's spec version is far above the binary's reported version | the version predicate, which fires here — this is the false positive, and it must be watched failing first |
| 2 | §2 | two binaries reporting the **same** version but different revisions are distinguished: one silent, one advising | the version predicate, which cannot separate them by construction |
| 3 | §3 *release* | `BuildInfo` with no `vcs.revision` advises, naming the installed version | treat a missing revision as a match, which silences the case §4 says the hook cannot catch |
| 4 | §3 *dirty* | `vcs.modified` true with a matching revision emits nothing | trigger on dirty, which fires on nearly every development invocation |
| 5 | §3 *order* | the release state is decided before the stale state, so a missing revision never reads as a mismatch against `HEAD` | compare first and check presence second, producing "stale: (empty) vs 10dee489" |
| 6 | §5.1 | the advisory changes no exit code — asserted across a command set, not one command | assert on `tp audit` alone, which leaves every sibling free to regress |
| 7 | §5.6 | two invocations in the same shell each emit the advisory | deduplicate via a file or environment marker, which is the session identity §5.6 declines to invent |

**Row 1 is the acceptance and is the transcript in §2.** The fixture is a binary built from `HEAD`
inside this repository while a spec numbered well above its version is being developed — the exact
configuration the original predicate would have flagged, and the one the rule asks for.
