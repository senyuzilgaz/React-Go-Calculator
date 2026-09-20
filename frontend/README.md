# Calculator frontend

React + TypeScript on Vite (ADR-0008). It performs no arithmetic: it collects operands,
calls the API, and renders the returned string verbatim (ADR-0009).

## Running

```bash
npm install
npm run dev      # http://localhost:5173
```

The Go service must be running on `:8080`; start it with `go run ./cmd/server` from
`../backend`.

| Script | What it does |
|---|---|
| `npm run dev` | Dev server with the API proxy |
| `npm test` | Vitest, one run |
| `npm run test:watch` | Vitest, watching |
| `npm run typecheck` | `tsc -b` |
| `npm run lint` | oxlint |
| `npm run build` | Typecheck, then production bundle into `dist/` |

## Talking to the API

Requests are made to **relative paths** — `/api/v1/operations` — so the browser only ever
calls the origin it was served from. `vite.config.ts` proxies those paths to
`http://localhost:8080` in development and in `vite preview`, which is why CORS never
arises locally (ADR-0012). In production nginx serves `dist/` and proxies the same paths.

## Keyboard

Digits, `.` and `,`, `=` and `Enter`, `Escape` and `c` all work, as do the operator keys — `+`
`-` `*` `/` `^` `%`, `x` for multiply and `r` for square root. Operators are resolved against
the catalog by symbol, so typing an operation's own symbol works too and a new single-character
symbol needs no change here (ADR-0023). `Enter` and `Space` are left to whichever key has
focus, since that is how a button is activated; `=` submits regardless. There is no backspace:
the state machine has no action for it, so corrections go through `C`.

## Layout

```
src/api/types.ts                  the contract's shapes, hand-written from api/openapi.yaml
src/api/errors.ts                 CalcError, the nine contract codes plus two client-side
                                  ones, and the message shown for each
src/api/client.ts                 getOperations and calculate; the only place fetch appears
src/features/calculator/
  reducer.ts                      the whole state machine; no React, no I/O, no arithmetic
  useCalculator.ts                holds the reducer and issues the one request it asks for
  useOperations.ts                the catalog fetch, with a retry
  useKeyboard.ts                  window keydown, dispatched through the reducer's key map
  Display.tsx, Keypad.tsx         props in, buttons out
src/App.tsx                       wiring: the only place an intent becomes an action
src/test/                         MSW handlers, the catalog fixture, and Vitest setup
```

Operation keys are rendered from `GET /api/v1/operations`. No operation is named anywhere in
this codebase outside tests, so registering one in Go puts it on the keypad with no change
here (ADR-0009).
