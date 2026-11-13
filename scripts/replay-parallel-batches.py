#!/usr/bin/env python3
"""Replay tp's own task files under the implement-batch rule (spec/1.19.0-measurements.md).

Each closed task's file set is what its recorded commit_shas changed
(git show --name-only), minus tp-owned paths: a perfect `touches`. A step takes
the ready tasks in tp next's order (most dependents, smallest estimate, id) and
adds each later one whose file set is disjoint from the batch, up to the cap; a
task with no recorded files runs alone. The ratio is tasks per step, against
one per step today. It counts steps, not wall-clock time: the gates of a batch
still run one at a time.

Usage: scripts/replay-parallel-batches.py [repo-root]
"""
import glob
import json
import subprocess
import sys

ROOT = sys.argv[1] if len(sys.argv) > 1 else "."
CAPS = (2, 3, 0)  # 0 = no cap


def files_of(shas):
    out = set()
    for sha in shas:
        r = subprocess.run(["git", "-C", ROOT, "show", "--name-only", "--format=", sha],
                           capture_output=True, text=True, check=False)
        if r.returncode != 0:
            continue
        for f in r.stdout.splitlines():
            f = f.strip()
            if not f or f.endswith(".tasks.json") or "/.tp-review/" in f or f.startswith(".tp/"):
                continue
            out.add(f)
    return out


def steps_for(tasks, cap):
    ids = {t["id"] for t in tasks}
    dependents = {}
    for t in tasks:
        for d in t.get("depends_on") or []:
            dependents[d] = dependents.get(d, 0) + 1
    done, steps = set(), 0
    while len(done) < len(tasks):
        ready = [t for t in tasks if t["id"] not in done
                 and all(d in done or d not in ids for d in (t.get("depends_on") or []))]
        ready.sort(key=lambda t: (-dependents.get(t["id"], 0), t.get("estimate_minutes", 0), t["id"]))
        batch = [ready[0]]
        if cap != 1 and ready[0]["files"]:
            for t in ready[1:]:
                if cap and len(batch) >= cap:
                    break
                if t["files"] and all(not (t["files"] & b["files"]) for b in batch):
                    batch.append(t)
        done.update(b["id"] for b in batch)
        steps += 1
    return steps


def main():
    files, total = 0, 0
    steps = dict.fromkeys(CAPS, 0)
    for path in sorted(glob.glob(ROOT + "/spec/**/*.tasks.json", recursive=True)):
        with open(path, encoding="utf-8") as fh:
            tasks = [t for t in json.load(fh).get("tasks", []) if t.get("commit_shas")]
        if len(tasks) < 2:
            continue
        for t in tasks:
            t["files"] = files_of(t["commit_shas"])
        files += 1
        total += len(tasks)
        for cap in CAPS:
            steps[cap] += steps_for(tasks, cap)
    print(f"task files: {files}, tasks: {total}")
    for cap in CAPS:
        print(f"cap {cap or 'none'}: {steps[cap]} steps, x{total / steps[cap]:.2f}")


if __name__ == "__main__":
    main()
