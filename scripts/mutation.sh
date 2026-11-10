#!/bin/bash
# Mutation run for tp: go-mutesting with per-test selection, over the Go source files
# changed since a base ref, in a fresh copy outside the repository.
#
#   ./scripts/mutation.sh                 # files changed since the latest tag
#   ./scripts/mutation.sh v1.5.0          # files changed since v1.5.0
#   ./scripts/mutation.sh --all           # every non-test file in the target packages
#
# Env: TP_MUT_PKGS (default "internal/engine"), TP_MUT_WORKERS (default 4),
#      TP_MUT_OUT (default ~/.cache/tp-mut).
#
# Why this shape (docs/lessons.md, "Mutation testing"):
# - --per-test runs each mutant against only the tests whose coverage reaches it; on the
#   v1.6.0 epic files it gave the same verdict for every mutant as a whole-package run,
#   in under a third of the time.
# - The copy is fresh and outside the repo so a run never touches round state, and the
#   reports land under TP_MUT_OUT, which survives a reboot (/tmp does not).
# - Workers are pinned: the engine suite is itself parallel, so more workers on one
#   machine buy timeouts, not speed.
# - The mutator set matches what gremlins ran here before (arithmetic, comparison,
#   negation, numeric off-by-one), so survivor counts stay comparable across releases.
set -euo pipefail

repo=$(git rev-parse --show-toplevel)
pkgs=${TP_MUT_PKGS:-internal/engine}
workers=${TP_MUT_WORKERS:-4}
out_root=${TP_MUT_OUT:-$HOME/.cache/tp-mut}
gm=$(command -v go-mutesting || echo "$HOME/go/bin/go-mutesting")
[ -x "$gm" ] || { echo "go-mutesting not found: go install github.com/jonbaldie/go-mutesting/v2/cmd/go-mutesting@v2.10.43" >&2; exit 2; }

base=${1:-$(git -C "$repo" describe --tags --abbrev=0)}
files=()
for p in $pkgs; do
  if [ "$base" = "--all" ]; then
    list=$(git -C "$repo" ls-files "$p/*.go")
  else
    list=$(git -C "$repo" diff --name-only "$base" HEAD -- "$p")
  fi
  for f in $list; do
    case "$f" in *_test.go|*/testdata/*) continue ;; *.go) [ -f "$repo/$f" ] && files+=("$f") ;; esac
  done
done
if [ ${#files[@]} -eq 0 ]; then
  echo "no Go source changed under [$pkgs] since $base"
  exit 0
fi

sha=$(git -C "$repo" rev-parse --short HEAD)
run="$out_root/$sha-$(date +%Y%m%d-%H%M%S)"
mkdir -p "$run"
rsync -a --exclude .git/ "$repo/" "$run/tree/"
printf '%s\n' "${files[@]}" > "$run/files.txt"

keep="arithmetic/base arithmetic/negate expression/comparison conditional/negated numbers/incrementer numbers/decrementer"
disable=()
for m in $("$gm" --list-mutators); do
  case " $keep " in *" $m "*) ;; *) disable+=(--disable="$m") ;; esac
done

echo "mutating ${#files[@]} files since $base -> $run"
cd "$run/tree"
start=$(date +%s)
set +e
"$gm" --per-test --coverage --workers="$workers" --timeout-coefficient=3 --no-diffs --quiet \
  --logger-agentic-json --logger-summary-json --baseline="$repo/scripts/mutation-baseline.json" \
  "${disable[@]}" "${files[@]}" > "$run/run.log" 2>&1
code=$?
set -e
echo "exit $code wall $(( $(date +%s) - start ))s" >> "$run/run.log"
cp go-mutesting-*.json "$run/" 2>/dev/null || true
grep -E '^The (covered-code )?mutation score' "$run/run.log" || true
echo "survivors with diffs and kill hints: $run/go-mutesting-agentic.json"
exit "$code"
