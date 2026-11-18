#!/bin/bash
# Measures spec/backlog/gate-slots.md's measurement on rolling's fixture
# (spec/1.25.0-measurements.md, "Rolling implement slots"): six independent
# tasks, a `sleep 1` gate that logs its start and end, and scripted units of
# 4 s for t1 and t4 and 1 s for the rest. The pool runs at run_max_parallel 4,
# once with run_max_parallel_gates unset (no cap) and once at 2. The numbers in
# spec/1.26.0-measurements.md came from:
#
#   go build -o /tmp/tp-dev/tp ./cmd/tp
#   REPS=3 scripts/measure-gate-slots.sh /tmp/tp-dev/tp "$(mktemp -d)"
#
#   measure-gate-slots.sh <tp> <work dir outside any repository>
#
# Each line is one run: its wall time, the peak of gates running at one instant
# from the gate log, the gate runs and the closes that reused the unit's gate,
# the units' slot waits (summed and longest, from the run state's rows) and the
# main gates' (from its totals), the 1-minute load average before the run, and
# the stop reason.
set -euo pipefail
TP=$1
WORK=$2
REPS=${REPS:-1}
POOL=${POOL:-4}
HERE=$(cd "$(dirname "$0")" && pwd)
export TP_HC=0 NO_COLOR=1 UNIT_SLEEPS="t1=4 t4=4"

now() { python3 -c 'import time; print(time.time())'; }
load1() { sysctl -n vm.loadavg 2> /dev/null | tr -d '{}' | cut -d' ' -f2 || cut -d' ' -f1 /proc/loadavg; }

make_template() {
	local t=$WORK/template
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
	"$TP" --json init spec.md > /dev/null
	"$TP" --json set --workflow --project quality_gate=./gate.sh commit_strategy=builtin > /dev/null
	for i in 1 2 3 4 5 6; do
		"$TP" --json add "{\"id\":\"t$i\",\"title\":\"T$i\",\"estimate_minutes\":5,\"acceptance\":\"Done.\",\"source_sections\":[\"### 1.1 Do\"],\"depends_on\":[],\"touches\":[\"t$i.txt\"]}" > /dev/null
	done
	git add -A
	git commit -q -m base
}

# report prints one run's figures from its run state, gate log and
# integration logs.
report() {
	local gates=$1 wall=$2 log=$3 load=$4 reason=$5
	python3 - "$gates" "$wall" "$log" "$load" "$reason" <<'PY'
import glob, json, sys
gates, wall, log, load, reason = sys.argv[1], float(sys.argv[2]), sys.argv[3], sys.argv[4], sys.argv[5]
st = json.load(open(glob.glob(".tp/run-*.json")[0]))
events = []
for line in open(log):
    kind, *rest = line.split()
    if kind in ("gate-start", "gate-end"):
        events.append((float(rest[-1]), 1 if kind == "gate-start" else -1))
live = peak = 0
for _, step in sorted(events, key=lambda e: (e[0], e[1])):
    live += step
    peak = max(peak, live)
runs = sum(1 for _, step in events if step == 1)
reused = sum("gate reused from the unit's worktree" in open(f).read() for f in glob.glob(".tp/runs/*/integrate-*.log"))
waits = [u.get("wait_seconds") or 0 for u in st["units"]]
tot = st.get("totals", {})
tasks = json.load(open("spec.tasks.json"))["tasks"]
done = sum(t["status"] == "done" for t in tasks)
print(f"gates={gates} wall={wall:.1f} peak_gates={peak} gate_runs={runs} reused={reused}/6 "
      f"unit_wait_sum={sum(waits):.2f} unit_wait_max={max(waits):.2f} "
      f"main_wait_sum={tot.get('main_gate_wait_seconds', 0):.2f} main_wait_max={tot.get('main_gate_wait_max_seconds', 0):.2f} "
      f"main_rows={len(st.get('main_gates') or [])} load1={load} done={done}/6 stop={reason}")
PY
}

drive() {
	local gates=$1 rep=$2
	local run=$WORK/run-$gates-$rep
	rm -rf "$run"
	/bin/cp -R "$WORK/template" "$run"
	cd "$run"
	"$TP" --json set --workflow --project "run_max_parallel=$POOL" "run_max_parallel_gates=$gates" run_max_units=6 > /dev/null
	git add -A
	git commit -q -m "gates $gates"
	local log=$WORK/log-$gates-$rep
	: > "$log"
	local s e out load
	load=$(load1)
	s=$(now)
	out=$(MEASURE_LOG=$log TP_RUNNER_SEAM=$HERE/measure-rolling-implement-unit.sh MEASURE_TP=$TP "$TP" --json run 2> "$run.stderr" || true)
	e=$(now)
	report "$gates" "$(python3 -c "print($e - $s)")" "$log" "$load" \
		"$(printf '%s' "$out" | python3 -c 'import json,sys; print(json.load(sys.stdin)["stop_reason"])')"
}

mkdir -p "$WORK"
make_template
for rep in $(seq 1 "$REPS"); do
	for gates in 0 2; do
		drive "$gates" "$rep"
	done
done
