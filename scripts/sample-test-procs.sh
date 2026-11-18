#!/bin/bash
# Samples the test processes running outside tp's gate slots
# (spec/backlog/gate-slots.md, "What must be shown"): every --interval seconds
# it appends one line to --out, written as it is taken, so a run that crashes
# leaves its samples behind.
#
#   sample-test-procs.sh --out samples.tsv [--interval 2] [--root <project root>]
#   sample-test-procs.sh --summary samples.tsv
#
# A line is tab-separated:
#
#   <UTC time> <test binaries outside slots> <go test outside slots>
#   <test binaries inside slots> <slot holders> <1-minute load average>
#
# A test binary is a process whose argv[0] ends in ".test" (what `go test`
# builds and runs); a `go test` is a process whose argv[0] is go and argv[1]
# is test. "Inside slots" means a descendant of a slot holder: a process with
# an open descriptor on <root>/.tp/runs/*/gate-slots/slot-*.lock, found with
# lsof. An open descriptor is a held slot: gofrs/flock closes the descriptor
# of a failed try at once, and the gate's command runs close-on-exec, so it
# never inherits one. The sampler takes no lock, so it never delays a gate.
#
# --summary prints the sample count and the peak and median of each count
# column and of the load average.
set -euo pipefail

usage() {
	echo "usage: $0 --out <file> [--interval <seconds>] [--root <dir>] | --summary <file>" >&2
	exit 2
}

summary() {
	local file=$1
	[ -s "$file" ] || { echo "no samples in $file" >&2; exit 3; }
	awk -F'\t' '
		{ n++; for (c = 2; c <= 6; c++) col[c, n] = $c }
		function stat(c, label,    i, j, t, v, m) {
			for (i = 1; i <= n; i++) v[i] = col[c, i] + 0
			for (i = 2; i <= n; i++) { t = v[i]; for (j = i - 1; j >= 1 && v[j] > t; j--) v[j + 1] = v[j]; v[j + 1] = t }
			m = (n % 2) ? v[(n + 1) / 2] : (v[n / 2] + v[n / 2 + 1]) / 2
			printf "%s\tpeak=%s\tmedian=%s\n", label, v[n], m
		}
		END {
			printf "samples\t%d\n", n
			stat(2, "test_binaries_outside_slots")
			stat(3, "go_test_outside_slots")
			stat(4, "test_binaries_inside_slots")
			stat(5, "slot_holders")
			stat(6, "load1")
		}' "$file"
}

load1() {
	if [ -r /proc/loadavg ]; then
		cut -d' ' -f1 /proc/loadavg
	else
		sysctl -n vm.loadavg | tr -d '{}' | awk '{ print $1 }'
	fi
}

# holders prints the pids holding a gate slot under root, one per line.
holders() {
	local root=$1 files
	files=$(ls "$root"/.tp/runs/*/gate-slots/slot-*.lock 2> /dev/null || true)
	[ -n "$files" ] || return 0
	# shellcheck disable=SC2086 # one argument per slot file
	lsof -t $files 2> /dev/null | sort -u || true
}

# sample prints one line: the counts over the process table, with the slot
# holders' descendants set apart.
sample() {
	local root=$1 held
	held=$(holders "$root" | tr '\n' ' ')
	ps -axo pid=,ppid=,args= | awk -v held="$held" -v ts="$(date -u +%Y-%m-%dT%H:%M:%SZ)" -v load="$(load1)" '
		BEGIN { nh = split(held, h, " "); for (i = 1; i <= nh; i++) holder[h[i]] = 1 }
		{
			pid = $1; parent[pid] = $2
			prog = $3; sub(/.*\//, "", prog)
			if (prog ~ /\.test$/) kind[pid] = "bin"
			else if (prog == "go" && $4 == "test") kind[pid] = "go"
		}
		function inside(p,    hops) {
			for (hops = 0; p != "" && p != 0 && hops < 64; hops++) {
				if (p in holder) return 1
				p = parent[p]
			}
			return 0
		}
		END {
			for (p in kind) {
				if (inside(p)) { if (kind[p] == "bin") in_bin++ }
				else if (kind[p] == "bin") out_bin++
				else out_go++
			}
			printf "%s\t%d\t%d\t%d\t%d\t%s\n", ts, out_bin, out_go, in_bin, nh, load
		}'
}

out="" interval=2 root=.
while [ $# -gt 0 ]; do
	case $1 in
	--out) out=$2; shift 2 ;;
	--interval) interval=$2; shift 2 ;;
	--root) root=$2; shift 2 ;;
	--summary) summary "$2"; exit 0 ;;
	*) usage ;;
	esac
done
[ -n "$out" ] || usage
while :; do
	sample "$root" >> "$out"
	sleep "$interval"
done
