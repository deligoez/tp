#!/bin/bash
# Mutation run for tp: go-mutesting with per-test selection, over the Go source lines
# changed since a base ref, in a fresh clone of HEAD outside the repository.
#
#   ./scripts/mutation.sh                 # lines changed since the latest tag
#   ./scripts/mutation.sh v1.5.0          # lines changed since v1.5.0
#   ./scripts/mutation.sh --all           # every line of every non-test file in the target packages
#
# Env: TP_MUT_PKGS (default "internal/engine"), TP_MUT_WORKERS (default 4),
#      TP_MUT_OUT (default ~/.cache/tp-mut),
#      TP_MUT_UPDATE_BASELINE=1 writes this run's survivors to scripts/mutation-baseline.json
#      (only after every survivor is classified equivalent or undocumented; see docs/lessons.md).
#
# Why this shape (docs/lessons.md, "Mutation testing"):
# - Only the changed lines are mutated (--git-diff-lines). Survivors are classified on
#   those lines alone, and the whole-file run spent most of its time elsewhere: v1.24.0's
#   158 escapes held 13 on changed lines, and v1.25.0's 20 files give 524 mutants whole
#   and 77 on changed lines (spec/1.25.0-measurements.md).
# - The line filter runs `git diff`, which honours a global diff.external and then parses
#   nothing: the filter admits zero mutants and the run reports a clean pass. The run gets
#   a global git config of its own carrying only the identity, and a dry count of zero
#   stops it instead of passing.
# - --per-test runs each mutant against only the tests whose coverage reaches it; on the
#   v1.6.0 epic files it gave the same verdict for every mutant as a whole-package run,
#   in under a third of the time.
# - The copy is a fresh clone outside the repo, so a run never touches round state and the
#   line filter has a history to diff against; it mutates HEAD, so uncommitted changes in
#   the target packages stop the run. The reports land under TP_MUT_OUT, which survives a
#   reboot (/tmp does not). The clone and the run's own build cache are removed at the end.
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
# shellcheck disable=SC2086 # pkgs is a space-separated list of pathspecs
git -C "$repo" diff --quiet HEAD -- $pkgs || { echo "uncommitted changes under [$pkgs]: the run mutates HEAD; commit first" >&2; exit 2; }
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
# Every mutant is a fresh build, so a run writes tens of GB of build cache. It gets a
# cache of its own, and the cache and the clone are removed when the run ends; the
# reports under $run stay.
export GOCACHE="$run/gocache"
trap 'rm -rf "$run/gocache" "$run/tree" "$run/gitconfig"' EXIT
git clone -q "$repo" "$run/tree"
git -C "$run/tree" checkout -q "$sha"
printf '%s\n' "${files[@]}" > "$run/files.txt"

# A global config with the identity alone: no diff.external for the line filter's
# `git diff`, and the tests that commit without setting a name still have one.
printf '[user]\n\tname = %s\n\temail = %s\n' \
  "$(git -C "$repo" config user.name || echo tp-mut)" \
  "$(git -C "$repo" config user.email || echo tp-mut@example.com)" > "$run/gitconfig"
export GIT_CONFIG_GLOBAL="$run/gitconfig"

keep="arithmetic/base arithmetic/negate expression/comparison conditional/negated numbers/incrementer numbers/decrementer"
disable=()
for m in $("$gm" --list-mutators); do
  case " $keep " in *" $m "*) ;; *) disable+=(--disable="$m") ;; esac
done

scope=()
[ "$base" != "--all" ] && scope+=(--git-diff-lines --git-diff-base="$base")

update=()
[ "${TP_MUT_UPDATE_BASELINE:-}" = 1 ] && update+=(--update-baseline)

cd "$run/tree"
count=$("$gm" --dry-run ${scope[@]+"${scope[@]}"} "${disable[@]}" "${files[@]}" 2>&1 | sed -n 's/^Total: \([0-9]*\) mutation.*/\1/p')
if [ -z "$count" ] || [ "$count" -eq 0 ]; then
  echo "the dry count found no mutant on the lines changed since $base in ${#files[@]} files; nothing would be tested." >&2
  echo "Either the changes hold no mutable code (comments, strings) or the line filter saw no diff: compare git diff $base HEAD -- ${files[*]}" >&2
  exit 3
fi
echo "mutating $count mutants in ${#files[@]} files since $base -> $run"
start=$(date +%s)
set +e
"$gm" --per-test --coverage --workers="$workers" --timeout-coefficient=3 --no-diffs --quiet \
  --logger-agentic-json --logger-summary-json --baseline="$repo/scripts/mutation-baseline.json" \
  ${scope[@]+"${scope[@]}"} ${update[@]+"${update[@]}"} "${disable[@]}" "${files[@]}" > "$run/run.log" 2>&1
code=$?
set -e
echo "exit $code wall $(( $(date +%s) - start ))s" >> "$run/run.log"
cp go-mutesting-*.json "$run/" 2>/dev/null || true
grep -E '^The (covered-code )?mutation score' "$run/run.log" || true
echo "survivors with diffs and kill hints: $run/go-mutesting-agentic.json"
exit "$code"
