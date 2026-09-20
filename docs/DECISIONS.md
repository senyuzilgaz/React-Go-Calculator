# Architecture Decision Log

Decisions that shape this codebase, recorded as they are accepted.

## How this file works

This log is **append-only**.

- Existing entries are never edited, reworded, or deleted — not even to fix a decision that
  later turns out to be wrong. They are a record of what was decided and why, at the time.
- To change a decision, **append a new entry** that supersedes the old one. The new entry
  names the entry it replaces; the superseded entry keeps its original text.
- The only permitted edit to an existing entry is flipping its `Status` line to
  `Superseded by ADR-NNNN`. Nothing else in it changes.
- Entries are numbered sequentially and never renumbered.

Status values: `Accepted` · `Superseded by ADR-NNNN` · `Proposed`

---

## ADR-0001 — Single Go backend service

**Status:** Accepted · 2026-09-20

**Context.** The brief asks for "Go backend microservices" (plural). The deliverable is a
3–4 hour exercise judged on clarity and maintainability.

**Decision.** Ship **one** Go HTTP service. It is small, single-purpose, and independently
deployable — which is what "microservice" describes. The split seam is preserved in the code:
`internal/calc` holds pure arithmetic with no transport dependencies, so extracting a second
service later is a packaging change, not a rewrite.

**Alternatives considered.**
- *Two services (basic ops / advanced ops) behind a gateway.* Demonstrates the pattern
  literally. Costs roughly a quarter of the available time in gateway routing, duplicated
  config/logging/CORS, and cross-service error mapping — for zero user-visible benefit.
- *One service per operation (seven).* Cargo cult; rejected outright.

**Consequences.** The README must state this reasoning explicitly, so the single service reads
as a deliberate trade-off rather than an unread brief. A reviewer who disagrees still sees the
decision was made consciously.

---

## ADR-0002 — REST API shape: per-operation endpoints plus a discovery catalog

**Status:** Accepted · 2026-09-20

**Context.** Seven operations with mixed arity (√ is unary, the rest binary). The API has to be
obvious to a reviewer reading it cold, and cheap to extend.

**Decision.**

- `POST /api/v1/operations/{op}` — execute one operation.
- `GET  /api/v1/operations` — return the operation catalog.
- `GET  /healthz` — liveness.

Request bodies use a **uniform operand array**, never named parameters:

```json
POST /api/v1/operations/divide
{ "operands": [10, 4] }
```

```json
POST /api/v1/operations/sqrt
{ "operands": [9] }
```

Named parameters (`{"a":…,"b":…}`) were rejected because they degrade for the unary case and
force per-operation request structs.

Success response:

```json
{ "operation": "divide", "operands": [10, 4], "result": "2.5" }
```

The catalog is the single source of truth for operation metadata:

```json
{
  "operations": [
    {
      "id": "divide",
      "name": "Division",
      "symbol": "÷",
      "arity": 2,
      "parameters": [ { "name": "dividend" }, { "name": "divisor" } ],
      "endpoint": "/api/v1/operations/divide"
    }
  ]
}
```

**Implementation note.** Go 1.22+ `ServeMux` wildcard routing (`r.PathValue("op")`) means this
is **one handler plus an operation registry**, not seven handlers. The registry drives routing,
arity validation, and the catalog response from the same table — adding an operation is a
single registry entry with no other code change.

**Alternatives considered.**
- *Single `POST /api/v1/calculate` with `op` in the body.* Same economy, but the URL stops
  describing the resource and the catalog has no natural home.
- *Seven hand-written handlers.* Self-documenting but multiplies validation and error-mapping
  boilerplate.
- *`GET` with query parameters.* Semantically correct for pure computation and trivially
  cacheable, but floats in query strings are awkward and the body-based contract is clearer.

**Consequences.** An unknown operation is a **404** (the path segment names a resource that does
not exist), not a 400. The catalog endpoint must stay in sync with the registry automatically —
it is generated from it, never hand-maintained.

---

## ADR-0003 — Numeric precision and rounding policy

**Status:** Accepted · 2026-09-20

**Context.** A calculator's most visible correctness property is that `0.1 + 0.2` shows `0.3`.
In IEEE-754 binary64 that sum is `0.30000000000000004`. A reviewer will type it in. A further
constraint: JSON numbers are parsed by browsers as binary64 regardless of how the server
computed them, so any exactness that is not carried as a string dies at the wire.

**Decision.** Compute in `float64` and apply an explicit, documented **output rounding policy**.

*Rounding policy — normative:*

1. **Internal arithmetic** is IEEE-754 binary64 (`float64`) for every operation.
2. **Results are rounded to 12 significant decimal digits** before serialization. binary64
   carries approximately 15.95 significant decimal digits; discarding the last ~4 absorbs
   accumulated representation error (`0.30000000000000004` → `0.3`) while retaining far more
   precision than a calculator display requires.
3. **Rounding mode is round-half-to-even**, as implemented by
   `strconv.FormatFloat(value, 'g', 12, 64)`.
4. **Notation:** decimal for `1e-9 ≤ |value| < 1e12` and for zero; scientific
   (e.g. `1.23456789012e+15`) outside that band.
5. **Normalization:** trailing zeros stripped; negative zero rendered as `0`.
6. **Transport:** `result` is a JSON **string** carrying exactly the rounded form. The server
   owns the rounding policy, so the client renders the string verbatim and never re-formats or
   re-parses it. Operands are accepted as plain JSON numbers.
7. **Non-finite values never reach the wire.** `NaN` and `±Inf` are not valid JSON; results are
   checked before serialization and mapped to domain errors (see ADR-0004).

**Alternatives considered.**
- *`shopspring/decimal` for `+ − × ÷ %`, `float64` for `√` and `^`, operands and results as
  strings.* Genuinely exact for the four basic operations. Rejected as more machinery than this
  exercise warrants — one dependency, a division-scale constant, and a documented exact/
  approximate boundary running through the domain package.
- *`math/big.Rat`.* Stdlib and exact for `+ − × ÷`, but `√` and non-integer `^` are irrational
  so a float fallback is needed anyway, and rendering a rational as a decimal still requires
  choosing a precision — this policy's work, plus rationals to carry.
- *Raw `float64` with no rounding.* Zero work, but surfaces `0.30000000000000004` to the user.
- *Writing a custom arbitrary-precision implementation.* Out of scope at any time budget.

**Consequences — stated as limitations, not hidden.**
- Rounding **masks** representation error; it does not eliminate it. A value whose true error
  exceeds the 12th significant digit will still display that error.
- Because each `=` re-enters the rounded result as the next operand (ADR-0009), chained
  operations compound error at the 12th significant digit. Acceptable for a calculator UI;
  unacceptable for financial arithmetic.
- `√` and `^` are approximate by nature (`math.Sqrt`, `math.Pow`); no library changes this.
- **Migration path if exactness is later required:** move `+ − × ÷ %` to `shopspring/decimal`
  and change operand transport from JSON numbers to strings. The `result`-as-string contract
  established here already survives that change unmodified.
- Precision and rounding mode are **compile-time constants in one place**. They are deliberately
  not exposed as API parameters or configuration.
- Tests must assert the policy directly: `0.1 + 0.2` → `"0.3"`, `1 ÷ 3`, large-magnitude
  products crossing into scientific notation, and negative-zero normalization.

---

## ADR-0004 — Error model and HTTP status codes

**Status:** Accepted · 2026-09-20

**Context.** Division by zero, `√` of a negative, and `^` overflow must not produce `Inf`/`NaN`
in a response. The frontend needs to render distinct messages without string-matching prose.

**Decision.** A single structured error envelope:

```json
{ "error": { "code": "DIVISION_BY_ZERO", "message": "Cannot divide by zero" } }
```

Status code mapping:

| Status | Meaning | Codes |
|---|---|---|
| `400` | Malformed or unusable input | `INVALID_JSON`, `INVALID_OPERAND`, `WRONG_OPERAND_COUNT` |
| `404` | Operation does not exist | `UNKNOWN_OPERATION` |
| `422` | Well-formed, mathematically undefined | `DIVISION_BY_ZERO`, `NEGATIVE_SQRT`, `RESULT_OVERFLOW` |
| `500` | Unexpected server fault | `INTERNAL_ERROR` |

Non-finite operands (`NaN`, `±Inf`) are rejected as `INVALID_OPERAND`. Non-finite *results* are
`RESULT_OVERFLOW`.

Raw Go error strings are never surfaced. The frontend switches on `code`; `message` is a
fallback for humans and logs.

**Alternatives considered.** *`400` for everything, distinguished by `code`.* Simpler and
defensible. Rejected because the 400/404/422 split communicates the difference between
"you sent garbage", "that operation does not exist", and "that computation is undefined" —
which is information the status line should carry.

**Consequences.** The error taxonomy is a closed enum shared conceptually with the frontend
(ADR-0005). Adding a domain error means adding a code, not inventing a new envelope.

---

## ADR-0005 — Validation on both sides, with distinct responsibilities

**Status:** Accepted · 2026-09-20

**Decision.** Validate in both places, for different reasons.

- **Frontend** validates for *user experience*: block non-numeric key entry, disable `=` until
  the operand/operator/operand triple is complete, show inline messages. Never a security or
  correctness boundary.
- **Backend** validates as the *authority*, assuming a hostile client: JSON shape, operand
  count against the registry arity, operand finiteness, operation existence.

**Contract synchronization.** TypeScript request/response types are **hand-written** to mirror
the Go DTOs (~30 lines). Operation metadata is not duplicated at all — it comes from the
catalog endpoint at runtime (ADR-0002, ADR-0009).

**Alternatives considered.** *OpenAPI specification with client codegen.* Rejected as
disproportionate for three endpoints; the generator setup would cost more than the types it
produces. An `openapi.yaml` written purely as documentation remains an option with no
pipeline attached.

**Consequences.** The hand-written types can drift from the Go structs. Mitigated by keeping
them in one small file and by the contract being genuinely tiny.

---

## ADR-0006 — Standard library `net/http`, no web framework

**Status:** Accepted · 2026-09-20

**Decision.** Use stdlib `net/http` only. Go 1.22+ `ServeMux` provides method-and-pattern
routing with path wildcards, which covers every route in ADR-0002.

**Alternatives considered.** *chi* — the reasonable fallback if middleware composition grows
awkward; adds a dependency for capability already present. *gin / echo* — disproportionate, and
each imposes framework idioms a reviewer must context-switch into.

**Consequences.** Middleware (request ID, logging, CORS, panic recovery) is written as plain
`http.Handler` decorators. Small, explicit, and readable without framework knowledge.

---

## ADR-0007 — Backend package layout

**Status:** Accepted · 2026-09-20

**Decision.**

```
cmd/server/main.go     wiring, configuration, graceful shutdown
internal/calc/         pure arithmetic, operation registry, domain errors, rounding policy
                       — no HTTP, no JSON, no external dependencies
internal/httpapi/      handlers, DTOs, validation, error mapping, middleware
```

`internal/calc` is the testable core: pure functions, table-driven tests, no mocks. It is also
the seam referenced in ADR-0001.

**Alternatives considered.** *Repository / service / usecase layering with a DI container.*
Rejected — there is no persistence, no external I/O, and no second implementation of anything.
Interfaces with exactly one implementation are noise here.

**Consequences.** There is **no database and no persistence**. No calculation history is stored
and no history endpoint exists unless the brief later requires one.

---

## ADR-0008 — Frontend stack: Vite + React + TypeScript

**Status:** Accepted · 2026-09-20

**Decision.** Vite, React, TypeScript. Local state only, via a single `useCalculator` hook built
on `useReducer`. Styling is plain CSS with custom properties and CSS Grid for the keypad.

**Alternatives considered.** *Next.js* — nothing here is server-rendered or SEO-relevant; the
app is a static SPA calling one API. *Redux / Zustand / React Query* — disproportionate for a
single POST and one catalog fetch. *MUI / Chakra* — a component library would dominate the
diff and obscure the actual work.

**Consequences.** Responsive behaviour is hand-written: single-column grid at ≤480px, minimum
44px touch targets, `inputmode="decimal"` on entry, full keyboard support on desktop.

---

## ADR-0009 — Keypad UI on a reduced state machine, with no client-side arithmetic

**Status:** Accepted · 2026-09-20

**Context.** A classic keypad is what users expect and what reads as finished on mobile, but a
full calculator state machine (chained expressions, operator precedence, contextual behaviours)
is where take-home bugs concentrate — and it would duplicate the backend's job.

**Decision.** A keypad UI over a deliberately reduced state machine.

- A computation is **only ever** `(operand, operation, operand)` — or `(operation, operand)` for
  unary `√`.
- Each `=` issues **exactly one API call**.
- **The frontend performs no arithmetic of any kind.** Not addition, not rounding, not
  normalization of results. It accumulates keystrokes into operand strings, sends them, and
  renders the returned `result` string verbatim (ADR-0003).
- No chained expression evaluation, no operator precedence, no contextual operators. Pressing an
  operator after a completed computation re-enters the returned result as the next left operand.

**Operation keys are rendered from the catalog** (`GET /api/v1/operations`): labels, symbols, and
arity come from the server, so adding an operation requires no frontend change. Digit and
control keys (`0`–`9`, `.`, `±`, `C`) are static, since they are not operations.

**Alternatives considered.**
- *Form layout (operand A, operator select, operand B, `=`).* Bulletproof and trivially
  validated; retained as the fallback if time runs short. Looks like a form, not a calculator.
- *Free-text expression input.* Requires a tokenizer and precedence handling on one side or the
  other. Out of scope.

**Consequences.** The catalog fetch introduces a loading state and a failure mode at first
paint. Operation keys render disabled while loading and surface an error banner if the catalog
is unreachable; the numeric keypad renders immediately either way. This cost is accepted
deliberately, so that the catalog is load-bearing rather than decorative.

---

## ADR-0010 — Percentage is a plain binary operation

**Status:** Accepted · 2026-09-20

**Context.** `%` is ambiguous on physical calculators, where it is contextual: `200 + 10 %`
yields `220`, not `200.1`. Supporting that requires expression context in the client, which
directly contradicts ADR-0009.

**Decision.** `%` is an ordinary binary operation: `percent(a, b) = a × b ÷ 100` — "*a* percent
of *b*". The UI labels the key unambiguously so the semantics are visible without documentation.

**Consequences.** Contextual percentage is **explicitly out of scope** and stated as such in the
README, so its absence reads as a decision rather than a gap.

---

## ADR-0011 — Testing strategy

**Status:** Accepted · 2026-09-20

**Decision.**

- **Go, `internal/calc`** — table-driven unit tests carrying the weight: the rounding policy
  cases from ADR-0003, division by zero, `√` of a negative, `^` overflow to `Inf`, notation
  band boundaries, negative-zero normalization.
- **Go, `internal/httpapi`** — `httptest` handler tests covering each status code in ADR-0004
  plus the catalog response shape.
- **Frontend** — Vitest and React Testing Library over the reducer's state transitions and
  input validation, plus one interaction test that asserts a completed computation issues one
  request and renders the returned string.

**Alternatives considered.** *Playwright / end-to-end.* Wrong cost-benefit at this size; the
setup would consume time better spent on the domain tests that actually demonstrate rigour.

**Consequences.** Test value is concentrated where the logic is. The reducer test doubles as
the proof of ADR-0009's no-client-arithmetic rule.

---

## ADR-0012 — Delivery and observability

**Status:** Accepted · 2026-09-20

**Decision.**

- Multi-stage `Dockerfile` for the Go service, distroless final image.
- `docker-compose.yml` running the API and the built frontend behind nginx.
- `Makefile` with `dev`, `test`, `build`.
- **CORS:** Vite dev proxy locally (no CORS in development); a narrow middleware in production
  reading a single `ALLOWED_ORIGIN` environment variable.
- **Logging:** stdlib `log/slog` with the JSON handler, plus request-ID and panic-recovery
  middleware.

**Alternatives considered.** OpenTelemetry, Prometheus metrics, Kubernetes manifests, Helm —
all rejected as infrastructure without a consumer in this exercise.

**Consequences.** A single GitHub Actions workflow running `go test ./...` and `npm test` is a
cheap addition and may be included; nothing depends on it.

---

## ADR-0013 — Explicitly out of scope

**Status:** Accepted · 2026-09-20

**Context.** For a 3–4 hour exercise judged on clarity and maintainability, naming what was
deliberately excluded demonstrates more judgement than building any one of these would.

**Decision.** The following are deliberately **not** built, and the README says so:

gRPC or protobuf transport · message queue or event-sourced calculations · database and
calculation history · authentication and JWT · multiple physical services (ADR-0001) ·
OpenAPI client codegen (ADR-0005) · Redux or React Query (ADR-0008) · hand-rolled
arbitrary-precision arithmetic (ADR-0003) · Kubernetes, Helm, Terraform · rate limiting,
circuit breakers, retry-with-backoff · end-to-end browser tests (ADR-0011) · monorepo tooling
(nx, turborepo) · a plugin architecture for operations — the registry in ADR-0002 already makes
an operation a one-line addition.

**Consequences.** Each entry above is one sentence in the README. If a reviewer asks why
something is missing, the answer is recorded here with its reasoning.

---

## ADR-0014 — Hand-maintained OpenAPI specification as the contract, written before implementation

**Status:** Accepted · 2026-09-20

**Context.** ADR-0005 rejected an OpenAPI *codegen pipeline* but explicitly left a
documentation-only specification open. Both sides of the application are about to be built
against the same API, and the contract needs to be settled before either side starts so the
frontend and backend are not designed against different assumptions.

**Decision.** Write `api/openapi.yaml` (OpenAPI 3.1.0) as the complete contract, **before any
implementation**, covering all three endpoints, every schema, every error code, and worked
examples for each operation and each failure mode.

The specification is **maintained by hand and generates nothing**. No client, no server stubs,
no TypeScript types. Its purpose is to be the authoritative written description of the API
and the reference both implementations are built against.

**Alternatives considered.**
- *Codegen from the spec.* Rejected in ADR-0005 and still rejected; the generator setup would
  cost more than the ~30 lines of hand-written types it would replace.
- *Deriving the spec from Go annotations (swaggo or similar).* Inverts the dependency — the
  contract would then be a by-product of the implementation rather than something agreed before
  it, which defeats the purpose of writing it first.
- *No specification, README prose only.* Cheaper, but leaves the error taxonomy and response
  shapes to be discovered during implementation, which is exactly where frontend and backend
  assumptions diverge.

**Confirmed in this entry (restating ADR-0004 as implemented in the spec).** The complete
failure-mode taxonomy is:

| Status | Codes |
|---|---|
| `400` | `INVALID_JSON`, `INVALID_OPERAND`, `WRONG_OPERAND_COUNT` |
| `404` | `UNKNOWN_OPERATION` |
| `405` | `METHOD_NOT_ALLOWED` |
| `422` | `DIVISION_BY_ZERO`, `NEGATIVE_SQRT`, `RESULT_OVERFLOW` |
| `500` | `INTERNAL_ERROR` |

`405` was not listed in ADR-0004 and is added here: Go's `ServeMux` emits it automatically for a
registered path addressed with an unsupported method, so the contract documents what the server
actually does rather than what was originally anticipated. `INVALID_OPERAND` covers non-finite
*inputs* while `RESULT_OVERFLOW` covers non-finite *outputs* — the same condition on opposite
sides of the computation, deliberately given distinct codes and distinct statuses.

**Additional contract details settled here.**
- Operand roles are named by their mathematical function (`dividend`/`divisor`,
  `minuend`/`subtrahend`, `base`/`exponent`, `radicand`, `percentage`/`value`). Each name is
  unique within its operation, so catalog-driven UI labels are unambiguous.
- Every response carries an `X-Request-Id` header, echoed from the request when present and
  generated otherwise, matching the logging middleware in ADR-0012.
- `Content-Type` is not enforced; bodies are parsed as JSON regardless. Request bodies are
  size-limited, and an oversized body is reported as `400 INVALID_JSON`.

**Consequences.**
- The spec can drift from the implementation, since nothing enforces agreement. Mitigated by
  the handler tests in ADR-0011, which assert the documented status codes and response shapes
  directly — those tests are the executable half of this contract.
- Per-operation arity cannot be expressed in the schema, because the operation is a path
  parameter on a single path. The schema permits 1–2 operands and the exact count is enforced
  server-side against the registry. This limitation is stated in the spec itself.
- The specification is a review artifact in its own right: it shows the API was designed rather
  than accreted.

---

## ADR-0015 — Correction: the rounding policy's formatting algorithm

**Status:** Accepted · 2026-09-20
**Amends:** ADR-0003 (clause 3 only; ADR-0003 remains in force and is not superseded)

**Context.** ADR-0003 clause 3 stated that rounding is "round-half-to-even, as implemented by
`strconv.FormatFloat(value, 'g', 12, 64)`", while clause 4 required decimal notation for
`1e-9 <= |value| < 1e12`. These two clauses contradict each other. Go's `'g'` verb selects
scientific notation when the decimal exponent is less than `-4`, so a single `'g'` call renders
values in `[1e-9, 1e-4)` in scientific notation — `1 ÷ 1000000000` would produce `1e-09` where
clause 4 requires `0.000000001`.

The contradiction was found while producing verified examples for the README, by running both
formulations against the same inputs and comparing. It exists only in ADR-0003's implementation
note; `api/openapi.yaml` describes the policy correctly and needs no change.

**Decision.** Clauses 1, 2, 4, 5, 6 and 7 of ADR-0003 stand unchanged. Clause 3's implementation
note is replaced: rounding and notation are **two separate steps**, not one formatting call.

```
1. Reject non-finite values (NaN, ±Inf) before formatting — these are domain errors (ADR-0004).
2. Return "0" for zero, which normalizes negative zero.
3. Round to 12 significant digits:
       rounded = ParseFloat(FormatFloat(v, 'e', 11, 64))
4. Select notation by the magnitude of the rounded value:
       |rounded| >= 1e12 or |rounded| < 1e-9  ->  FormatFloat(rounded, 'e', -1, 64)
       otherwise                              ->  FormatFloat(rounded, 'f', -1, 64)
```

Step 3 uses `'e'` with precision 11 because `'e'` counts digits after the point, so precision 11
yields 12 significant digits, applied uniformly regardless of magnitude. Re-parsing produces the
rounded binary64 value. Step 4's `-1` precision emits the shortest representation that
round-trips, which strips trailing zeros for free and cannot exceed 12 significant digits, since
the 12-digit decimal from step 3 already round-trips to that value.

Rounding mode remains round-half-to-even, which is what `FormatFloat` performs.

**Verification.** The algorithm was executed against runtime binary64 arithmetic — not
compile-time constant expressions, which Go evaluates in arbitrary precision and which would
have silently hidden the very error the policy exists to absorb. Confirmed outputs:

| Operands | Raw binary64 | Result |
|---|---|---|
| `0.1 + 0.2` | `0.30000000000000004` | `"0.3"` |
| `0.3 − 0.1` | `0.19999999999999998` | `"0.2"` |
| `1.1 × 1.1` | `1.2100000000000002` | `"1.21"` |
| `1 ÷ 3` | `0.3333333333333333` | `"0.333333333333"` |
| `123456789 × 987654321` | `1.2193263111263526e+17` | `"1.21932631113e+17"` |
| `1 ÷ 1000000000` | `1e-09` | `"0.000000001"` |
| `1 ÷ 10000000000` | `1e-10` | `"1e-10"` |
| `0 × −5` | `-0` | `"0"` |

**Consequences.**
- The formatter is a single function in `internal/calc` with these four steps and no branching
  beyond them. It is the only place precision or notation is decided.
- The two band edges (`1e-9` and `1e12`) and the negative-zero case are required test cases —
  they are where a naive single-call implementation diverges from the policy.
- Tests must construct operands as runtime values, never as constant expressions, or they will
  pass without testing anything.
- `docs/API_EXAMPLES.md` records the verified request/response pairs for every endpoint and
  every error code, generated by executing this algorithm and marshalling the real response
  structures rather than by hand.

---

## ADR-0016 — Verified API examples as a documentation artifact

**Status:** Accepted · 2026-09-20

**Context.** The README will carry request/response examples. Hand-written examples in a
calculator's documentation are particularly hazardous: a plausible-looking but wrong result
(`"0.30000000000000004"`, or `1e-09` where the policy says `0.000000001`) directly undermines
the precision claims the documentation is making.

**Decision.** Maintain `docs/API_EXAMPLES.md` containing a worked example for every endpoint,
every operation, and all nine error codes. Values are **generated by execution** — results by
running the rounding policy against runtime binary64 arithmetic, JSON bodies by marshalling the
actual response structures — never written by hand.

**Consequences.**
- Examples can go stale if the implementation diverges. Mitigated by the handler tests
  (ADR-0011), which assert the same pairs; the examples file and the test table should be kept
  in correspondence.
- Where a raw float64 differs from the returned string, the document shows both, so the rounding
  policy is demonstrated rather than asserted.
- The file is structured for extraction into the README, including a status/code summary table.

## ADR-0017 — Transport package location and the routing decisions it forced

**Status:** Accepted · 2026-09-20
**Amends:** ADR-0007 (the `internal/httpapi/` path only; the rest of ADR-0007 remains in
force and is not superseded)

**Context.** ADR-0007 placed the HTTP layer at `internal/httpapi/`. The package was built at
`internal/transport/http/` instead, which names the layer by what it is — one transport —
rather than by the protocol it happens to speak. Writing the package also settled several
routing questions the contract describes the output of but not the mechanism for.

**Decision.**

- The HTTP layer lives at `internal/transport/http/`, package `http`. It imports `net/http`
  normally; a package's own name is not bound in its own file scope, so there is no
  ambiguity. A second transport, if one is ever added, sits beside it rather than inside it.
- **Every path is registered twice**, once with its method and once without. `ServeMux`
  prefers the more specific pattern, so a supported method reaches the handler and every
  other method falls through to the second registration. ADR-0014 assumed `ServeMux`'s
  automatic 405 was sufficient; it is not, because that 405 is `text/plain` and the
  contract publishes a JSON envelope with an `Allow` header for it.
- **Operation existence is resolved before the request body is read.** A `POST` to an
  operation that does not exist is `404 UNKNOWN_OPERATION` whatever the body contains. The
  path segment names a resource (ADR-0002), and a malformed body for a resource that does
  not exist is not the more useful thing to report.
- **Unroutable paths answer in the envelope too**, as `404 UNKNOWN_OPERATION` with a
  generic message. `UNKNOWN_OPERATION` is the only 404 in the closed enum of ADR-0004.
  This keeps the promise that every response from the API is JSON, at the cost of a code
  that reads oddly for a path like `/metrics`.
- **Operands are decoded one element at a time**, into `[]json.RawMessage` rather than
  `[]float64`, so a bad element is reported with its position as the contract requires.
  JSON `null` is rejected explicitly, because it unmarshals into a `float64` without error
  and would otherwise arrive silently as zero.
- **An inbound `X-Request-Id` is echoed only if it is printable ASCII and at most 128
  characters**, and replaced otherwise. The value is attacker-controlled and reaches every
  log line for the request.
- The request body is capped at 4 KiB, reported as `INVALID_JSON` per ADR-0014.

**Alternatives considered.**
- *`internal/httpapi/` as originally written.* Shorter, and avoids a package named after a
  stdlib one. Rejected for the seam: `transport/` states that HTTP is one way in rather
  than the only one, which is the same argument ADR-0001 makes about `internal/calc`.
- *Rewriting `ServeMux`'s plain-text 405 in middleware* by intercepting the status and
  substituting a body. Fewer registrations, but it recovers routing information after the
  router has already discarded it, and the `Allow` header would have to be trusted from a
  response already written.
- *Letting unrouted paths keep the stdlib's `text/plain` 404.* Honest about being outside
  the contract, but a JSON API that sometimes answers in prose is worse for a client than
  one that reuses a slightly ill-fitting code.

**Consequences.**
- Adding a route means two registrations, not one. The pairing is mechanical and visible in
  a single function; a missed second registration shows up as a plain-text 405 in the
  routing test.
- `calc.ErrInvalidOperand` is now unreachable from HTTP, because the decoder rejects
  non-numeric operands before `calc.Apply` is called. Its mapping is kept and tested
  directly, so a future change to the decode path degrades to a `400` rather than a `500`.
- Panic recovery cannot be provoked through the router, since no handler panics. That
  middleware is tested around a handler that does, still over a real server.

## ADR-0018 — Configuration is read once, validated strictly, and never defaulted silently

**Status:** Accepted · 2026-09-20

**Context.** ADR-0012 named `ALLOWED_ORIGIN` and the `log/slog` JSON handler but did not say
how settings are read, what happens when one is wrong, or which knobs exist at all.

**Decision.** `internal/config` reads four variables, each with a working default:

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8080` | Bound on every interface; a loopback bind is unreachable from a container |
| `LOG_LEVEL` | `info` | `debug` · `info` · `warn` · `error`, case-insensitive |
| `ALLOWED_ORIGIN` | unset | Enables CORS for exactly that origin; unset disables it |
| `SHUTDOWN_TIMEOUT` | `10s` | Grace period for in-flight requests on `SIGINT`/`SIGTERM` |

- **A malformed value is a startup failure, never a silent fallback.** A service that
  quietly ignores `PORT=htp` and listens somewhere else is harder to diagnose than one that
  refuses to start and says which variable is wrong.
- **Every problem is reported together**, via `errors.Join`, so a misconfigured deployment
  takes one restart to diagnose rather than four.
- **`ALLOWED_ORIGIN` must be a bare origin.** `*`, a trailing slash, or a path are rejected
  at startup. The CORS middleware compares the value to the `Origin` header exactly, so
  `https://app.example/` would deploy cleanly and then match nothing — a failure that shows
  up as a browser error with no server-side trace.
- **`Load` takes the lookup function as an argument** rather than calling `os.Getenv`, so
  configuration is tested without mutating process state.
- **Connection timeouts are constants in `main`, not settings.** The API is pure
  computation behind a 4 KiB body cap; a request outside these bounds is a stuck client,
  not slow work, and no deployment has a reason to differ.

**Alternatives considered.**
- *Falling back to defaults on a bad value, with a warning.* Keeps the service up, which is
  the wrong instinct for a value that changes where it listens or who may call it.
- *A flags-and-env library (viper, kong).* A dependency and a config file format to answer
  four environment variables.
- *Prefixing the variables (`CALC_PORT`).* Avoids collisions in a shared environment, but
  `PORT` is the convention nearly every container platform already sets.

**Consequences.**
- `main` is wiring only: load config, build the logger, build the router, run the server,
  shut it down. It holds no arithmetic, no routing, and no policy.
- `run` takes its context, environment, and log destination as arguments, so the wiring is
  covered by a test that starts a real listener, serves the contract, and cancels.
- `main` itself and the `ErrServerClosed` guard in the shutdown select are the only
  uncovered statements in the backend. The first ends in `os.Exit`; the second is the
  standard guard against reporting a clean stop as a failure.

---

## ADR-0019 — Frontend API layer: hand-written contract types, one error type, relative URLs

**Status:** Accepted · 2026-09-20

**Context.** ADR-0005 settled that the TypeScript types are hand-written, ADR-0008 and ADR-0011
named the stack. None of them says what the client module looks like: how a response is
trusted, what a component catches, or how the browser addresses the API. Writing it settled
those questions and a few more.

**Decision.**

- **`src/api/types.ts` mirrors `api/openapi.yaml` and nothing else.** Shapes the client never
  exchanges (`Health`) are omitted rather than declared unused. `ERROR_CODES` is declared as a
  value and `ErrorCode` derived from it, so the runtime check and the type cannot drift apart.
- **`OperationId` is an open union.** The contract enumerates today's seven operations while
  naming the catalog authoritative at runtime, and ADR-0009 requires a new operation to reach
  the UI with no frontend change. A closed union would turn valid data into a type error.
- **Everything leaving the API layer is a `CalcError`**, carrying the contract `code`, the HTTP
  status, and the `X-Request-Id`. Components switch on `code` and never parse `message`
  (ADR-0004); the request id is kept so a message a user reports can be found in the logs.
- **Two codes exist that the contract does not define:** `NETWORK_ERROR`, where fetch produced
  no response at all, and `MALFORMED_RESPONSE`. They live in `errors.ts` rather than
  `types.ts`, which is the specification's mirror and stays that. A response whose `code` falls
  outside the closed enum is `MALFORMED_RESPONSE`, not a relabelled `INTERNAL_ERROR`: the
  taxonomy is versioned with the contract, so a code this client does not know is an answer it
  cannot honour rather than a fault it can describe.
- **Responses are verified, not trusted.** In particular `result` is rejected unless it is a
  string. A server sending a JSON number would silently reintroduce exactly the representation
  error the rounding policy exists to remove (ADR-0003), and the calculator would go on looking
  like it worked.
- **Cancellation is re-thrown untouched.** Both functions take an `AbortSignal`; an aborted
  call rejects with its `AbortError`, which is not a failure and has nothing to show a user.
- **Request URLs are built from the documented path template, never from the catalog's
  `endpoint` field.** No request address should come out of a response body. The field remains
  published as documentation of where each operation lives.
- **The API is addressed by relative path**, with `vite.config.ts` proxying `/api` and
  `/healthz` to `:8080` for both `dev` and `preview`. The browser only ever calls the origin it
  was served from, so CORS is absent in development by construction rather than by
  configuration (ADR-0012), and the same paths work behind nginx in production. There is no
  configurable API base URL.
- **MSW backs the frontend tests**, added to the stack ADR-0011 named. The client is exercised
  through real `fetch` against real `Response` objects — statuses, headers, unparseable bodies,
  aborts — none of which a stubbed `fetch` would exercise. The catalog fixture is extracted
  from `docs/API_EXAMPLES.md` by script rather than retyped.

**Alternatives considered.**
- *zod or another runtime schema library.* Derives validators and types from one declaration.
  Rejected for ADR-0005's reason: a dependency and a schema DSL to describe four shapes the
  contract already states in thirty lines.
- *Returning a result union (`{ ok: false, error }`) rather than throwing.* Honest about
  failure being ordinary here, but every call site would branch twice and `await` would stop
  being the success path. One error type with `try`/`catch` is what the React code around it
  already reads as.
- *Mapping an unrecognised server code onto `INTERNAL_ERROR`.* Keeps a UI working against a
  newer server, at the cost of reporting a server fault when the real condition is a client too
  old to understand the answer.
- *A `VITE_API_BASE_URL` for a separately hosted API.* A configuration knob with no deployment
  that needs it; ADR-0012 puts the API and the assets behind one origin.
- *Stubbing `globalThis.fetch` instead of MSW.* One less dependency, but the assertions would
  be about the stub rather than about a request.

**Consequences.**
- Adding a code to the contract fails the frontend build until the UI says what it means: the
  code-to-message table is a total `Record`, so the compiler enforces the mapping.
- Response validation is hand-written and grows a line whenever a schema does. It is confined
  to `client.ts` and rejects into a single code.
- A component that catches must ignore or re-throw `AbortError` itself; `isAbortError` exists
  for that.
- Serving the frontend from a different origin than the API needs a change here. That is
  deliberate — no deployment in ADR-0012 does it.

---

## ADR-0020 — The calculator state machine, and what the hook is not allowed to decide

**Status:** Accepted · 2026-09-20

**Context.** ADR-0009 fixed the interaction model — a reduced state machine, one API call per
`=`, no client arithmetic — but not the keystroke rules that follow from it. Writing the tests
before the reducer forced each one to be stated as an assertion rather than discovered while
implementing.

**Decision.**

- **All calculator logic lives in `features/calculator/reducer.ts`**, which imports nothing from
  React and nothing that performs I/O. The hook holds no rule about digits, operands, or
  operations; if a condition about a key ever appears in the hook, it belongs in the reducer.
- **The phase carries the pending request:**
  `{ kind: 'entering' } | { kind: 'calculating', request } | { kind: 'result' }`. A request
  cannot exist without the calculating phase and the phase cannot exist without a request, so
  "one `=` is one call" is a property of the shape rather than a convention to maintain.
- **An action the machine ignores returns the state object it was given.** React then skips the
  render, and the hook can depend on `state.phase` identity: the effect that issues the request
  runs exactly once per calculation, and no key pressed mid-flight can start a second one.
- **One rule covers every operation key:** pressing it commits *the displayed value* as the left
  operand and then waits for `arity - 1` more. Binary operations wait for a second operand;
  `√` becomes submittable at once. The same rule is what re-enters a returned result as the next
  left operand (ADR-0009), so that behaviour is not a special case.
- **An operator pressed mid-entry re-anchors rather than chains.** `12 ÷ 4 +` leaves `4` as the
  left operand of `+`; nothing is computed, because computing is the server's job and chaining
  is out of scope.
- **`±` edits the entry, including a returned result**, turning `0.3` into `-0.3` as the next
  operand. It prefixes a character to a literal and performs no arithmetic; the result is no
  longer a result once the user has edited it, so the phase returns to `entering`.
- **A failure clears only the operand the user can retype** — the second one — and keeps the
  left operand and the operation. `=` is disabled until a new operand is entered, so the same
  rejected computation cannot simply be re-sent.
- **Every key is inert while a calculation is in flight except `C`,** which resets the machine.
- **`Number()` at the request boundary is the only place a typed string becomes a number**, and
  it parses a literal rather than computing with it. Operands are sent as JSON numbers, results
  come back as strings, and no state field is ever derived arithmetically from another.

**The hook.** `useCalculator` owns the reducer, issues the request the calculating phase
describes, and maps a `CalcError` to its message through `uiMessage` before dispatching it back.
It exposes `state`, `dispatch`, and the derived `status`, `display`, `canSubmit`, and `error`.

- **A request in flight is cancelled when the user leaves it** — clearing, or unmounting — by
  the effect's cleanup aborting the signal the client already accepts (ADR-0019). Because no new
  computation can start while one is in flight, those are the only two cases; there is no
  "newer request supersedes older" race to resolve.
- **A response that arrives anyway is ignored by the reducer**, which drops any result or
  failure that reaches it outside the calculating phase. The abort makes that rare and the guard
  makes it harmless.

**Alternatives considered.**
- *Unary as a prefix (`√` then the operand).* Matches ADR-0009's `(operation, operand)` notation
  literally, but needs a second rule for how the operation key treats the current entry, and the
  keys would behave differently depending on arity.
- *Evaluating a pending computation when a second operator is pressed*, the way a physical
  calculator chains. It is client-side arithmetic by another name unless it issues a second
  request, and ADR-0009 rules both out.
- *A session history of completed computations.* Deliberately not built: ADR-0013 lists
  calculation history as out of scope, and there is no UI that shows it. It is an append on
  `calculationSucceeded` in the reducer if that changes — not a field the hook accumulates.
- *Intent callbacks (`pressDigit`, `selectOperation`) instead of exposing `dispatch`.* Stable
  references and a narrower surface, but every one of them would be a wrapper with no logic in
  it, and the temptation to put a condition in one is exactly what this entry forbids.
- *Storing the display string in state.* It is `right ?? left ?? "0"`; derived state that can
  disagree with what it is derived from is a bug waiting to be written.

**Consequences.**
- The reducer is testable without React, a DOM, or a network: 38 tests construct states by
  folding actions, which is also where the no-arithmetic invariant is proven.
- Adding a rule means adding a test and a case, and the keypad stays a rendering of `display`,
  `canSubmit`, and the catalog.
- `C` is the only escape from a slow request. There is no timeout; a request that never answers
  leaves the calculator in `calculating` until the user clears it.

---

## ADR-0021 — Keypad and display: props in, buttons out

**Status:** Accepted · 2026-09-20

**Context.** ADR-0020 put every calculator rule in the reducer. The components that render it
still had to decide where intent becomes an action, what the keypad does while the catalog is
loading, and what it offers when the catalog never arrives.

**Decision.**

- **`Display` and `Keypad` take props and render real `<button>` elements.** They hold no state,
  no effects, and no rule; the only conditions in them are whether an optional message exists.
  Every key is a button, so keyboard activation, focus order, and assistive technology work
  without anything being reimplemented.
- **`App` is the single place an intent becomes an action.** It passes `onDigit`, `onOperation`
  and the rest, so the components never see `dispatch` or the action shapes. ADR-0020 rejected
  intent callbacks as the *hook's* surface for hiding logic behind wrappers; here the wrappers
  are the wiring itself, and they keep the reducer out of the component tree.
- **Operation keys are the only thing gated on the catalog.** The digits, `.`, `±`, `C` and `=`
  render immediately whatever the catalog does, as ADR-0009 requires. No operation is named in
  application code, so a key appears for whatever the registry publishes — proven by a test that
  serves an operation this codebase has never heard of and drives a computation through it.
- **A catalog that fails shows its message and a `Try again` button.** ADR-0013 rules out
  retry-with-backoff; a retry the user asks for is not that, and the alternative — a calculator
  with a permanently empty operation row — is not a state worth shipping.
- **`useOperations` owns the catalog, separately from `useCalculator`.** The catalog is data the
  app loads once; the calculation is a state machine. Merging them would put an unrelated
  loading state into the machine's surface.
- **`C` stays enabled while a calculation is in flight**, as the escape hatch ADR-0020 names.
  Every other key is disabled, and `=` is disabled whenever the computation is incomplete.
- **The pending expression is a reducer selector, not a component's string.** `pendingExpression`
  renders a unary symbol before its operand (`√ 9`) and a binary one after (`12 ÷`), matching
  how each is read and the order each is pressed. It is derived state, so it is computed rather
  than stored, and it is tested with the rest of the machine.

**Alternatives considered.**
- *Components reading the hook themselves.* Fewer props, but each component would then own a
  piece of the wiring and could not be rendered in a test without the API behind it.
- *A fixed keypad with the seven operations laid out by hand.* The familiar phone-calculator
  grid, and it would make the catalog decorative — the exact outcome ADR-0009 set out to avoid.
- *Reloading the page as the recovery from a failed catalog.* Fewer moving parts, but it
  discards whatever the user had typed into a keypad that was working fine without it.

**Consequences.**
- The operation row is a wrapping grid rather than a fixed column, because the number of
  operations is decided by the server.
- `App` grows one prop per key. That is the cost of components that can be rendered and tested
  with nothing behind them.
- There is no global keyboard handling yet: each key responds to Enter and Space because it is a
  button, but typing `1 + 2` on a physical keyboard does nothing. That is a hook over `keydown`
  mapping to the same actions, and it is not built.

---

## ADR-0022 — Review corrections: preflight detection, detached registry values, origin normalization

**Status:** Accepted · 2026-09-20
**Amends:** ADR-0017 (the `internal/transport/http/` path only; the rest of ADR-0017 remains
in force and is not superseded)

**Context.** A review of the Go backend found three defects, each in code whose own comment or
test claimed the opposite property, plus one table that had outgrown what it was doing. All
four are recorded here because each reverses something an earlier entry stated.

**Decision.**

- **The HTTP package is `internal/transport/httpapi`, package `httpapi`.** ADR-0017 named it
  `internal/transport/http`, package `http`, on the grounds that a package's own name is not
  bound in its file scope. That is true and it compiled, but every importer had to alias it
  and every reader of the package had to disambiguate `http.Handler` from the package they
  were inside. The `transport/` seam that ADR-0017 was actually defending is in the directory
  path and is unaffected.

- **Only a genuine preflight is answered as one.** The CORS middleware short-circuited *every*
  `OPTIONS` request with `204`, before the router saw it. With `ALLOWED_ORIGIN` set,
  `OPTIONS /api/v1/operations/add` answered `204` with no `Allow` header instead of the
  contract's `405 METHOD_NOT_ALLOWED`, and `OPTIONS /metrics` answered `204` with an empty
  body instead of the `404` envelope — so the API broke the ADR-0017 promise that every
  response is JSON, and did so *only in production*, since the middleware is a no-op in
  development. A request is now treated as a preflight only when it is `OPTIONS`, carries the
  allowed `Origin`, **and** carries `Access-Control-Request-Method`. Everything else falls
  through to the router and gets the same answer it gets with CORS disabled. The preflight
  response headers (`Allow-Methods`, `Allow-Headers`, `Max-Age`) moved with it, so they are
  no longer set on ordinary responses where they mean nothing.

- **`calc.Catalog` and `calc.Lookup` return operations detached from the registry.** Both
  returned a struct copy whose `Parameters` slice still aliased registry state, so
  `Catalog()[0].Parameters[0].Name = x` corrupted the catalog for every later request in the
  process. `Catalog`'s doc comment asserted the opposite, and its test only mutated the outer
  slice, so the claim went unchecked. Copying is now one `clone` method used by both, and the
  test mutates through `Parameters`.

- **`ALLOWED_ORIGIN` is normalized, not merely validated.** `parseAllowedOrigin` exists
  because the middleware compares the configured origin to the `Origin` header byte for byte,
  and it rejected trailing slashes and paths for exactly that reason — then returned the raw
  input. `HTTPS://Calculator.Example` passed every check and matched nothing, which is the
  failure the function was written to prevent. It now returns `scheme://host` with the host
  lowercased, and rejects userinfo.

- **`classify` is a switch, not a table of closures.** The `domainFailures` table carried a
  `message func(calc.Operation, int) string` per row that four of six rows ignored, and was
  scanned linearly. A switch says the same thing in less space with nothing to hold in mind.
  The `calc.ErrUnknownOperation` row is **dropped**: `handleExecute` resolves existence before
  calling `calc`, so that row was unconstructable, and its message would have read
  `Unknown operation '<an operation that exists>'`. Reaching `classify` with it now means
  this package is broken, which is what `500 INTERNAL_ERROR` is for, and a test says so.
  The `calc.ErrInvalidOperand` row stays for the reason ADR-0017 gave.

**Alternatives considered.**
- *Leaving `Catalog`'s doc comment and dropping the copy instead*, documenting the returned
  value as read-only. Cheaper, and honest. Rejected because the registry is process-wide
  state reachable from a request handler, and a convention is a weaker guarantee than a copy
  that costs two words.
- *Fixing the preflight check inside the router rather than the middleware.* Would need the
  router to know about CORS, which is the coupling the middleware exists to avoid.
- *Keeping `UNKNOWN_OPERATION` in `classify` as defence in depth*, as with
  `ErrInvalidOperand`. Rejected: `ErrInvalidOperand` degrades to a correct `400` if the decode
  path changes, whereas this row could only ever produce a wrong message, and the 404 it
  duplicates is `handleExecute`'s to return.

**Consequences.**
- Each of the three defects now has a regression test that fails against the previous code:
  `OPTIONS` asserted across all four CORS configurations, mutation through `Parameters` on
  both `Catalog` and `Lookup`, and case normalization of `ALLOWED_ORIGIN`.
- `api/openapi.yaml` and `docs/API_EXAMPLES.md` need no change. Every correction moves the
  implementation *towards* the published contract rather than altering it.
- Comment volume across the backend was cut in the same pass. The rule in CLAUDE.md is that
  prose appears only where the code cannot carry the meaning; three of these four defects
  were in code carrying a comment that asserted the property it did not have, which is the
  argument for fewer comments rather than more.

---

## ADR-0023 — Physical keyboard input, mapped through the catalog

**Status:** Accepted · 2026-09-20

**Context.** ADR-0008 promised full keyboard support on desktop. ADR-0021 delivered a keypad of
real `<button>` elements, which gives Enter and Space on a focused key but nothing else: typing
`12/4` did nothing at all. The gap was reported from use.

**Decision.**

- **What a keystroke means is calculator logic, so `actionForKey(key, operations)` lives in the
  reducer module** as a pure function returning an action or `null`. The hook that listens for
  `keydown` decides nothing; it asks and dispatches.
- **Operator keys resolve through the catalog by symbol**, not through a table of operation
  identifiers. A small alias map translates characters a keyboard has to characters the catalog
  publishes — `/`→`÷`, `*` and `x`→`×`, `-`→`−`, `r`→`√` — and the symbol is then looked up in
  the operations the server sent. Typing an operation's own symbol works without an alias, so an
  operation added in Go with a single-character symbol is typeable with no frontend change, the
  same property ADR-0009 requires of the keypad.
- **`Enter` and `Space` are left to whichever key has focus.** They are how a button is
  activated, and stealing them would break keyboard navigation of the keypad. `=` submits
  regardless of focus, so a user who has been clicking can still finish from the keyboard.
- **Text-entry elements keep their keys** (`input`, `textarea`, `select`, anything
  `contenteditable`), and any keystroke carrying Meta, Control or Alt is left to the browser.
  Everything else is the calculator's, and a key the calculator maps is `preventDefault`ed.
- **Backspace is not mapped**, because the state machine has no action for it. Correcting a
  digit means `C`, as it does on the keypad.

**Alternatives considered.**
- *Mapping keys to operation ids (`/` → `divide`).* Simpler to read, and it would put the seven
  operation names back into frontend code — undoing the property the catalog exists to provide.
- *Taking `Enter` globally and `preventDefault`ing the focused button's activation.* Matches
  what a calculator user expects after clicking, but breaks Enter on the catalog's `Try again`
  button and on every future control, for a case `=` already covers.
- *A `tabIndex` container with `onKeyDown` instead of a window listener.* Scopes the listener
  properly, but only works while the container has focus, so typing would do nothing until the
  user clicked the page first.

**Consequences.**
- Pressing Enter with a keypad button focused re-activates that button rather than submitting.
  That is the browser's behaviour for buttons and is now covered by a test that states it.
- An operation whose symbol is more than one character (`mod`) is clickable but not typeable.
  Aliases are the escape hatch, and adding one is a character-to-character entry.
- `c` and `C` clear, so an operation could not later use `c` as its symbol without colliding.

## ADR-0024 — A carried entry is replaced, not extended

**Status:** Accepted · 2026-09-20

**Context.** Pressing `√` with `5` displayed and then typing `3` computed `sqrt(53)`, not
`sqrt(3)`. ADR-0020's reducer routes a digit to `right` when the selected operation is binary
and to `left` otherwise, and `operationSelected` commits the displayed value to `left`. For a
binary operation the digit lands in a slot that was just cleared, so entry starts fresh; for a
unary one it lands in the slot just populated, so `appendDigit` concatenates. The reported
sequence `√2 + √2` sent `{"operands":[22]}`, which read like broken chaining rather than one
lost keystroke.

Restarting entry was already the behaviour after a result, but it was inferred from
`phase.kind === 'result'` — a condition that is about the request lifecycle, not about who put
the value on screen.

**Decision.** State carries `carriedEntry: boolean`, meaning the displayed value was placed
there by the calculator rather than typed. It is set by `calculationSucceeded` and by
`operationSelected`, cleared by any edit and by `cleared`, and it — not the phase — is what
makes a digit or a decimal point start a new entry. `signToggled` continues to edit the carried
value in place, so `√` then `±` negates the operand shown instead of discarding it.

**Alternatives considered.**
- *Clearing `left` when a unary operation is selected.* Two keystrokes would then be needed to
  take the square root of what is already on screen, and `canSubmit` would go false for a
  computation the user has fully expressed.
- *A fourth phase, `carried`.* The phase union is the request lifecycle and is the dependency
  of the hook's effect (ADR-0020); a value that has nothing to do with requests does not belong
  in it.
- *Fixing only the unary slot.* The same defect would return for any future arity-1 behaviour,
  and the result-phase rule would stay expressed as a coincidence of the lifecycle.

**Consequences.**
- `√2 + √2` computes `sqrt(2)`: the second operation replaces the first, one operation at a
  time per ADR-0009. The `+` is dropped silently, though the pending expression re-renders from
  `2 +` to `√ 2`, which is the only signal the user gets. Chaining remains out of scope
  (ADR-0013).
- The reducer's identity rule needs the flag too: an edit that produces the same string as the
  carried value (`5 √ 5`) must still return new state, or the flag would survive and the next
  digit would restart again. A test covers exactly that.

## ADR-0025 — A rejected computation is not submittable until the user answers it

**Status:** Accepted · 2026-09-20

**Context.** ADR-0020 stated that a failure "clears only the operand the user can retype — the
second one — and keeps the left operand and the operation. `=` is disabled until a new operand
is entered, so the same rejected computation cannot simply be re-sent." The reducer implemented
that by clearing `right`, which holds for binary operations and silently does not for unary
ones: `pendingRequest` reads only `left` when `arity === 1`, so after `√(-4)` returned
`NEGATIVE_SQRT` the request was still fully formed and `=` stayed enabled. Every key of the
identical failing computation could be re-sent by clicking it again. The rule was stated for the
whole machine and enforced for half of it; the `a returned failure` tests were all binary, so
nothing caught it.

**Decision.** The guard is the unanswered failure itself, not the emptied slot. `pendingRequest`
returns `null` while `state.error` is non-null. `error` is set only by `calculationFailed` and
cleared by every action that changes what would be sent — any edit, selecting an operation,
clearing — so "non-null" means exactly "the computation on screen is the one the server just
rejected, unchanged". The rule now covers both arities from one condition rather than from a
side effect of which slot a failure happens to empty.

**Alternatives considered.**
- *Clearing `left` for a unary operation, mirroring `right`.* Literally "clear the rejected
  operand", but it erases the operand from the display and takes `±` — the one-press fix for the
  negative input that causes almost every `NEGATIVE_SQRT` — away with it, since there would no
  longer be a value to negate. ADR-0024 rejected clearing `left` on the same grounds.
- *A `rejected: boolean` flag beside `carriedEntry`.* A second field that would have to be set
  and cleared in exactly the places `error` already is, and could drift out of step with the
  message the user is looking at.
- *Comparing the pending request against the one that failed.* Precise, but it stores a request
  to compare with and answers a question the presence of the message already answers.

**Consequences.**
- A unary failure keeps its operand on display: `√(-4)` shows `-4` under the message, `±` makes
  it `4`, and `=` becomes available again at that keystroke.
- `=` is now inert for the one render between a failure arriving and the user's next key, for
  binary operations too. It was already inert there by way of the cleared `right`; the reason is
  now the stated rule rather than a coincidence.
- Re-selecting the same operation after a failure clears the message and does make the same
  computation submittable again. That is a deliberate keypress rather than a stuck control, and
  it follows the ADR-0020 rule that an operation key commits the displayed value.

---

## ADR-0026 — One image runs both halves, nginx in front of the API on loopback

**Status:** Accepted · 2026-09-20

**Amends:** ADR-0012 (the packaging clauses only; its CORS, logging and observability decisions
stand unchanged)

**Context.** ADR-0012 planned a multi-stage `Dockerfile` for the Go service with a distroless
final image, and a `docker-compose.yml` placing that service and the built frontend behind
nginx. What was asked for is a single image that runs the frontend and backend together. The
two are not compatible: a distroless image contains no nginx and no shell, so the container
that serves the static bundle cannot also be the one that runs the API.

**Decision.** One `Dockerfile` at the repository root, three stages: Node builds the frontend,
Go builds a static binary, and `nginx:alpine` is the runtime carrying both artifacts.

1. nginx listens on `:80` and is the only published port. It serves `dist/` and proxies `/api/`
   and `/healthz` to the Go binary on `127.0.0.1:8080`, which is never exposed.
2. Because every request arrives at one origin, `ALLOWED_ORIGIN` stays empty and the CORS
   middleware stays off — the same arrangement the Vite dev proxy gives locally (ADR-0012).
3. `deploy/entrypoint.sh` is PID 1. It starts both processes, forwards `SIGTERM` to them, and
   exits as soon as either one does, so a crashed API takes the container down instead of
   leaving nginx serving a UI whose every calculation fails.
4. The proxy sets no `client_max_body_size`. The handler's own 4 KiB limit already answers an
   oversized body with the API's error envelope (ADR-0004); an nginx-generated 413 HTML page
   would replace it with something outside the contract.
5. `HEALTHCHECK` fetches `/healthz` through the proxy, which passes only when both processes
   are answering.

**Alternatives considered.**
- *Two images and a compose file, as ADR-0012 planned.* The better production shape — each half
  scales and restarts independently — but it is not one image, and running the exercise then
  needs an orchestrator rather than `docker run`.
- *Serving `dist/` from the Go binary.* One process, no shell, distroless intact. Rejected
  because it puts static-file serving and cache-control policy inside the transport package for
  a packaging reason, and ADR-0012 already names nginx as what serves the frontend.
- *`supervisord` or `s6-overlay` as PID 1.* A supervisor's job is restarting what it watches;
  here a dead process should stop the container, which is twenty lines of `sh`.
- *`wait -n` instead of the poll loop.* Blocks until the first child exits, which is exactly
  what is wanted, but `sh` runs a trap only between foreground commands — a blocking `wait`
  would swallow `SIGTERM` until a child happened to exit.

**Consequences.**
- The final image carries nginx, a shell and busybox rather than the distroless base ADR-0012
  named. That is the price of one container, and the surface is an nginx image's, not a
  full distribution's.
- Both processes run as root, as the stock nginx image does; binding `:80` requires it.
- Signal delivery is up to one second behind the poll interval, which shutdown timeouts absorb.
- There is still no `docker-compose.yml` or `Makefile`. ADR-0012 named both; neither has a
  consumer while there is one image and one command.
