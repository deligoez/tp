#!/usr/bin/env python3
"""Prepare one audit round: one prebuilt tree, and a delta re-grade per role.

Mechanizes rules 1, 3 and 4 of CLAUDE.md's "Run the round cheaply" (and
`skills/tp/SKILL.md` Workflow D's matching subsection), which were measured on
v1.1.0's audit and until now were prose an orchestrator had to remember:

* Rule 1 — one clone, one binary, built once before the role stage. Four roles
  each cloning and building the same tree against each other's CPU is the
  measured waste; this script builds it once and every brief fragment names the
  path with "do not clone, do not build".
* Rule 3 — delta re-grade. A row is carried only when it was `PASS` last round
  AND names an `evidence_file` AND that file is untouched since the commit that
  recorded the round. The orchestrator derives that list; the role does not
  guess it.
* Rule 4 — the class-to-slug table reaches the role before it writes a row, so a
  finding in an already-routed class is recorded `PARTIAL` with the slug in the
  note and expects no repair.

Rule 2 (one repair unit per independent item) is a scheduling decision over
findings that do not exist yet, so it is not mechanized here.

What "untouched" means, stated so a disagreement is about the rule and not the
code: a path is CHANGED when it appears in
`git diff --name-only <previous record sha>..HEAD` or when it has any
uncommitted change at all (`git status --porcelain`, renames contributing both
their old and their new name). Uncommitted changes count because the roles read
the working tree, not `HEAD` — but the clone this script builds is of `HEAD`,
so the summary reports `working_tree_dirty` and leaves the discrepancy to the
operator rather than papering over it by copying the dirt into the clone.

The previous record sha is the commit that first ADDED the round file
(`git log --follow --diff-filter=A`, the oldest if there are several), not the
last commit to touch it. A round file is written again after its record — `tp
audit --resolve` puts dispositions into it — and committing that must not move
the base of the delta past the repairs the disposition answers, or a repaired
evidence file reads as untouched and its row is carried unverified. A move of
the round directory must not move it either, hence `--follow`.

A round is refused when any of its `file_check` rows (an `item_id` starting
`file-`) carries an id of the earlier derivation: `file-<role>-<slug>`,
optionally with a positional `-2`/`-3` suffix, rather than one ending in `.`
and a 16-hex digest of the path. Such an id names no item of the next round, so
there is nothing to carry and every role measures from scratch.

Usage:
  audit-round-prep.py <spec.md> [--previous-record <sha>] [--out <dir>]
                      [--no-build] [--routed <json>]

`--out` defaults to `<tmpdir>/tp-audit-prep-<base>-round-<N>`; any
`carried-*.ndjson` / `remeasure-*.json` already there is removed first, so a
role that disappeared between rounds cannot leave a stale file behind.
`--routed` defaults to `<repo root>/.tp/routed-classes.json` when that exists.

Stdout on success is ONE JSON object: `previous_round`, `previous_record_sha`,
`changed_files`, `working_tree_dirty`, `tree`, `binary`, `roles`
(`{carried, remeasure, carried_file, remeasure_file}` each), `routed_classes`
and `brief` — the last a ready-to-paste paragraph per role.

Exit 0 when the prep succeeded; 2 when there is nothing to carry (no round
directory, no recorded audit round, a round file git does not track, a round
holding an earlier-derivation `file_check` id, or bad arguments) — a first
round carries nothing and must be briefed by hand; 3 when
the clone or the build failed, with the failing step named. Errors are a JSON
object on stderr so stdout stays parseable either way.
"""

from __future__ import annotations

import argparse
import json
import os
import pathlib
import re
import subprocess
import sys
import tempfile

ROUND_RE = re.compile(r"^audit-round-(\d+)\.ndjson$")
# A current file_check id ends in `.` and a 16-hex digest of the path; the `.`
# never occurs in an earlier-derivation slug, so the suffix alone tells them apart.
CURRENT_FILE_CHECK_RE = re.compile(r"\.[0-9a-f]{16}$")


def fail(code: int, **payload: object) -> None:
    """Print a JSON error on stderr and exit, leaving stdout empty."""
    json.dump(payload, sys.stderr, sort_keys=True)
    sys.stderr.write("\n")
    raise SystemExit(code)


def git(root: pathlib.Path, *args: str) -> str:
    """Run git with the external differ disabled and return its stdout.

    `diff.external` is disabled on every call because this machine's global
    config sets `difft`, under which `git diff` from a non-TTY child returns
    near-empty output with exit 0 — a delta re-grade built on that would carry
    every row.
    """
    proc = subprocess.run(
        ["git", "-c", "diff.external=", *args],
        cwd=str(root),
        capture_output=True,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        fail(2, error="git-failed", command=list(args), stderr=proc.stderr.strip())
    return proc.stdout


def status_paths(root: pathlib.Path) -> list[str]:
    """Every path with an uncommitted change, both names of a rename."""
    raw = git(root, "status", "--porcelain", "-z")
    parts = raw.split("\0")
    paths: list[str] = []
    i = 0
    while i < len(parts):
        entry = parts[i]
        i += 1
        if len(entry) < 4:
            continue
        xy, path = entry[:2], entry[3:]
        paths.append(path)
        if "R" in xy or "C" in xy:
            # A rename/copy entry is followed by its source path.
            if i < len(parts) and parts[i]:
                paths.append(parts[i])
                i += 1
    return paths


def previous_round(review_dir: pathlib.Path) -> tuple[int, pathlib.Path]:
    """The highest-numbered audit round file, or exit 2 naming where we looked."""
    if not review_dir.is_dir():
        fail(
            2,
            error="no-round-directory",
            looked_in=str(review_dir),
            detail="a first audit round has nothing to carry; brief it by hand",
        )
    best: tuple[int, pathlib.Path] | None = None
    for path in sorted(review_dir.iterdir()):
        match = ROUND_RE.match(path.name)
        if match and (best is None or int(match.group(1)) > best[0]):
            best = (int(match.group(1)), path)
    if best is None:
        fail(
            2,
            error="no-audit-round",
            looked_in=str(review_dir),
            detail="no audit-round-<N>.ndjson; a first audit round has nothing to carry",
        )
    return best  # type: ignore[return-value]


def rows_of(path: pathlib.Path) -> list[tuple[bytes, dict]]:
    """Each non-blank line as (its exact bytes, its parsed object)."""
    out: list[tuple[bytes, dict]] = []
    for lineno, line in enumerate(path.read_bytes().split(b"\n"), start=1):
        if not line.strip():
            continue
        try:
            row = json.loads(line.decode("utf-8"))
        except (UnicodeDecodeError, ValueError) as exc:
            fail(2, error="unparseable-row", file=str(path), line=lineno, detail=str(exc))
        if not isinstance(row, dict):
            fail(2, error="unparseable-row", file=str(path), line=lineno, detail="not an object")
        out.append((line, row))
    return out


def earlier_derivation_ids(rows: list[tuple[bytes, dict]]) -> list[str]:
    """The file_check item ids of the earlier derivation, in row order."""
    earlier: list[str] = []
    for _, row in rows:
        item = row.get("item_id")
        if (isinstance(item, str) and item.startswith("file-")
                and not CURRENT_FILE_CHECK_RE.search(item)):
            earlier.append(item)
    return earlier


def record_sha(root: pathlib.Path, round_rel: str) -> str:
    """The commit that first added the round file, or exit 2 when none did.

    `git log` lists newest first, so the oldest add is the last line; a later
    commit that only rewrites the file (a disposition) is not an add and cannot
    move it. `--follow` carries the search across a rename, so a round
    directory moved after its record (as under spec/backlog/) reports its
    original record rather than the move, which is a rename and not an add.
    """
    adds = git(root, "log", "--follow", "--diff-filter=A", "--format=%H", "--",
               round_rel).split()
    if not adds:
        fail(
            2,
            error="unrecorded-round",
            file=round_rel,
            detail="git tracks no commit adding this round file; an unrecorded "
                   "round carries nothing",
        )
    return adds[-1]


def split_rows(
    rows: list[tuple[bytes, dict]], changed: set[str]
) -> dict[str, dict[str, list]]:
    """Partition rows per role into carried (verbatim) and re-measure."""
    roles: dict[str, dict[str, list]] = {}
    for line, row in rows:
        role = row.get("role") or "unknown"
        bucket = roles.setdefault(role, {"carried": [], "remeasure": []})
        evidence = row.get("evidence_file")
        carried = (
            row.get("status") == "PASS"
            and isinstance(evidence, str)
            and evidence != ""
            and evidence not in changed
        )
        if carried:
            bucket["carried"].append(line)
        else:
            item = row.get("item_id")
            bucket["remeasure"].append(item if isinstance(item, str) and item else row)
    return roles


def build_tree(root: pathlib.Path, out: pathlib.Path) -> tuple[str, str]:
    """Clone HEAD once and build one binary in it; exit 3 naming the step."""
    tree = out / "tree"
    if tree.exists():
        fail(3, error="tree-exists", step="clone", tree=str(tree),
             detail="remove it or pass a fresh --out; a reused clone is not HEAD")
    clone = subprocess.run(
        ["git", "-c", "diff.external=", "clone", "--no-hardlinks", "-q",
         str(root), str(tree)],
        capture_output=True, text=True, check=False,
    )
    if clone.returncode != 0:
        fail(3, error="clone-failed", step="clone", stderr=clone.stderr.strip())

    binary = tree / "tp"
    build = subprocess.run(
        ["go", "build", "-o", str(binary), "./cmd/tp"],
        cwd=str(tree), capture_output=True, text=True, check=False,
    )
    if build.returncode != 0:
        fail(3, error="build-failed", step="build", stderr=build.stderr.strip())
    return str(tree), str(binary)


def brief_for(
    role: str,
    counts: dict,
    tree: str | None,
    binary: str | None,
    sha: str,
    round_n: int,
    routed: dict,
    dirty: bool,
) -> str:
    parts: list[str] = []
    if tree and binary:
        parts.append(
            f"A tree is already cloned at {tree} and a tp binary is already built at "
            f"{binary}; use them and do not clone, do not build. A mutant needs its own "
            f"copy: rsync that tree, never mutate it in place."
        )
    else:
        parts.append(
            "No tree was prebuilt for this round (--no-build), so clone and build your own "
            "if a probe needs one."
        )

    if counts["carried"]:
        parts.append(
            f"{counts['carried']} row(s) are carried: {counts['carried_file']}. Re-record "
            f"them verbatim into your results file with evidence_file and evidence_lines "
            f"unchanged, and do not re-verify them — each was PASS in audit round "
            f"{round_n} and its evidence file is untouched since {sha}."
        )
    else:
        parts.append(
            f"No row of audit round {round_n} is carried for this role; measure every item "
            f"from scratch."
        )

    if counts["remeasure"]:
        ids = ", ".join(
            item if isinstance(item, str) else json.dumps(item, sort_keys=True)
            for item in counts["remeasure_items"]
        )
        parts.append(f"Re-measure exactly these {counts['remeasure']} item(s): {ids}.")
    else:
        parts.append("Nothing is left to re-measure for this role beyond the carried rows.")

    if routed:
        table = "; ".join(f"{cls} -> {slug}" for cls, slug in sorted(routed.items()))
        parts.append(
            "A finding whose class appears in this routed table is recorded PARTIAL with "
            f"the slug in the note and expects no repair: {table}."
        )
    else:
        parts.append("No class is routed yet, so every finding reaches the orchestrator.")

    if dirty:
        parts.append(
            "The working tree is dirty, so it differs from the clone; measure the working "
            "tree and say so when a finding depends on an uncommitted change."
        )
    return " ".join(parts)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        prog="audit-round-prep.py",
        description="Prepare one audit round: one prebuilt tree and a per-role delta re-grade.",
    )
    parser.add_argument("spec", help="the spec the audit is grading, e.g. spec/1.1.0.md")
    parser.add_argument("--previous-record", metavar="SHA",
                        help="commit the delta is measured from; defaults to the commit "
                             "that first added the previous round file")
    parser.add_argument("--out", metavar="DIR", help="where the carried/re-measure files and "
                                                     "the clone go")
    parser.add_argument("--no-build", action="store_true",
                        help="skip the clone and the build; tree and binary come back null")
    parser.add_argument("--routed", metavar="JSON",
                        help="class-to-slug table; defaults to .tp/routed-classes.json")
    args = parser.parse_args(argv[1:])

    spec = pathlib.Path(args.spec)
    if not spec.is_file():
        fail(2, error="no-such-spec", spec=str(spec))

    root = pathlib.Path(git(spec.resolve().parent, "rev-parse", "--show-toplevel").strip())
    round_n, round_file = previous_round(spec.parent / ".tp-review" / spec.stem)
    round_rel = os.path.relpath(round_file.resolve(), root)

    rows = rows_of(round_file)
    earlier = earlier_derivation_ids(rows)
    if earlier:
        fail(
            2,
            error="earlier-derivation-file-check-ids",
            file=round_rel,
            count=len(earlier),
            example_item_id=earlier[0],
            detail="nothing to carry: the previous round's file_check ids were derived "
                   "under an earlier scheme and name no item of the next round, so every "
                   "role measures from scratch without this script's carry",
        )

    sha = args.previous_record or record_sha(root, round_rel)

    dirty_paths = status_paths(root)
    changed = set(git(root, "diff", "--name-only", f"{sha}..HEAD").split("\n"))
    changed.discard("")
    changed.update(dirty_paths)

    roles = split_rows(rows, changed)

    out = pathlib.Path(
        args.out
        or os.path.join(tempfile.gettempdir(), f"tp-audit-prep-{spec.stem}-round-{round_n}")
    )
    out.mkdir(parents=True, exist_ok=True)
    for stale in [*out.glob("carried-*.ndjson"), *out.glob("remeasure-*.json")]:
        stale.unlink()

    tree = binary = None
    if not args.no_build:
        tree, binary = build_tree(root, out)

    routed_path = pathlib.Path(args.routed) if args.routed else root / ".tp" / "routed-classes.json"
    routed: dict = {}
    if args.routed and not routed_path.is_file():
        fail(2, error="no-such-routed-table", routed=str(routed_path))
    if routed_path.is_file():
        try:
            routed = json.loads(routed_path.read_text(encoding="utf-8"))
        except ValueError as exc:
            fail(2, error="unparseable-routed-table", routed=str(routed_path), detail=str(exc))
        if not isinstance(routed, dict):
            fail(2, error="unparseable-routed-table", routed=str(routed_path),
                 detail="not an object of class -> slug")

    summary_roles: dict[str, dict] = {}
    briefs: dict[str, str] = {}
    for role in sorted(roles):
        carried_file = out / f"carried-{role}.ndjson"
        remeasure_file = out / f"remeasure-{role}.json"
        lines = roles[role]["carried"]
        items = roles[role]["remeasure"]
        carried_file.write_bytes(b"".join(line + b"\n" for line in lines))
        remeasure_file.write_text(json.dumps(items, indent=2, sort_keys=True) + "\n",
                                  encoding="utf-8")
        counts = {
            "carried": len(lines),
            "remeasure": len(items),
            "carried_file": str(carried_file),
            "remeasure_file": str(remeasure_file),
        }
        summary_roles[role] = counts
        briefs[role] = brief_for(
            role, {**counts, "remeasure_items": items}, tree, binary, sha, round_n,
            routed, bool(dirty_paths),
        )

    json.dump(
        {
            "previous_round": round_n,
            "previous_record_sha": sha,
            "changed_files": sorted(changed),
            "working_tree_dirty": bool(dirty_paths),
            "tree": tree,
            "binary": binary,
            "roles": summary_roles,
            "routed_classes": routed,
            "brief": briefs,
        },
        sys.stdout,
        indent=2,
        sort_keys=True,
    )
    sys.stdout.write("\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
