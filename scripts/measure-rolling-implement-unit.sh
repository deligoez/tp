#!/bin/sh
# A scripted implement unit for scripts/measure-rolling-implement.sh, spawned
# through TP_RUNNER_SEAM (args: kind, id, log path, budget). It stands in for an
# agent: it sleeps for its task's time (UNIT_SLEEPS, "t1=4 t4=4", default 1
# second), commits one file, and closes through the real `tp done`
# ($MEASURE_TP), so the quality gate runs where the unit runs. Its start and
# end go to $MEASURE_LOG, outside the repository.
kind=$1
id=$2
log=$3
[ "$kind" = implement ] || exit 1
stamp() { python3 -c 'import time; print(f"{time.time():.3f}")'; }
secs=1
for pair in $UNIT_SLEEPS; do
	[ "${pair%%=*}" = "$id" ] && secs=${pair#*=}
done
{
	echo "unit-start $id $(stamp)" >> "$MEASURE_LOG"
	sleep "$secs"
	echo "$id" > "$id.txt"
	git add "$id.txt" && git commit -q -m "unit $id"
	sha=$(git rev-parse HEAD)
	"$MEASURE_TP" done "$id" --commit "$sha" -- "implemented by the scripted measurement unit, gate run by tp done"
	code=$?
	echo "unit-end $id $(stamp)" >> "$MEASURE_LOG"
	exit $code
} >> "$log" 2>&1
