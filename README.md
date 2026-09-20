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

## Test it

```bash
cd backend  && go test ./...
cd frontend && npm test
```
