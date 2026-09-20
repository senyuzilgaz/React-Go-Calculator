# CLAUDE.md

Full-stack calculator: Go REST API + React/TypeScript frontend.

## Documents

- `docs/DECISIONS.md` — architecture decision log. The reasoning behind everything here.
- `api/openapi.yaml` — the complete API contract. Hand-maintained; nothing is generated from it.
- `docs/API_EXAMPLES.md` — verified request/response pairs for every endpoint and error code.

## Decision log

`docs/DECISIONS.md` is **append-only**.

- Never edit, reword, or delete an existing entry.
- To change a decision, append a new entry naming the one it replaces. The old entry's text
  stays exactly as written — only its `Status` line may change to `Superseded by ADR-NNNN`.
- Append an entry whenever a change makes a non-obvious choice, or reverses an earlier one.
  An obvious choice with no alternative worth weighing does not need an entry.

## Comments

Write comments only when the code genuinely cannot carry the meaning itself — a non-obvious
constraint, a subtle reason, a reference to the ADR behind a rule. Everything else is expressed
through naming and structure, not prose.

No comments restating what the code does. No section banners. No commented-out code.

## Invariants

These are load-bearing. Changing any of them requires a new ADR entry.

- **The frontend performs no arithmetic.** Not addition, not rounding, not reformatting results.
  It collects operands, calls the API, and renders the returned string verbatim.
- **`result` is a string.** The server owns the rounding policy; JSON numbers would reintroduce
  the representation error it removes.
- **Rounding lives in exactly one function** in `internal/calc` — 12 significant digits,
  round-half-to-even, notation by magnitude band. See ADR-0015 for the algorithm.
- **`internal/calc` has no HTTP or JSON dependency.** It is pure arithmetic and the operation
  registry.
- **The operation registry is the single source of truth** for routing, arity validation, and
  the catalog response. Adding an operation is one registry entry.
- **Non-finite values never reach the wire.** `NaN` and `±Inf` become domain errors.

## Testing

Construct operands as runtime values, never as constant expressions — Go folds constants in
arbitrary precision, so such tests pass without exercising float64 at all.

Handler tests assert the status codes and response shapes in `api/openapi.yaml`. They are the
executable half of the contract.

## Scope

3–4 hour exercise judged on clarity and maintainability. ADR-0013 lists what is deliberately
not built; prefer removing work over adding it.
