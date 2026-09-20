# API Examples

Worked request/response pairs for every endpoint and every failure mode, intended as the source
for the README's API section.

**Every value below was computed, not hand-written.** Results were produced by executing the
rounding policy from ADR-0003/ADR-0015 against true runtime IEEE-754 binary64 arithmetic, and
the JSON bodies were produced by marshalling the actual response structures. Where a raw float64
differs from the returned string, the raw value is shown so the rounding is visible rather than
implied.

Base URL for all examples: `http://localhost:8080`

Conventions: operands are sent as JSON numbers; `result` is returned as a **string** already
rounded and formatted for display, and is rendered verbatim by clients (ADR-0003).

---

## Health

```bash
curl -sS http://localhost:8080/healthz
```

**200 OK**

```json
{
  "status": "ok"
}
```

---

## Operation catalog

```bash
curl -sS http://localhost:8080/api/v1/operations
```

**200 OK**

```json
{
  "operations": [
    {
      "id": "add",
      "name": "Addition",
      "symbol": "+",
      "arity": 2,
      "parameters": [
        { "name": "augend", "description": "The value added to." },
        { "name": "addend", "description": "The value added." }
      ],
      "endpoint": "/api/v1/operations/add"
    },
    {
      "id": "subtract",
      "name": "Subtraction",
      "symbol": "−",
      "arity": 2,
      "parameters": [
        { "name": "minuend", "description": "The value subtracted from." },
        { "name": "subtrahend", "description": "The value subtracted." }
      ],
      "endpoint": "/api/v1/operations/subtract"
    },
    {
      "id": "multiply",
      "name": "Multiplication",
      "symbol": "×",
      "arity": 2,
      "parameters": [
        { "name": "multiplicand", "description": "The value multiplied." },
        { "name": "multiplier", "description": "The number of times it is taken." }
      ],
      "endpoint": "/api/v1/operations/multiply"
    },
    {
      "id": "divide",
      "name": "Division",
      "symbol": "÷",
      "arity": 2,
      "parameters": [
        { "name": "dividend", "description": "The value divided." },
        { "name": "divisor", "description": "The value divided by. Must not be zero." }
      ],
      "endpoint": "/api/v1/operations/divide"
    },
    {
      "id": "power",
      "name": "Exponentiation",
      "symbol": "^",
      "arity": 2,
      "parameters": [
        { "name": "base", "description": "The value raised to a power." },
        { "name": "exponent", "description": "The power the base is raised to." }
      ],
      "endpoint": "/api/v1/operations/power"
    },
    {
      "id": "sqrt",
      "name": "Square root",
      "symbol": "√",
      "arity": 1,
      "parameters": [
        { "name": "radicand", "description": "The value whose square root is taken. Must not be negative." }
      ],
      "endpoint": "/api/v1/operations/sqrt"
    },
    {
      "id": "percent",
      "name": "Percentage",
      "symbol": "%",
      "arity": 2,
      "parameters": [
        { "name": "percentage", "description": "The percentage to take." },
        { "name": "value", "description": "The value the percentage is taken of." }
      ],
      "endpoint": "/api/v1/operations/percent"
    }
  ]
}
```

---

## Success — one per operation

### Addition

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/add \
  -H 'Content-Type: application/json' \
  -d '{"operands":[0.1,0.2]}'
```

**200 OK**

```json
{
  "operation": "add",
  "operands": [
    0.1,
    0.2
  ],
  "result": "0.3"
}
```

> The raw binary64 sum is `0.30000000000000004`. Rounding to 12 significant digits yields `0.3`.

### Subtraction

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/subtract \
  -H 'Content-Type: application/json' \
  -d '{"operands":[0.3,0.1]}'
```

**200 OK**

```json
{
  "operation": "subtract",
  "operands": [
    0.3,
    0.1
  ],
  "result": "0.2"
}
```

> Raw binary64 difference: `0.19999999999999998`.

### Multiplication

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/multiply \
  -H 'Content-Type: application/json' \
  -d '{"operands":[1.1,1.1]}'
```

**200 OK**

```json
{
  "operation": "multiply",
  "operands": [
    1.1,
    1.1
  ],
  "result": "1.21"
}
```

> Raw binary64 product: `1.2100000000000002`.

### Division

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/divide \
  -H 'Content-Type: application/json' \
  -d '{"operands":[10,4]}'
```

**200 OK**

```json
{
  "operation": "divide",
  "operands": [
    10,
    4
  ],
  "result": "2.5"
}
```

### Exponentiation

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/power \
  -H 'Content-Type: application/json' \
  -d '{"operands":[2,10]}'
```

**200 OK**

```json
{
  "operation": "power",
  "operands": [
    2,
    10
  ],
  "result": "1024"
}
```

### Square root — the unary case

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/sqrt \
  -H 'Content-Type: application/json' \
  -d '{"operands":[2]}'
```

**200 OK**

```json
{
  "operation": "sqrt",
  "operands": [
    2
  ],
  "result": "1.41421356237"
}
```

> Raw binary64 root: `1.4142135623730951`. Square root is approximate by nature; the policy
> truncates the display to 12 significant digits but does not make it exact.

### Percentage

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/percent \
  -H 'Content-Type: application/json' \
  -d '{"operands":[15,200]}'
```

**200 OK**

```json
{
  "operation": "percent",
  "operands": [
    15,
    200
  ],
  "result": "30"
}
```

> `percent(a, b) = a × b ÷ 100` — "15 percent of 200" (ADR-0010).

---

## The rounding policy, demonstrated

These are the cases worth putting in the README, because they show the policy doing something
observable rather than merely being described.

| Operation | Operands | Raw binary64 | Returned `result` | Why |
|---|---|---|---|---|
| `add` | `0.1, 0.2` | `0.30000000000000004` | `"0.3"` | Representation error absorbed |
| `subtract` | `0.3, 0.1` | `0.19999999999999998` | `"0.2"` | Representation error absorbed |
| `multiply` | `1.1, 1.1` | `1.2100000000000002` | `"1.21"` | Representation error absorbed |
| `divide` | `1, 3` | `0.3333333333333333` | `"0.333333333333"` | Truncated to 12 significant digits |
| `multiply` | `123456789, 987654321` | `1.2193263111263526e+17` | `"1.21932631113e+17"` | ≥ `1e12`, scientific notation |
| `divide` | `1, 1000000000` | `1e-09` | `"0.000000001"` | ≥ `1e-9`, stays decimal |
| `divide` | `1, 10000000000` | `1e-10` | `"1e-10"` | < `1e-9`, scientific notation |
| `multiply` | `0, -5` | `-0` | `"0"` | Negative zero normalized |

The last three are the band edges and the negative-zero case — the ones most likely to be wrong
in an implementation, and therefore the ones worth asserting in tests.

Large-magnitude example in full:

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/multiply \
  -H 'Content-Type: application/json' \
  -d '{"operands":[123456789,987654321]}'
```

**200 OK**

```json
{
  "operation": "multiply",
  "operands": [
    123456789,
    987654321
  ],
  "result": "1.21932631113e+17"
}
```

---

## Errors — one per code

### `400 INVALID_JSON` — body is not parseable

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/add \
  -H 'Content-Type: application/json' \
  -d '{"operands":[1,2'
```

**400 Bad Request**

```json
{
  "error": {
    "code": "INVALID_JSON",
    "message": "Request body is not valid JSON"
  }
}
```

> Also returned when the request body exceeds the configured size limit.

### `400 INVALID_OPERAND` — operand is not a finite number

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/add \
  -H 'Content-Type: application/json' \
  -d '{"operands":["abc",2]}'
```

**400 Bad Request**

```json
{
  "error": {
    "code": "INVALID_OPERAND",
    "message": "Operand at position 1 is not a finite number"
  }
}
```

> A type mismatch *inside* `operands` is reported as `INVALID_OPERAND` rather than
> `INVALID_JSON`, because the position of the offending operand is known and more useful to the
> caller. `INVALID_JSON` is reserved for bodies that cannot be parsed at all.

### `400 WRONG_OPERAND_COUNT` — arity mismatch

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/sqrt \
  -H 'Content-Type: application/json' \
  -d '{"operands":[9,16]}'
```

**400 Bad Request**

```json
{
  "error": {
    "code": "WRONG_OPERAND_COUNT",
    "message": "Operation 'sqrt' requires exactly 1 operand, received 2"
  }
}
```

### `404 UNKNOWN_OPERATION` — no such operation

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/modulo \
  -H 'Content-Type: application/json' \
  -d '{"operands":[10,3]}'
```

**404 Not Found**

```json
{
  "error": {
    "code": "UNKNOWN_OPERATION",
    "message": "Unknown operation 'modulo'"
  }
}
```

### `405 METHOD_NOT_ALLOWED` — wrong method on a real path

```bash
curl -sS http://localhost:8080/api/v1/operations/add
```

**405 Method Not Allowed** · `Allow: POST`

```json
{
  "error": {
    "code": "METHOD_NOT_ALLOWED",
    "message": "Method GET is not supported for this resource"
  }
}
```

### `422 DIVISION_BY_ZERO`

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/divide \
  -H 'Content-Type: application/json' \
  -d '{"operands":[10,0]}'
```

**422 Unprocessable Content**

```json
{
  "error": {
    "code": "DIVISION_BY_ZERO",
    "message": "Cannot divide by zero"
  }
}
```

> In binary64 this computation yields `+Inf`, which is not representable in JSON. The guard
> converts it to a domain error rather than letting it reach the encoder.

### `422 NEGATIVE_SQRT`

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/sqrt \
  -H 'Content-Type: application/json' \
  -d '{"operands":[-4]}'
```

**422 Unprocessable Content**

```json
{
  "error": {
    "code": "NEGATIVE_SQRT",
    "message": "Cannot take the square root of a negative number"
  }
}
```

> Raw computation yields `NaN`.

### `422 RESULT_OVERFLOW`

```bash
curl -sS -X POST http://localhost:8080/api/v1/operations/power \
  -H 'Content-Type: application/json' \
  -d '{"operands":[1e308,2]}'
```

**422 Unprocessable Content**

```json
{
  "error": {
    "code": "RESULT_OVERFLOW",
    "message": "Result is too large to represent"
  }
}
```

> Raw computation yields `+Inf`. Note the distinction from `INVALID_OPERAND`: this is a
> non-finite **result** from valid inputs, whereas `INVALID_OPERAND` is a non-finite **input**.

### `500 INTERNAL_ERROR`

Not reproducible on demand — emitted by the panic-recovery middleware when an unexpected fault
occurs. The response deliberately carries no internal detail; the `X-Request-Id` header
correlates it with the server log entry that does.

**500 Internal Server Error**

```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "An unexpected error occurred"
  }
}
```

---

## Summary table for the README

| Status | Code | Trigger |
|---|---|---|
| `400` | `INVALID_JSON` | Body is not parseable JSON, or exceeds the size limit |
| `400` | `INVALID_OPERAND` | An operand is not a finite number |
| `400` | `WRONG_OPERAND_COUNT` | Operand count does not match the operation's arity |
| `404` | `UNKNOWN_OPERATION` | No operation with that identifier exists |
| `405` | `METHOD_NOT_ALLOWED` | Path exists but does not support the method |
| `422` | `DIVISION_BY_ZERO` | Divisor is zero |
| `422` | `NEGATIVE_SQRT` | Square root of a negative value |
| `422` | `RESULT_OVERFLOW` | Result is not finite |
| `500` | `INTERNAL_ERROR` | Unexpected server fault |
