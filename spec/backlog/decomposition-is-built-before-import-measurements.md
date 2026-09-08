# decompose-build-probe — measurements

Supplemental material for `decompose-build-probe.md`; the spec stands without it.

## Tasks that closed without a production change

Over `spec/1.1.0.tasks.json` at `0dc3ed44` (the cycle was at 14 of 16 tasks closed), every closed
task's commits were listed and their files classed as production Go
(`*.go` not ending `_test.go`), test Go, or other:

```bash
bash -c 'python3 - <<"PY"
import json,subprocess
d=json.load(open("spec/1.1.0.tasks.json"))
for t in d["tasks"]:
    if t.get("status")!="done": continue
    shas=t.get("commit_shas") or ([t["commit_sha"]] if t.get("commit_sha") else [])
    prod=set();test=set();doc=set()
    for s in shas:
        r=subprocess.run(["git","-c","diff.external=","show","--name-only","--format=",s],capture_output=True,text=True)
        for f in r.stdout.split():
            (test if f.endswith("_test.go") else prod if f.endswith(".go") else doc).add(f)
    print(t["id"],len(shas),len(prod),len(test),len(doc))
PY'
```

Counting rule: a *code task* is one whose closing commits touch at least one `.go` file; a task
*closed by a guard* is a code task whose commits touch test files only. At the commit of writing:
14 closed, 10 code tasks, of which 3 closed by a guard (`record-legal-file-records`,
`record-preresolved-boundary`, `merge-representative-evidence`) and one is the fixture migration,
which touched thirty test files and no production file by design. The three are the tasks whose
acceptance predicted a production change that `HEAD` already satisfied; each unit found this by
building. `git -c diff.external=` is required on this machine, where `diff.external` is set
globally (see `CLAUDE.md`'s measurement traps).

## The predictions that did not hold

The `v1.1.0` cycle's decomposition sized the fixture migration on a narrow probe; the tasks whose
*"these tests turn red"* predictions failed for that reason are recorded in their own closure
reasons in `spec/1.1.0.tasks.json` and in `spec/1.1.0-measurements.md` under the round-3 and
implementation sections. The one measurement that saw the real size before implementation was the
implementer role's clone build during review round 1 (`spec/1.1.0-measurements.md`, the section on
what review round 1 measured), which reported the broken-test count the decomposition had not.

## What the earlier cycle measured

`spec/1.0.1-measurements.md`, *What implementation did that five review rounds could not*: every
implementing task corrected something a previous unit or the spec had asserted, each from a run.
The task count went from seven to ten when acceptance criteria were recounted as tests rather than
as findings.

## What a task file passes before import, at the same commit

`tp validate --help` at `0dc3ed44` names the checks it runs; the spec's context paragraph rests on
the observation that none of them executes anything. Derive rather than trust the sentence:

```bash
bash -c '/tmp/tp-dev/tp validate --help | sed -n "1,40p"'
```

The checks are shape checks over the task file (atomicity limits, source-section coverage, dependency
cycles, references, schema, id uniqueness); the list is the output above, not this paragraph.
