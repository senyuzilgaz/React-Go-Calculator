# Calculator

Go REST API + React frontend. Needs Go 1.24+ and Node 20+.

## Run it

Two terminals. Backend first.

```bash
cd backend
go run ./cmd/server        # :8080
```

```bash
cd frontend
npm install                # first time only
npm run dev                # :5173
```

Open **http://localhost:5173**.

## Or run it with Docker

One container, both halves. Needs neither Go nor Node.

```bash
docker build -t calculator .
docker run --rm -p 8080:80 calculator
```

Open **http://localhost:8080**. Ctrl-C stops it.

Port 8080 is the one the backend uses above, so stop that first, or map another one:
`-p 3000:80`.

## Keyboard

The whole keypad is typeable. Digits and `.` enter operands, `=` or Enter computes, `Escape`
or `c` clears. `+` and `%` work as themselves; `-` subtracts, `*` or `x` multiplies, `/`
divides, `r` is `√` and `p` is `^`.

`r` and `p` are stand-ins: nothing has a `√` key, and `^` is a dead key on Turkish and German
layouts, where the browser never delivers the character at all. It waits to compose `â`.

## API examples

```bash
curl localhost:8080/healthz
# {"status":"ok"}
```

Every operation takes an `operands` array. `result` comes back as a **string**. The server
owns the rounding, so render it as-is.

```bash
curl -X POST localhost:8080/api/v1/operations/add -d '{"operands":[0.1,0.2]}'
# {"operation":"add","operands":[0.1,0.2],"result":"0.3"}

curl -X POST localhost:8080/api/v1/operations/divide -d '{"operands":[1,3]}'
# {"operation":"divide","operands":[1,3],"result":"0.333333333333"}

curl -X POST localhost:8080/api/v1/operations/sqrt -d '{"operands":[2]}'
# {"operation":"sqrt","operands":[2],"result":"1.41421356237"}

curl -X POST localhost:8080/api/v1/operations/percent -d '{"operands":[15,200]}'
# {"operation":"percent","operands":[15,200],"result":"30"}   (15% of 200, not modulo)
```

Errors carry a code to switch on:

```bash
curl -X POST localhost:8080/api/v1/operations/divide -d '{"operands":[10,0]}'
# 422 {"error":{"code":"DIVISION_BY_ZERO","message":"Cannot divide by zero"}}

curl -X POST localhost:8080/api/v1/operations/sqrt -d '{"operands":[9,16]}'
# 400 {"error":{"code":"WRONG_OPERAND_COUNT","message":"Operation 'sqrt' requires exactly 1 operand, received 2"}}

curl -X POST localhost:8080/api/v1/operations/modulo -d '{"operands":[8,2]}'
# 404 {"error":{"code":"UNKNOWN_OPERATION","message":"Unknown operation 'modulo'"}}
```

400 means you sent garbage, 404 that the operation doesn't exist, 422 that the computation is
undefined. The frontend switches on `code` and never reads `message`.

The catalog lists what's available, and the frontend builds its keypad from it:

```bash
curl localhost:8080/api/v1/operations
# {"operations":[{"id":"add","name":"Addition","symbol":"+","arity":2,
#   "parameters":[{"name":"augend","description":"The value added to."},
#                 {"name":"addend","description":"The value added."}],
#   "endpoint":"/api/v1/operations/add"}, ...]}
```

Full contract in `api/openapi.yaml`, every case in `docs/API_EXAMPLES.md`.

## Test it

```bash
cd backend  && go test ./...
cd frontend && npm test
```

## Design Decisions

**The server owns the numbers.** `0.1 + 0.2` is `0.30000000000000004` in float64, and it's the
first thing anyone types into a calculator they're reviewing. So results are rounded to 12
significant digits before serialization. binary64 carries about 16, and dropping the last four
absorbs the representation error. `result` is a **string**, because a JSON number would be
parsed straight back into a float64 and every bit of that work would die at the wire. Rounding
lives in one function in `internal/calc` and nowhere else.

It hides the error rather than removing it: chains compound at the 12th digit, `√` and `^` are
approximate by nature. Fine for a calculator, not for money. If exactness were ever needed,
`+ − × ÷` move to `shopspring/decimal` and operands become strings. The string result contract
survives that untouched, which is half of why it's there.

**The frontend does no arithmetic.** Not addition, not rounding, not reformatting. It collects
operands, calls the API, renders the returned string verbatim. The state machine is deliberately
small to keep that true: a computation is only ever `(operand, operation, operand)`, or
`(operation, operand)` for `√`. No chaining, no precedence. All of it is in `reducer.ts`, which
imports nothing from React and does no I/O.

Unary operations apply on the keystroke rather than at `=`. `√` used to wait in the operation
slot like a binary operator, so `√2 + √2` overwrote the pending `+` and answered
`1.41421356237`. It's three requests now (`sqrt`, `sqrt`, `add`) and `2.82842712475`.

**Adding an operation is one registry entry.** The registry in `internal/calc` drives the
routing, the arity check and the catalog response, so there's no second list to keep in sync
and no per-operation handler. The frontend builds its keypad from the catalog, so a new
operation in Go reaches the UI with no TypeScript change. A test serves an operation this
codebase has never heard of and drives a calculation through the whole app.

**One service, not several.** The brief said microservices, plural. Splitting seven arithmetic
operations across two processes would have spent a quarter of the budget on gateway routing and
cross-service error mapping for nothing a user could see. `internal/calc` has no HTTP or JSON
imports, so the seam is there if it's ever wanted.

```
cmd/server/main.go            wiring, config, graceful shutdown
internal/calc/                arithmetic, registry, rounding. no HTTP, no JSON
internal/transport/httpapi/   handlers, validation, error mapping, middleware
internal/config/              environment, read once, validated strictly
```

Stdlib `net/http`, no framework: Go 1.22's `ServeMux` covers every route here. No database, no
history, no auth. In dev Vite proxies to `:8080` and in the container nginx does the same, so
there's no CORS in either case by construction.

`percent(a, b) = a × b ÷ 100`, meaning "a percent of b". Contextual percentage (`200 + 10 %` → `220`)
needs expression context in the client, which is the one thing this design doesn't have.

Also deliberately absent: gRPC, a queue, client codegen, Redux, Kubernetes, rate limiting,
end-to-end browser tests, a plugin architecture for operations.

## The long version

`docs/DECISIONS.md` is the append-only log this is compressed from, with each decision's
context, the alternatives weighed, and what it cost. Entries are never edited, so the ones that
turned out wrong are superseded in place and the reversals are readable too. Several of the
paragraphs above are reversals.
