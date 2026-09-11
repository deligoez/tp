#!/usr/bin/env bash
# Fixtures for audit-round-prep.py: one throwaway git repository holding one
# recorded audit round, exercised through the four decisions the script makes.
#
# The round file carries six rows across two roles plus one row with no role at
# all, and the six are the cross-product that matters: a PASS whose evidence
# file is untouched (the only kind that may be carried), a PASS whose evidence
# file is modified after the round was recorded, a PASS naming no evidence file,
# a PARTIAL, a FAIL, and a row with no `role` key. A delta re-grade that carried
# any of the last five would tell a role it need not re-measure something the
# repair round changed underneath it, which is the failure this script exists to
# make impossible — and it is invisible from the exit code, so it is asserted
# here row by row.
#
# One carried row deliberately holds a non-ASCII em dash. Byte-identity is the
# contract for a carried line and a reimplementation that round-tripped rows
# through json.dumps would pass every count assertion while escaping it, so the
# test diffs the carried file against a grep of the source rather than counting.
#
# The two go-safety rows that name a file carry `file_check` ids of the current
# derivation (`file-<role>-<slug>.<16 hex>`) and the rest carry spec ids, so the
# main fixture is a round recorded under the current derivation. Further
# fixtures follow it: a round whose `file_check` ids are of the earlier
# derivation (no digest), which must be refused on a tree where one of its rows
# would otherwise be carried; and a round file committed again after its record
# (a disposition), deleted and added back, or moved with its directory — none of
# which may move the commit the delta is measured from.
#
# Everything happens in a throwaway repository under mktemp. The git variables
# a caller's own repository can export (GIT_DIR, GIT_INDEX_FILE, ... — set, for
# one, inside a git hook) are cleared first, or the fixture's `git add -A` and
# `git commit` would land in that repository instead.
#
# Run: scripts/audit-round-prep-test.sh
set -u
# shellcheck disable=SC2046
unset $(git rev-parse --local-env-vars)

here=$(cd "$(dirname "$0")" && pwd)
script="$here/audit-round-prep.py"
failures=0

pass() { printf 'ok   %s\n' "$1"; }
fail() {
	printf 'FAIL %s\n' "$1"
	shift
	[ $# -gt 0 ] && printf '%s\n' "$*" | sed 's/^/       /'
	failures=$((failures + 1))
}

check_eq() {
	# check_eq <label> <want> <got>
	if [ "$2" = "$3" ]; then
		pass "$1 = $3"
	else
		fail "$1" "want [$2], got [$3]"
	fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
repo="$tmp/repo"
out="$tmp/out"
mkdir -p "$repo/src" "$repo/spec/.tp-review/x" "$out"

cd "$repo" || exit 1
git init -q .
git config user.email tester@example.invalid
git config user.name Tester
git config commit.gpgsign false

printf 'package src // untouched\n' >src/untouched.go
printf 'package src // touched\n' >src/touched.go
printf '# x\n\n## 1. Thing\n\nA thing.\n' >spec/x.md

# A buildable module, so the clone-and-build path is exercised rather than
# described: without it the only reachable verdict is the failure branch, and a
# brief that names a binary nobody built is the defect rule 1 exists to prevent.
mkdir -p cmd/tp
printf 'module example.invalid/x\n\ngo 1.21\n' >go.mod
printf 'package main\n\nfunc main() { println("x") }\n' >cmd/tp/main.go

round=spec/.tp-review/x/audit-round-1.ndjson
id_untouched=file-go-safety-src-untouched-go.0123456789abcdef
id_touched=file-go-safety-src-touched-go.fedcba9876543210
{
	printf '%s\n' '{"role":"go-safety","status":"PASS","item_id":"'"$id_untouched"'","evidence_file":"src/untouched.go","evidence_lines":"1-1","notes":"carried — this note holds an em dash on purpose"}'
	printf '%s\n' '{"role":"go-safety","status":"PASS","item_id":"'"$id_touched"'","evidence_file":"src/touched.go","evidence_lines":"1-1","notes":"its evidence file moves after the record"}'
	printf '%s\n' '{"role":"go-safety","status":"PASS","item_id":"gs-no-evidence","evidence_file":"","evidence_lines":"","notes":"PASS naming no evidence file"}'
	printf '%s\n' '{"role":"ax-contract","status":"PARTIAL","item_id":"ax-partial","evidence_file":"src/untouched.go","evidence_lines":"1-1","notes":"not PASS"}'
	printf '%s\n' '{"status":"PASS","item_id":"orphan-row","evidence_file":"src/untouched.go","evidence_lines":"1-1","notes":"no role key at all"}'
	printf '%s\n' '{"role":"ax-contract","status":"FAIL","item_id":"ax-fail","evidence_file":"src/untouched.go","evidence_lines":"1-1","notes":"not PASS"}'
} >"$round"

git add -A
git commit -qm "record audit round 1"
record_sha=$(git rev-parse HEAD)

# The repair the round provoked: one evidence file changes, one does not.
printf 'package src // touched, after the record\n' >src/touched.go
git add -A
git commit -qm "repair the touched file"

summary="$out/summary.json"
python3 "$script" spec/x.md --no-build --out "$out" >"$summary" 2>"$out/err.txt"
code=$?
check_eq "clean run exit code" 0 "$code"
if [ "$code" != 0 ]; then
	sed 's/^/       /' "$out/err.txt"
fi

field() { python3 -c 'import json,sys;print(eval("d"+sys.argv[2],{"d":json.load(open(sys.argv[1]))}))' "$summary" "$1"; }

check_eq "go-safety carried" 1 "$(field '["roles"]["go-safety"]["carried"]')"
check_eq "go-safety remeasure" 2 "$(field '["roles"]["go-safety"]["remeasure"]')"
check_eq "ax-contract carried" 0 "$(field '["roles"]["ax-contract"]["carried"]')"
check_eq "ax-contract remeasure" 2 "$(field '["roles"]["ax-contract"]["remeasure"]')"
check_eq "unknown carried" 1 "$(field '["roles"]["unknown"]["carried"]')"
check_eq "unknown remeasure" 0 "$(field '["roles"]["unknown"]["remeasure"]')"
check_eq "roles seen" "['ax-contract', 'go-safety', 'unknown']" "$(field '["roles"] and sorted(d["roles"])')"

check_eq "changed_files" "['src/touched.go']" "$(field '["changed_files"]')"
check_eq "clean tree is not dirty" False "$(field '["working_tree_dirty"]')"
check_eq "no tree under --no-build" None "$(field '["tree"]')"
check_eq "no binary under --no-build" None "$(field '["binary"]')"
check_eq "previous_round" 1 "$(field '["previous_round"]')"
check_eq "previous_record_sha" "$record_sha" "$(field '["previous_record_sha"]')"

# The re-measure list names the item ids of exactly the rows that were not carried.
check_eq "go-safety re-measure ids" "['$id_touched', 'gs-no-evidence']" \
	"$(python3 -c 'import json,sys;print(sorted(json.load(open(sys.argv[1]))))' "$out/remeasure-go-safety.json")"
check_eq "ax-contract re-measure ids" "['ax-fail', 'ax-partial']" \
	"$(python3 -c 'import json,sys;print(sorted(json.load(open(sys.argv[1]))))' "$out/remeasure-ax-contract.json")"

# Byte-identity: the carried line is the source line, em dash and all.
grep -F "\"item_id\":\"$id_untouched\"" "$round" >"$tmp/want-go-safety.ndjson"
if diff -q "$tmp/want-go-safety.ndjson" "$out/carried-go-safety.ndjson" >/dev/null; then
	pass "carried go-safety line is byte-identical to its source line"
else
	fail "carried go-safety line is byte-identical to its source line" \
		"$(diff "$tmp/want-go-safety.ndjson" "$out/carried-go-safety.ndjson")"
fi
grep -F '"item_id":"orphan-row"' "$round" >"$tmp/want-unknown.ndjson"
if diff -q "$tmp/want-unknown.ndjson" "$out/carried-unknown.ndjson" >/dev/null; then
	pass "carried unknown line is byte-identical to its source line"
else
	fail "carried unknown line is byte-identical to its source line" \
		"$(diff "$tmp/want-unknown.ndjson" "$out/carried-unknown.ndjson")"
fi

# Under --no-build the brief says so, rather than naming a tree nobody built.
case "$(field '["brief"]["go-safety"]')" in
*"No tree was prebuilt"*) pass "the --no-build brief names no tree" ;;
*) fail "the --no-build brief names no tree" "$(field '["brief"]["go-safety"]')" ;;
esac

# The build path, and with it the brief a role actually pastes: all four of the
# things it is for, in one paragraph.
printf '%s\n' '{"silent-swallowed-error":"refusals-that-name-nothing"}' >"$tmp/routed.json"
built="$tmp/built"
mkdir -p "$built"
summary="$built/summary.json"
python3 "$script" spec/x.md --out "$built" --routed "$tmp/routed.json" >"$summary" 2>"$built/err.txt"
check_eq "build run exit code" 0 "$?"
if [ ! -s "$summary" ]; then
	fail "build run produced a summary" "$(cat "$built/err.txt")"
else
	check_eq "tree is the clone" "$built/tree" "$(field '["tree"]')"
	check_eq "binary is inside the clone" "$built/tree/tp" "$(field '["binary"]')"
	# The reported path, not a path this test recomputes: the brief's whole
	# value is that a role can run what it names without checking.
	if [ -x "$(field '["binary"]')" ]; then
		pass "the binary the brief names exists and is executable"
	else
		fail "the binary the brief names exists and is executable" "$(ls -l "$built/tree" 2>&1)"
	fi
	brief=$(field '["brief"]["go-safety"]')
	for want in "do not clone, do not build" "verbatim" "$id_touched" \
		"silent-swallowed-error -> refusals-that-name-nothing" "expects no repair"; do
		case "$brief" in
		*"$want"*) pass "go-safety brief names '$want'" ;;
		*) fail "go-safety brief names '$want'" "$brief" ;;
		esac
	done
fi

# A second prep into the same --out would hand the roles a clone that is not
# HEAD, so it is refused rather than reused.
python3 "$script" spec/x.md --out "$built" --routed "$tmp/routed.json" >/dev/null 2>"$tmp/err-reuse.txt"
check_eq "a reused --out tree exits 3" 3 "$?"
case "$(cat "$tmp/err-reuse.txt")" in
*'"step": "clone"'*) pass "the refusal names the step that failed" ;;
*) fail "the refusal names the step that failed" "$(cat "$tmp/err-reuse.txt")" ;;
esac

summary="$out/summary.json"

# Nothing to carry: a spec whose round directory does not exist.
printf '# y\n' >spec/y.md
python3 "$script" spec/y.md --no-build --out "$out" >/dev/null 2>"$tmp/err-y.txt"
check_eq "no round directory exits 2" 2 "$?"
case "$(cat "$tmp/err-y.txt")" in
*".tp-review/y"*) pass "the refusal names the directory it looked in" ;;
*) fail "the refusal names the directory it looked in" "$(cat "$tmp/err-y.txt")" ;;
esac

# Nothing to carry: a round file git does not track.
mkdir -p spec/.tp-review/z
printf '# z\n' >spec/z.md
cp "$round" spec/.tp-review/z/audit-round-1.ndjson
python3 "$script" spec/z.md --no-build --out "$out" >/dev/null 2>"$tmp/err-z.txt"
check_eq "untracked round file exits 2" 2 "$?"
case "$(cat "$tmp/err-z.txt")" in
*"audit-round-1.ndjson"*) pass "the refusal names the untracked round file" ;;
*) fail "the refusal names the untracked round file" "$(cat "$tmp/err-z.txt")" ;;
esac

# A dirty working tree is reported, and its paths count as changed — the roles
# read the working tree, the clone is of HEAD, and only the operator can say
# which one a finding is about.
rm -rf spec/y.md spec/z.md spec/.tp-review/z
printf 'package src // uncommitted\n' >>src/untouched.go
python3 "$script" spec/x.md --no-build --out "$out" >"$summary" 2>"$out/err.txt"
check_eq "dirty run exit code" 0 "$?"
check_eq "dirty tree is reported" True "$(field '["working_tree_dirty"]')"
check_eq "dirty path is changed" "['src/touched.go', 'src/untouched.go']" "$(field '["changed_files"]')"
check_eq "the dirty evidence file is no longer carried" 0 "$(field '["roles"]["go-safety"]["carried"]')"

# Nothing to carry: a round whose file_check ids are of the earlier derivation
# (`file-<role>-<slug>`, no digest) names no item of the next round. Its PASS row
# cites a file untouched since the record, so without the refusal that row would
# be carried: a refusal tested on a tree where nothing would be carried anyway
# passes whether or not the rule exists. The spec-id row beside it is not what
# triggers the refusal; the main fixture's spec ids already run without one.
git checkout -q -- src/untouched.go
mkdir -p spec/.tp-review/o
printf '# o\n' >spec/o.md
{
	printf '%s\n' '{"role":"go-safety","status":"PASS","item_id":"file-go-safety-src-untouched-go","evidence_file":"src/untouched.go","evidence_lines":"1-1","notes":"earlier derivation: a slug and no digest"}'
	printf '%s\n' '{"role":"spec-coverage","status":"PASS","item_id":"spec-1-thing","evidence_file":"src/untouched.go","evidence_lines":"1-1","notes":"a spec item, not a file_check"}'
} >spec/.tp-review/o/audit-round-1.ndjson
git add -A
git commit -qm "record audit round 1 of o under the earlier derivation"
python3 "$script" spec/o.md --no-build --out "$out" >"$tmp/out-o.txt" 2>"$tmp/err-o.txt"
check_eq "earlier-derivation file_check ids exit 2" 2 "$?"
if [ -s "$tmp/out-o.txt" ]; then
	fail "the earlier-derivation refusal leaves stdout empty" "$(cat "$tmp/out-o.txt")"
else
	pass "the earlier-derivation refusal leaves stdout empty"
fi
case "$(cat "$tmp/err-o.txt")" in
*'"error": "earlier-derivation-file-check-ids"'*) pass "the refusal names why nothing is carried" ;;
*) fail "the refusal names why nothing is carried" "$(cat "$tmp/err-o.txt")" ;;
esac
case "$(cat "$tmp/err-o.txt")" in
*'"file-go-safety-src-untouched-go"'*) pass "the refusal names the earlier-derivation id" ;;
*) fail "the refusal names the earlier-derivation id" "$(cat "$tmp/err-o.txt")" ;;
esac

# The record commit is the one that ADDED the round file. A disposition written
# into it afterwards (`tp audit --resolve`) and committed must not move the
# delta's base past the repair it disposes of, or the repaired file reads as
# untouched and its row is carried without being re-measured.
mkdir -p spec/.tp-review/d
printf '# d\n' >spec/d.md
printf 'package src // before the repair\n' >src/x.go
d_round=spec/.tp-review/d/audit-round-1.ndjson
id_x=file-go-safety-src-x-go.00112233445566ff
{
	printf '%s\n' '{"role":"go-safety","status":"PASS","item_id":"'"$id_x"'","evidence_file":"src/x.go","evidence_lines":"1-1","notes":"its evidence file is repaired after the record"}'
	printf '%s\n' '{"role":"go-safety","status":"FAIL","item_id":"spec-1-thing","evidence_file":null,"evidence_lines":null,"notes":"the finding the repair answers"}'
} >"$d_round"
git add -A
git commit -qm "record audit round 1 of d"
d_record_sha=$(git rev-parse HEAD)
printf 'package src // repaired\n' >src/x.go
git add -A
git commit -qm "repair src/x.go"
python3 - "$d_round" <<'PY'
import json, sys

path = sys.argv[1]
rows = [json.loads(line) for line in open(path, encoding="utf-8") if line.strip()]
for row in rows:
    if row["status"] == "FAIL":
        row["resolved"] = {"status": "fixed", "evidence": "repaired src/x.go",
                           "resolved_at": "2026-09-11T00:00:00Z"}
with open(path, "w", encoding="utf-8") as fh:
    for row in rows:
        fh.write(json.dumps(row, ensure_ascii=False, separators=(",", ":")) + "\n")
PY
git add -A
git commit -qm "dispose of round 1's finding"
python3 "$script" spec/d.md --no-build --out "$out" >"$summary" 2>"$out/err.txt"
check_eq "disposition run exit code" 0 "$?"
check_eq "the record sha is the commit that added the round file" "$d_record_sha" "$(field '["previous_record_sha"]')"
check_eq "the repair committed before the disposition is changed" True \
	"$(field '["changed_files"] and "src/x.go" in d["changed_files"]')"
check_eq "the repaired evidence file is not carried" 0 "$(field '["roles"]["go-safety"]["carried"]')"
check_eq "go-safety re-measure ids after a disposition" "['$id_x', 'spec-1-thing']" \
	"$(python3 -c 'import json,sys;print(sorted(json.load(open(sys.argv[1]))))' "$out/remeasure-go-safety.json")"

# A round file deleted and added back has two adds; the oldest is the record.
cp "$d_round" "$tmp/d-round.ndjson"
git rm -q "$d_round"
git commit -qm "drop round 1 of d"
mkdir -p spec/.tp-review/d
cp "$tmp/d-round.ndjson" "$d_round"
git add -A
git commit -qm "restore round 1 of d"
python3 "$script" spec/d.md --no-build --out "$out" >"$summary" 2>"$out/err.txt"
check_eq "re-added round run exit code" 0 "$?"
check_eq "the record sha is the oldest add of a re-added round file" "$d_record_sha" "$(field '["previous_record_sha"]')"
check_eq "the repair before the re-add is still not carried" 0 "$(field '["roles"]["go-safety"]["carried"]')"

# A round directory moved after its record keeps its record commit. This
# repository moved unreleased specs and their .tp-review/ under spec/backlog/;
# the move adds the round file under its new path, and measuring from the move
# would carry a row whose evidence file the repair before the move changed.
mkdir -p spec/.tp-review/m
printf '# m\n' >spec/m.md
printf 'package src // m, before the repair\n' >src/m.go
id_m=file-go-safety-src-m-go.0000aaaa1111bbbb
printf '%s\n' '{"role":"go-safety","status":"PASS","item_id":"'"$id_m"'","evidence_file":"src/m.go","evidence_lines":"1-1","notes":"its evidence file is repaired before the move"}' \
	>spec/.tp-review/m/audit-round-1.ndjson
git add -A
git commit -qm "record audit round 1 of m"
m_record_sha=$(git rev-parse HEAD)
printf 'package src // m, repaired\n' >src/m.go
git add -A
git commit -qm "repair src/m.go"
mkdir -p spec/backlog/.tp-review
git mv spec/m.md spec/backlog/m.md
git mv spec/.tp-review/m spec/backlog/.tp-review/m
git commit -qm "move m under spec/backlog"
python3 "$script" spec/backlog/m.md --no-build --out "$out" >"$summary" 2>"$out/err.txt"
check_eq "moved round run exit code" 0 "$?"
check_eq "the record sha of a moved round is its original record" "$m_record_sha" "$(field '["previous_record_sha"]')"
check_eq "the repair before the move is changed" True \
	"$(field '["changed_files"] and "src/m.go" in d["changed_files"]')"
check_eq "the evidence file repaired before the move is not carried" 0 "$(field '["roles"]["go-safety"]["carried"]')"

if [ "$failures" != 0 ]; then
	printf '%s assertion(s) failed\n' "$failures"
	exit 1
fi
echo "all assertions passed"
