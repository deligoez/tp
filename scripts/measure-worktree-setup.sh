#!/bin/bash
# Measures spec/backlog/worktree-setup.md §2 on a synthetic fixture whose gate
# reads a large ignored directory standing in for a build cache. The numbers in
# spec/1.24.0-measurements.md ("Worktree setup") came from:
#
#   go build -o /tmp/tp-dev/tp ./cmd/tp
#   OBJS=200 TASKS=6 REPS=3 scripts/measure-worktree-setup.sh /tmp/tp-dev/tp "$(mktemp -d)"
#
#   measure-worktree-setup.sh <tp binary> <work dir outside any repository>
#
# The setup is `/bin/cp -cR`, a copy-on-write clone on macOS (APFS); elsewhere
# set SETUP_CMD to the platform's equivalent (`cp -a --reflink=auto` on Linux).
# The units are scripts/measure-worktree-setup-unit.sh, spawned through
# TP_RUNNER_SEAM: the fake runner's batched close skips the unit's gate, so it
# would not measure the worktree gate this script is about.
set -euo pipefail
TP=$1
WORK=$2
OBJS=${OBJS:-200}
TASKS=${TASKS:-6}
REPS=${REPS:-3}
# Single quotes on purpose: the setup command expands $TP_MAIN_ROOT itself.
# shellcheck disable=SC2016
SETUP_CMD=${SETUP_CMD:-'/bin/cp -cR "$TP_MAIN_ROOT/cache" .'}
HERE=$(cd "$(dirname "$0")" && pwd)
export TP_HC=0 NO_COLOR=1

now() { python3 -c 'import time; print(time.time())'; }
elapsed() { python3 -c "print(f'{$2 - $1:.2f}')"; }

make_template() {
	local t=$WORK/template
	rm -rf "$t"
	mkdir -p "$t"
	cd "$t"
	git init -q -b main
	git config user.email m@example.com
	git config user.name m
	printf '# Spec\n## 1. Work\n### 1.1 Do\nDo it.\n' > spec.md
	# The gate: a cold cache is rebuilt (4 MB of hashing per object, the
	# stand-in for compiling), a warm one is only read and hashed.
	cat > gate.sh <<EOF
#!/bin/sh
if [ ! -f cache/.built ]; then
	mkdir -p cache
	i=0
	while [ \$i -lt $OBJS ]; do
		head -c 4194304 /dev/urandom | shasum -a 256 > /dev/null
		head -c 1048576 /dev/urandom > cache/obj\$i
		i=\$((i+1))
	done
	touch cache/.built
fi
cat cache/obj* | shasum -a 256 > /dev/null
EOF
	chmod +x gate.sh
	"$TP" --json init spec.md > /dev/null
	printf 'cache/\n' >> .gitignore
	"$TP" --json set --workflow --project quality_gate=./gate.sh commit_strategy=builtin > /dev/null
	for i in $(seq 1 "$TASKS"); do
		"$TP" --json add "{\"id\":\"t$i\",\"title\":\"T$i\",\"estimate_minutes\":5,\"acceptance\":\"Done.\",\"source_sections\":[\"### 1.1 Do\"],\"depends_on\":[],\"touches\":[\"t$i.txt\"]}" > /dev/null
	done
	./gate.sh
	git add -A
	git commit -q -m base
}

gate_times() {
	local t=$WORK/template
	cd "$t"
	echo "## gate (seconds, $REPS runs each)"
	for _ in $(seq 1 "$REPS"); do
		s=$(now); ./gate.sh; e=$(now)
		echo "main-warm $(elapsed "$s" "$e")"
	done
	for r in $(seq 1 "$REPS"); do
		wt=$WORK/wt-cold-$r
		git worktree add -q "$wt" HEAD 2> /dev/null
		cd "$wt"
		s=$(now); ./gate.sh; e=$(now)
		echo "worktree-cold $(elapsed "$s" "$e")"
		cd "$t"
		git worktree remove --force "$wt"
	done
	for r in $(seq 1 "$REPS"); do
		wt=$WORK/wt-setup-$r
		git worktree add -q "$wt" HEAD 2> /dev/null
		cd "$wt"
		s=$(now); TP_MAIN_ROOT=$t sh -c "$SETUP_CMD"; m=$(now); ./gate.sh; e=$(now)
		echo "worktree-setup-cp $(elapsed "$s" "$m") worktree-setup-gate $(elapsed "$m" "$e")"
		[ -z "$(git status --porcelain --untracked-files=normal)" ] || echo "DIRTY after setup"
		cd "$t"
		git worktree remove --force "$wt"
	done
}

drive() {
	local cap=$1 setup=$2
	local run=$WORK/run-$cap-$setup
	rm -rf "$run"
	/bin/cp -R "$WORK/template" "$run"
	cd "$run"
	"$TP" --json set --workflow --project "run_max_parallel=$cap" "run_max_units=$TASKS" > /dev/null
	if [ "$setup" = setup ]; then
		"$TP" --json set --workflow --project "worktree_setup_cmd=$SETUP_CMD" > /dev/null
	fi
	git add -A
	git commit -q -m "cap $cap $setup"
	s=$(now)
	out=$(TP_RUNNER_SEAM=$HERE/measure-worktree-setup-unit.sh MEASURE_TP=$TP "$TP" --json run 2> "$run.stderr" || true)
	e=$(now)
	reason=$(printf '%s' "$out" | python3 -c 'import json,sys; print(json.load(sys.stdin)["stop_reason"])')
	done=$(python3 -c 'import json; print(sum(t["status"]=="done" for t in json.load(open("spec.tasks.json"))["tasks"]))')
	setups=$(python3 -c "import json,glob; s=json.load(open(glob.glob('.tp/run-*.json')[0])).get('setups',[]); print(len(s), f'{sum(r[\"duration_seconds\"] for r in s):.2f}')")
	echo "cap=$cap $setup wall=$(elapsed "$s" "$e") stop=$reason done=$done/$TASKS setups(count,total_s)=$setups"
}

mkdir -p "$WORK"
make_template
gate_times
echo "## wall time (seconds)"
for _ in $(seq 1 "$REPS"); do
	drive 1 none
	drive 2 setup
	drive 3 setup
	drive 2 none
	drive 3 none
done
