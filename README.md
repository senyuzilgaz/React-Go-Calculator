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

Port 8080 is the one the backend uses above, so stop that first — or map another one,
`-p 3000:80`.

## API examples

```bash
curl localhost:8080/healthz
# {"status":"ok"}
```

Every operation takes an `operands` array. `result` comes back as a **string** — the server
owns the rounding, so render it as-is.

```bash
curl -X POST localhost:8080/api/v1/operations/add -d '{"operands":[0.1,0.2]}'
# {"operation":"add","operands":[0.1,0.2],"result":"0.3"}

curl -X POST localhost:8080/api/v1/operations/divide -d '{"operands":[1,3]}'
# {"operation":"divide","operands":[1,3],"result":"0.333333333333"}

curl -X POST localhost:8080/api/v1/operations/sqrt -d '{"operands":[2]}'
# {"operation":"sqrt","operands":[2],"result":"1.41421356237"}

curl -X POST localhost:8080/api/v1/operations/percent -d '{"operands":[15,200]}'
# {"operation":"percent","operands":[15,200],"result":"30"}   — 15% of 200, not modulo
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

The catalog lists what's available — the frontend builds its keypad from it:

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
