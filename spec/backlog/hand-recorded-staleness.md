# tp — A record clears staleness only through its own emission

A backlog decision, decided by the operator; it replaces the BUGS.md entry "A phase whose entire
recorded history was hand-recorded, with no emission, still clears `spec-stale` at `--record`".

Class: **decision**. One contract sentence.

## 1. The decision

**Context.** With no round that ever read text, staleness falls back to comparing the last recorded
round's stored hash, the pre-v1.2.2 answer, so recording without emitting clears `spec-stale` in a
phase that has never emitted. Since v1.2.2 a round carries the hash of its own emission snapshot.

**Decision.** One rule: a record clears staleness only through the snapshot of its own emission. A
record with no emission is still accepted, but it does not clear `spec-stale`, and the output says
why and names the emission that would bind a round to the text. The implementation is removing the
fallback, not adding a case for one kind of phase.

**Verify before building.** What else the fallback serves. A spec that converged before v1.2.2 has
rounds with a stored hash and no emission-snapshot hash. If such a history cannot be told apart from
"hand-recorded, never emitted", removing the fallback turns every spec converged before v1.2.2 stale
on upgrade — tp's own repository holds such specs, so dogfooding shows it at once — and that cost
goes back to the operator before the build. Also list what each test pinning the fallback asserts,
and whether SKILL describes a workflow that records without emitting.

**Consequences.** Someone who only records by hand sees `spec-stale` until they emit once per text
version.

## 2. Alternatives

- **Refuse the record.** Rejected: it turns a ledger-only use into an error for no gain.
- **Leave it.** Rejected by the operator.

## 3. Non-goals

- Changing how an emitted round's snapshot hash is computed.
