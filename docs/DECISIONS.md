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
