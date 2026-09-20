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

## Or run it in Docker

One image, both halves. nginx serves the frontend and proxies the API (ADR-0026).

```bash
docker build -t calculator .
docker run --rm -p 8080:80 calculator
```

Open **http://localhost:8080**.

## Test it

```bash
cd backend  && go test ./...
cd frontend && npm test
```
