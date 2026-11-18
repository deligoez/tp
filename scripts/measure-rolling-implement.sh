#!/bin/bash
# Measures spec/backlog/rolling-implement.md §4 on the fixture
# spec/1.25.0-measurements.md ("Rolling implement slots") describes: six
# independent tasks, a `sleep 1` gate that logs its start and end, and scripted
# units of 4 s for t1 and t4 and 1 s for the rest. The numbers there came from:
#
#   go build -o /tmp/tp-dev/tp ./cmd/tp
#   git archive -o /tmp/barrier.tar 3158f7d8 && mkdir /tmp/barrier && tar -xf /tmp/barrier.tar -C /tmp/barrier
#   (cd /tmp/barrier && go build -o /tmp/tp-barrier ./cmd/tp)
#   REPS=3 scripts/measure-rolling-implement.sh /tmp/tp-dev/tp /tmp/tp-barrier "$(mktemp -d)"
#
#   measure-rolling-implement.sh <rolling tp> <barrier tp> <work dir outside any repository>
#
# Each line is one run: its wall time, idle slot-seconds (cap x wall minus the
# summed unit durations in the run state), the peak of gates running at one
# instant from the gate log, the closes that reused the unit's gate, the tasks
# left solo, and the stop reason.
set -euo pipefail
ROLLING=$1
BARRIER=$2
WORK=$3
REPS=${REPS:-1}
HERE=$(cd "$(dirname "$0")" && pwd)
export TP_HC=0 NO_COLOR=1 UNIT_SLEEPS="t1=4 t4=4"

now() { python3 -c 'import time; print(time.time())'; }

make_template() {
	local tp=$1 t=$WORK/template
	rm -rf "$t"
	mkdir -p "$t"
	cd "$t"
	git init -q -b main
	git config user.email m@example.com
	git config user.name m
	printf '# Spec\n## 1. Work\n### 1.1 Do\nDo it.\n' > spec.md
	cat > gate.sh <<'EOF'
#!/bin/sh
stamp() { python3 -c 'import time; print(f"{time.time():.3f}")'; }
echo "gate-start $(stamp)" >> "$MEASURE_LOG"
sleep 1
echo "gate-end $(stamp)" >> "$MEASURE_LOG"
EOF
	chmod +x gate.sh
	"$tp" --json init spec.md > /dev/null
	"$tp" --json set --workflow --project quality_gate=./gate.sh commit_strategy=builtin > /dev/null
	for i in 1 2 3 4 5 6; do
		"$tp" --json add "{\"id\":\"t$i\",\"title\":\"T$i\",\"estimate_minutes\":5,\"acceptance\":\"Done.\",\"source_sections\":[\"### 1.1 Do\"],\"depends_on\":[],\"touches\":[\"t$i.txt\"]}" > /dev/null
	done
	git add -A
	git commit -q -m base
}

# report prints one run's figures from its run state, gate log and
# integration logs.
report() {
	local label=$1 cap=$2 wall=$3 log=$4 reason=$5
	python3 - "$label" "$cap" "$wall" "$log" "$reason" <<'PY'
import glob, json, sys
label, cap, wall, log, reason = sys.argv[1], int(sys.argv[2]), float(sys.argv[3]), sys.argv[4], sys.argv[5]
st = json.load(open(glob.glob(".tp/run-*.json")[0]))
busy = sum(u["duration_seconds"] or 0 for u in st["units"])
events = []
for line in open(log):
    kind, *rest = line.split()
    if kind in ("gate-start", "gate-end"):
        events.append((float(rest[-1]), 1 if kind == "gate-start" else -1))
live = peak = 0
for _, step in sorted(events, key=lambda e: (e[0], e[1])):
    live += step
    peak = max(peak, live)
reused = sum("gate reused from the unit's worktree" in open(f).read() for f in glob.glob(".tp/runs/*/integrate-*.log"))
tasks = json.load(open("spec.tasks.json"))["tasks"]
done = sum(t["status"] == "done" for t in tasks)
solo = sum(bool(t.get("solo")) for t in tasks)
reuse = "n/a" if cap == 1 else f"{reused}/6"
print(f"{label} cap={cap} wall={wall:.1f} idle_slot_s={cap * wall - busy:.1f} peak_gates={peak} gate_reuse={reuse} solo={solo} done={done}/6 stop={reason}")
PY
}

drive() {
	local label=$1 tp=$2 cap=$3 rep=$4
	local run=$WORK/run-$label-$cap-$rep
	rm -rf "$run"
	/bin/cp -R "$WORK/template" "$run"
	cd "$run"
	"$tp" --json set --workflow --project "run_max_parallel=$cap" run_max_units=6 > /dev/null
	git add -A
	git commit -q -m "cap $cap"
	local log=$WORK/log-$label-$cap-$rep
	: > "$log"
	local s e out
	s=$(now)
	out=$(MEASURE_LOG=$log TP_RUNNER_SEAM=$HERE/measure-rolling-implement-unit.sh MEASURE_TP=$tp "$tp" --json run 2> "$run.stderr" || true)
	e=$(now)
	report "$label" "$cap" "$(python3 -c "print($e - $s)")" "$log" \
		"$(printf '%s' "$out" | python3 -c 'import json,sys; print(json.load(sys.stdin)["stop_reason"])')"
}

mkdir -p "$WORK"
make_template "$ROLLING"
for rep in $(seq 1 "$REPS"); do
	for cap in 1 2 3; do
		drive barrier "$BARRIER" "$cap" "$rep"
		drive rolling "$ROLLING" "$cap" "$rep"
	done
done
