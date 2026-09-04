#!/usr/bin/env bash
# Remove round artifacts under spec/.tp-review/ that belong to no recorded round.
#
# An emission writes two files per round — the snapshot (the text the round read)
# and the floor (what --record validates the payload against). `--record` writes
# the third, `<phase>-round-N.ndjson`, and commits all three together. A round
# that was emitted and never recorded therefore leaves two files behind that
# nothing references, and re-emitting an unrecorded round is idempotent, so
# removing them costs one command to restore.
#
# THREE GUARDS, and they fail independently on purpose:
#
#   1. the round is not recorded — no `<phase>-round-N.ndjson` beside the artifact
#   2. the file is not tracked by git — a tracked artifact belongs to a committed
#      round whatever guard 1 thinks, and the two disagreeing is a bug worth
#      surviving
#   3. the file is older than --min-age-hours (default 1) — an emission from
#      minutes ago is most likely being graded RIGHT NOW, and deleting the floor
#      out from under a grading unit makes its payload unrecordable. Age is the
#      only signal available for that; there is no lock to read.
#
# Dry run by default. Nothing is deleted without --apply.
#
# Usage:
#   scripts/clean-emissions.sh                     # report what it would remove
#   scripts/clean-emissions.sh --apply             # remove it
#   scripts/clean-emissions.sh --min-age-hours 24  # be more conservative

set -euo pipefail

APPLY=0
MIN_AGE_HOURS=1
ROOT="spec/.tp-review"

while [ $# -gt 0 ]; do
  case "$1" in
    --apply)          APPLY=1; shift ;;
    --min-age-hours)  MIN_AGE_HOURS="${2:?--min-age-hours needs a number}"; shift 2 ;;
    --root)           ROOT="${2:?--root needs a path}"; shift 2 ;;
    -h|--help)        sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)                echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

[ -d "$ROOT" ] || { echo "no $ROOT — nothing to do"; exit 0; }

# Portable mtime-in-epoch-seconds: GNU stat and BSD stat disagree on flags, and
# this repository runs on both.
mtime() {
  python3 -c 'import os,sys;print(int(os.stat(sys.argv[1]).st_mtime))' "$1"
}

now=$(date +%s)
cutoff=$(( MIN_AGE_HOURS * 3600 ))

removable=()
kept_recorded=0
kept_tracked=0
kept_young=0

while IFS= read -r f; do
  base=$(basename "$f")
  dir=$(dirname "$f")

  round=$(printf '%s' "$base" | sed -n 's/.*round-\([0-9][0-9]*\)\..*/\1/p')
  [ -n "$round" ] || continue

  case "$base" in
    *ground*) phase=ground ;;
    *audit*)  phase=audit  ;;
    *)        phase=review ;;
  esac

  # Guard 1 — the round is recorded, so these two files are its evidence.
  if [ -f "$dir/$phase-round-$round.ndjson" ]; then
    kept_recorded=$(( kept_recorded + 1 )); continue
  fi

  # Guard 2 — git tracks it, so it was committed with some round.
  if git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    kept_tracked=$(( kept_tracked + 1 )); continue
  fi

  # Guard 3 — too young to be sure nobody is grading against it.
  if [ $(( now - $(mtime "$f") )) -lt "$cutoff" ]; then
    kept_young=$(( kept_young + 1 )); continue
  fi

  removable+=("$f")
done < <(find "$ROOT" -type f \( -name 'floor-*round-*.txt' -o -name 'snapshot-*round-*.md' \) | sort)

echo "kept: $kept_recorded recorded · $kept_tracked tracked · $kept_young younger than ${MIN_AGE_HOURS}h"

if [ ${#removable[@]} -eq 0 ]; then
  echo "nothing to remove"
  exit 0
fi

printf '%s\n' "${removable[@]}"
echo "${#removable[@]} file(s)"

if [ "$APPLY" -eq 1 ]; then
  rm -f "${removable[@]}"
  echo "removed."
else
  echo "dry run — pass --apply to remove, or --min-age-hours N to be stricter"
fi
