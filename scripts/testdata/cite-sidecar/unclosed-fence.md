# Sample spec — a fence above the section is never closed

## 1. Overview

```bash
tp review spec/sample.md --record merged.ndjson

## 5. Tests

| # | the fixture that separates them | the mutant that must fail it | what it changes |
|---|---|---|---|
| 1 | the one in the measurements file | flip the comparator | `<=` becomes `<` |
