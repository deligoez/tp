#!/usr/bin/env bash
# Fixtures for check-test-rows-cite-sidecar.py, one per way the scan can empty
# itself while returning success.
#
# Each of the first three fixtures holds a Tests table whose fixture cell cites
# the measurements sidecar — the exact defect the checker exists to refuse — and
# each was measured returning `scanned 0 Tests-table row(s)` and exit 0 before
# the repair that made an empty scan an error. They are here because reading the
# checker does not show that: the scan empties in three different places (the
# header tokeniser, the heading regex, the fence toggle) and every one of them
# looks like a clean corpus from the exit code.
#
# Run: scripts/check-test-rows-cite-sidecar-test.sh
set -u

here=$(cd "$(dirname "$0")" && pwd)
checker="$here/check-test-rows-cite-sidecar.py"
data="$here/testdata/cite-sidecar"
failures=0

expect() {
	want=$1
	fixture=$2
	why=$3
	out=$(python3 "$checker" "$data/$fixture" 2>&1)
	got=$?
	if [ "$got" != "$want" ]; then
		printf 'FAIL %-20s want exit %s, got %s (%s)\n' "$fixture" "$want" "$got" "$why"
		printf '%s\n' "$out" | sed 's/^/       /'
		failures=$((failures + 1))
	else
		printf 'ok   %-20s exit %s (%s)\n' "$fixture" "$got" "$why"
	fi
}

expect 1 plural-header.md "a header written 'fixtures' names no column"
expect 1 heading-suffix.md "'## 5. Tests and fixtures' opens no section"
expect 1 unclosed-fence.md "an unclosed fence above the section swallows it"
expect 1 cites.md "a scanned cell citing the sidecar is the defect itself"
expect 0 clean.md "a scanned table carrying its own fixture and mutant"

if [ "$failures" != 0 ]; then
	printf '%s fixture(s) failed\n' "$failures"
	exit 1
fi
echo "5 fixtures passed"
