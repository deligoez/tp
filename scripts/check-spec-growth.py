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
  round 1's emission. tp keeps no snapshot of the sidecar, so the baseline
  sidecar is the sidecar as committed in the commit that ADDED that snapshot
  (the oldest add, should the snapshot have been deleted and re-added), and 0
  when the sidecar is absent from that commit. When git is unavailable or the
  snapshot is in no commit (untracked), the baseline sidecar is 0 and a line on
  stderr says so; that direction over-reports growth rather than hiding it.

Usage: check-spec-growth.py <spec.md>

Exit codes follow tp's `workflow.checks` contract (engine.CheckRan):
  0  the spec is no larger than at round 1, or it has no round-1 snapshot yet
     (nothing to compare; stderr says so)
  1  the spec grew; one line on stdout names the spec, baseline, now and delta
  2  usage error, or an input that cannot be read — the check could not run
  3  an unexpected error (a traceback on stderr) — the check could not run
A Python crash would otherwise exit 1, which tp reads as "found growth"; the
top-level handler is what keeps a crash in the could-not-run state.
"""

from __future__ import annotations

import os
import pathlib
import subprocess
import sys
import traceback

EXIT_CLEAN = 0
EXIT_GREW = 1
EXIT_USAGE = 2
EXIT_CRASH = 3

USAGE = "usage: check-spec-growth.py <spec.md>"


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


def git(args: list[str], cwd: str) -> subprocess.CompletedProcess[bytes]:
    return subprocess.run(["git", "-C", cwd, *args], capture_output=True, check=False)


def sidecar_baseline(snapshot: pathlib.Path, sidecar: pathlib.Path) -> tuple[int, str | None]:
    """Return the sidecar's line count at the commit that added the snapshot.

    The second value is a note for stderr when the baseline fell back to 0
    because git could not answer, and None when git answered (including the
    answer "the sidecar was not in that commit").
    """
    try:
        top = git(["rev-parse", "--show-toplevel"], str(snapshot.parent))
    except OSError as exc:
        return 0, f"git is not available ({exc})"
    if top.returncode != 0:
        return 0, "the snapshot is not inside a git repository"
    toplevel = os.path.realpath(top.stdout.decode().strip())
    snap_rel = os.path.relpath(os.path.realpath(snapshot), toplevel)

    log = git(["log", "--diff-filter=A", "--format=%H", "--", snap_rel], toplevel)
    if log.returncode != 0:
        return 0, f"git log failed: {log.stderr.decode().strip()}"
    shas = log.stdout.decode().split()
    if not shas:
        return 0, f"{snap_rel} is in no commit (untracked)"
    sha = shas[-1]  # git log is newest first; the oldest add is the round-1 record

    side_rel = os.path.relpath(os.path.realpath(sidecar), toplevel).replace(os.sep, "/")
    listed = git(["ls-tree", "--name-only", sha, "--", side_rel], toplevel)
    if listed.returncode != 0:
        return 0, f"git ls-tree failed: {listed.stderr.decode().strip()}"
    if not listed.stdout.strip():
        return 0, None  # the sidecar did not exist at round 1: that is a real 0
    blob = git(["cat-file", "blob", f"{sha}:{side_rel}"], toplevel)
    if blob.returncode != 0:
        return 0, f"git cat-file failed: {blob.stderr.decode().strip()}"
    return line_count(blob.stdout), None


def display(path: pathlib.Path) -> str:
    """The path relative to the working directory when it lies under it."""
    try:
        return str(path.resolve().relative_to(pathlib.Path.cwd().resolve()))
    except ValueError:
        return str(path)


def run(argv: list[str]) -> int:
    args = argv[1:]
    if len(args) != 1 or args[0].startswith("-"):
        print(USAGE, file=sys.stderr)
        return EXIT_USAGE
    if args[0] == "":
        print(
            "check-spec-growth: the spec path is empty — the command that names the "
            "spec (e.g. `tp resume`) printed nothing",
            file=sys.stderr,
        )
        return EXIT_USAGE
    spec = pathlib.Path(args[0])
    if not spec.is_file():
        print(f"check-spec-growth: no such spec: {args[0]}", file=sys.stderr)
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
    except OSError as exc:
        print(f"check-spec-growth: cannot read an input: {exc}", file=sys.stderr)
        return EXIT_USAGE

    side_then, note = sidecar_baseline(snapshot, sidecar)
    if note:
        print(f"check-spec-growth: sidecar baseline taken as 0 — {note}", file=sys.stderr)

    then, now = body_then + side_then, body_now + side_now
    grew = now > then
    verdict = "grew past round 1" if grew else "within round 1"
    tail = " — a repair after round 1 deletes or narrows" if grew else ""
    print(
        f"spec-growth: {display(spec)} {verdict}: baseline {then}, now {now}, "
        f"delta {now - then:+d} (body {body_then}->{body_now}, "
        f"sidecar {side_then}->{side_now}){tail}"
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
