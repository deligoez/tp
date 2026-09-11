#!/usr/bin/env python3
"""Fail when a spec has grown past the size it had at review round 1.

Registered as the mechanical check for the `spec-growth` finding class, so
`tp review` runs it every round. The rule it enforces: a repair after round 1
deletes or narrows; it does not add requirements. Measured over this
repository's own history, 18 of 20 spec cycles grew their spec during review
and none shrank, and most review findings were answered by adding spec text —
so growth was the default, and this check makes it visible and failing.

The counting rule, stated so a disagreement is about the rule and not the code:

* A file's size is its line count: the number of newline characters, plus one
  when the file is non-empty and does not end in a newline. Blank lines count.
  Nothing is skipped — fences, tables and frontmatter are lines like any other.
* A spec's size is its body's size plus the size of its forensics sidecar
  `<base>-measurements.md` (same directory, same base name) when that file
  exists, 0 when it does not. `<base>` is the file name minus its last
  extension, the rule tp's own state directory uses (engine.ReviewStateDir).
* The baseline body is round 1's snapshot,
  `<spec dir>/.tp-review/<base>/snapshot-round-1.md` — the bytes tp wrote at
  round 1's emission.
* tp keeps no snapshot of the sidecar, so its baseline comes from git: the
  sidecar as committed in the commit that ADDED that snapshot (the oldest add,
  should the snapshot have been deleted and re-added), and 0 when the sidecar
  is absent from that commit.
* When the round-1 snapshot is in no commit — untracked, which is its state
  between round 1's emission and `--record` — or git is unavailable, git cannot
  say what the sidecar was, and the latest emitted round decides:
  - round 1 (no `snapshot-round-2.md` or higher): the baseline sidecar is the
    sidecar as it is now, because growth against round 1 at round 1 is zero by
    definition;
  - round 2 or later: the sidecar is left out of both sides, and stderr and the
    output line say `sidecar not checked: the round-1 snapshot is not
    committed`. The body is still checked and alone decides the exit.

Usage: check-spec-growth.py <spec.md>

Exit codes follow tp's `workflow.checks` contract (engine.CheckRan):
  0  the spec is no larger than at round 1, or it has no round-1 snapshot yet
     (nothing to compare; stderr says so)
  1  the spec grew; one line on stdout names the spec, baseline, now and delta
  2  usage error, an input that cannot be read, or git failing on a committed
     snapshot — the check could not run
  3  an unexpected error (a traceback on stderr) — the check could not run
A Python crash would otherwise exit 1, which tp reads as "found growth"; the
top-level handler is what keeps a crash in the could-not-run state.
"""

from __future__ import annotations

import os
import pathlib
import re
import subprocess
import sys
import traceback

EXIT_CLEAN = 0
EXIT_GREW = 1
EXIT_USAGE = 2
EXIT_CRASH = 3

USAGE = "usage: check-spec-growth.py <spec.md>"
NOT_CHECKED = "sidecar not checked: the round-1 snapshot is not committed"
SNAPSHOT_RE = re.compile(r"^snapshot-round-(\d+)\.md$")


class GitFailed(Exception):
    """git answered with an error where it should have had an answer."""


def line_count(data: bytes) -> int:
    """Count lines under the docstring's rule."""
    n = data.count(b"\n")
    if data and not data.endswith(b"\n"):
        n += 1
    return n


def base_name(spec: pathlib.Path) -> str:
    """The file name minus its last extension, as Go's filepath.Ext trims it."""
    name = spec.name
    dot = name.rfind(".")
    return name[:dot] if dot >= 0 else name


def latest_round(state_dir: pathlib.Path) -> int:
    """The highest N with a review snapshot-round-N.md in the state directory."""
    rounds = [int(m.group(1)) for e in state_dir.iterdir() if (m := SNAPSHOT_RE.match(e.name))]
    return max(rounds, default=0)


def git(args: list[str], cwd: str) -> subprocess.CompletedProcess[bytes]:
    return subprocess.run(["git", "-C", cwd, *args], capture_output=True, check=False)


def committed_sidecar(snapshot: pathlib.Path, sidecar: pathlib.Path) -> tuple[int | None, str | None]:
    """Return the sidecar's line count at the commit that added the snapshot.

    (None, note) means git cannot say — the snapshot is in no commit, or git is
    unavailable — and note, when set, is the reason for stderr. Raises
    GitFailed when the snapshot is committed but git fails to read it back.
    """
    try:
        top = git(["rev-parse", "--show-toplevel"], str(snapshot.parent))
    except OSError as exc:
        return None, f"git is not available ({exc}); the round-1 snapshot is treated as untracked"
    if top.returncode != 0:
        return None, "the snapshot is not inside a git repository; it is treated as untracked"
    toplevel = os.path.realpath(top.stdout.decode().strip())
    if git(["rev-parse", "--verify", "-q", "HEAD"], toplevel).returncode != 0:
        return None, None  # a repository with no commit yet: nothing is tracked
    snap_rel = os.path.relpath(os.path.realpath(snapshot), toplevel)

    log = git(["log", "--diff-filter=A", "--format=%H", "--", snap_rel], toplevel)
    if log.returncode != 0:
        raise GitFailed(f"git log failed: {log.stderr.decode().strip()}")
    shas = log.stdout.decode().split()
    if not shas:
        return None, None  # untracked
    sha = shas[-1]  # git log is newest first; the oldest add is the round-1 record

    side_rel = os.path.relpath(os.path.realpath(sidecar), toplevel).replace(os.sep, "/")
    listed = git(["ls-tree", "--name-only", sha, "--", side_rel], toplevel)
    if listed.returncode != 0:
        raise GitFailed(f"git ls-tree failed: {listed.stderr.decode().strip()}")
    if not listed.stdout.strip():
        return 0, None  # the sidecar did not exist at round 1: that is a real 0
    blob = git(["cat-file", "blob", f"{sha}:{side_rel}"], toplevel)
    if blob.returncode != 0:
        raise GitFailed(f"git cat-file failed: {blob.stderr.decode().strip()}")
    return line_count(blob.stdout), None


def display(path: pathlib.Path) -> str:
    """The path relative to the working directory when it lies under it."""
    try:
        return str(path.resolve().relative_to(pathlib.Path.cwd().resolve()))
    except ValueError:
        return str(path)


def spec_arg(argv: list[str]) -> pathlib.Path | None:
    """The one spec argument, or None after printing why it is unusable."""
    args = argv[1:]
    if len(args) != 1 or args[0].startswith("-"):
        print(USAGE, file=sys.stderr)
        return None
    if args[0] == "":
        print(
            "check-spec-growth: the spec path is empty — the command that names the "
            "spec (e.g. `tp resume`) printed nothing",
            file=sys.stderr,
        )
        return None
    spec = pathlib.Path(args[0])
    if not spec.is_file():
        print(f"check-spec-growth: no such spec: {args[0]}", file=sys.stderr)
        return None
    return spec


def run(argv: list[str]) -> int:
    spec = spec_arg(argv)
    if spec is None:
        return EXIT_USAGE

    base = base_name(spec)
    snapshot = spec.parent / ".tp-review" / base / "snapshot-round-1.md"
    sidecar = spec.parent / f"{base}-measurements.md"
    if not snapshot.exists():
        print(
            f"check-spec-growth: no round-1 snapshot at {display(snapshot)}; nothing to compare",
            file=sys.stderr,
        )
        return EXIT_CLEAN

    try:
        body_then = line_count(snapshot.read_bytes())
        body_now = line_count(spec.read_bytes())
        side_now = line_count(sidecar.read_bytes()) if sidecar.exists() else 0
        latest = latest_round(snapshot.parent)
        side_then, note = committed_sidecar(snapshot, sidecar)
    except (OSError, GitFailed) as exc:
        print(f"check-spec-growth: cannot measure: {exc}", file=sys.stderr)
        return EXIT_USAGE
    if note:
        print(f"check-spec-growth: {note}", file=sys.stderr)

    if side_then is None and latest <= 1:
        side_then = side_now  # round 1 against round 1: zero growth by definition
    if side_then is None:
        print(f"check-spec-growth: {NOT_CHECKED}", file=sys.stderr)
        then, now, sidecar_text = body_then, body_now, NOT_CHECKED
    else:
        then, now = body_then + side_then, body_now + side_now
        sidecar_text = f"sidecar {side_then}->{side_now}"

    grew = now > then
    verdict = "grew past round 1" if grew else "within round 1"
    tail = " — a repair after round 1 deletes or narrows" if grew else ""
    print(
        f"spec-growth: {display(spec)} {verdict}: baseline {then}, now {now}, "
        f"delta {now - then:+d} (body {body_then}->{body_now}, {sidecar_text}){tail}"
    )
    return EXIT_GREW if grew else EXIT_CLEAN


def main() -> int:
    try:
        return run(sys.argv)
    except SystemExit:
        raise
    except BaseException:  # noqa: BLE001 — a crash must not exit 1 ("found growth")
        traceback.print_exc()
        return EXIT_CRASH


if __name__ == "__main__":
    sys.exit(main())
