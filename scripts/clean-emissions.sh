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
# Inbox files — ground's scratch file `ground-inbox/round-N.ndjson`, the
# review and audit role files `{review,audit}-inbox/round-N-<role>.ndjson`, the
# pools an audit emission keeps beside them, `audit-inbox/round-N.pools.json`,
# and the roles a review emission owes, `review-inbox/round-N.roles.json`, with
# the rows `--record-inbox` merged from them, `review-inbox/round-N.merged.ndjson`
# — are judged against the round files one directory up, and differ in guard 1:
# `--record` removes them, so one whose round IS recorded is debris, not the
# round's evidence, and is removable too.
#
# THREE GUARDS, and they fail independently on purpose:
#
#   1. the round is not recorded — no `<phase>-round-N.ndjson` beside the artifact
#      (not applied to inbox files, see above)
#   2. the file is not tracked by git — a tracked artifact belongs to a committed
#      round whatever guard 1 thinks, and the two disagreeing is a bug worth
#      surviving
#   3. the file is older than --min-age-hours (default 1) — an emission from
#      minutes ago is most likely being graded RIGHT NOW, and deleting the floor
#      out from under a grading unit makes its payload unrecordable. Age is the
#      only signal available for that; there is no lock to read.
#
# It also reports round directories left under the pre-1.12.0 layout of
# .tp/rounds/ — `.tp/rounds/<base>/<phase>-r<N>` with no `<base>.tasks.json` at
# the repository root, i.e. a nested task file's round, which the driver now
# keys as `.tp/rounds/<dir>/<base>/` and no longer reads. Guard 3 applies.
#
# Each path is printed with its reason. Dry run by default. Nothing is deleted
# without --apply. Run it from the project root; with the tp plugin installed it
# is ${CLAUDE_PLUGIN_ROOT}/scripts/clean-emissions.sh.
#
# Usage:
#   scripts/clean-emissions.sh                     # report what it would remove
#   scripts/clean-emissions.sh --apply             # remove it
#   scripts/clean-emissions.sh --min-age-hours 24  # be more conservative

set -euo pipefail

APPLY=0
MIN_AGE_HOURS=1
ROOT="spec/.tp-review"
ROUNDS=".tp/rounds"

while [ $# -gt 0 ]; do
  case "$1" in
    --apply)          APPLY=1; shift ;;
    --min-age-hours)  MIN_AGE_HOURS="${2:?--min-age-hours needs a number}"; shift 2 ;;
    --root)           ROOT="${2:?--root needs a path}"; shift 2 ;;
    -h|--help)        sed -n '2,42p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)                echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

# Portable mtime-in-epoch-seconds: GNU stat and BSD stat disagree on flags, and
# this repository runs on both.
mtime() {
  python3 -c 'import os,sys;print(int(os.stat(sys.argv[1]).st_mtime))' "$1"
}

# newest prints the newest mtime of any file under a directory.
newest() {
  python3 -c 'import os,sys
m=0
for d,_,fs in os.walk(sys.argv[1]):
    for f in fs: m=max(m,int(os.stat(os.path.join(d,f)).st_mtime))
print(m)' "$1"
}

now=$(date +%s)
cutoff=$(( MIN_AGE_HOURS * 3600 ))

removable=()
reasons=()
kept_recorded=0
kept_tracked=0
kept_young=0

if [ -d "$ROOT" ]; then
  while IFS= read -r f; do
    base=$(basename "$f")
    dir=$(dirname "$f")

    round=$(printf '%s' "$base" | sed -n 's/.*round-\([0-9][0-9]*\)[.-].*/\1/p')
    [ -n "$round" ] || continue

    case "$base" in
      *ground*) phase=ground ;;
      *audit*)  phase=audit  ;;
      *)        phase=review ;;
    esac
    inbox=0
    case "$(basename "$dir")" in
      ground-inbox) phase=ground; inbox=1 ;;
      review-inbox) phase=review; inbox=1 ;;
      audit-inbox)  phase=audit;  inbox=1 ;;
    esac
    [ "$inbox" -eq 0 ] || dir=$(dirname "$dir")

    # Guard 1 — the round is recorded, so these two files are its evidence.
    if [ "$inbox" -eq 0 ] && [ -f "$dir/$phase-round-$round.ndjson" ]; then
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

    if [ "$inbox" -eq 1 ]; then
      if [ -f "$dir/$phase-round-$round.ndjson" ]; then reason="recorded inbox file"; else reason="unrecorded inbox file"; fi
    else
      case "$base" in
        floor-*) reason="orphan floor" ;;
        *)       reason="orphan snapshot" ;;
      esac
    fi
    removable+=("$f"); reasons+=("$reason")
  done < <(find "$ROOT" -type f \( -name 'floor-*round-*.txt' -o -name 'snapshot-*round-*.md' \
    -o -path '*/ground-inbox/round-*.ndjson' -o -path '*/review-inbox/round-*.ndjson' -o -path '*/audit-inbox/round-*.ndjson' \
    -o -path '*/audit-inbox/round-*.pools.json' -o -path '*/review-inbox/round-*.roles.json' \) | sort)
fi

if [ -d "$ROUNDS" ]; then
  while IFS= read -r d; do
    base=$(basename "$(dirname "$d")")
    # A root task file's rounds never moved.
    [ -f "$base.tasks.json" ] && continue
    if [ $(( now - $(newest "$d") )) -lt "$cutoff" ]; then
      kept_young=$(( kept_young + 1 )); continue
    fi
    removable+=("$d"); reasons+=("old-layout round dir")
  done < <(find "$ROUNDS" -mindepth 2 -maxdepth 2 -type d \( -name 'review-r[0-9]*' -o -name 'audit-r[0-9]*' \) | sort)
fi

echo "kept: $kept_recorded recorded · $kept_tracked tracked · $kept_young younger than ${MIN_AGE_HOURS}h"

if [ ${#removable[@]} -eq 0 ]; then
  echo "nothing to remove"
  exit 0
fi

for i in "${!removable[@]}"; do
  printf '%s  %s\n' "${removable[$i]}" "${reasons[$i]}"
done
echo "${#removable[@]} path(s)"

if [ "$APPLY" -eq 1 ]; then
  rm -rf "${removable[@]}"
  echo "removed."
else
  echo "dry run — pass --apply to remove, or --min-age-hours N to be stricter"
fi
