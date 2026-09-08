# Sample spec — a scanned table that carries its own fixture and mutant

## 5. Tests

| # | the fixture that separates them | the mutant that must fail it | what it changes |
|---|---|---|---|
| 1 | a task file holding two tasks, one blocked | flip the comparator | `<=` becomes `<` |
