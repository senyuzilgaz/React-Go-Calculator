FROM node:24-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.24-alpine AS backend
WORKDIR /src/backend
COPY backend/ ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM nginx:1.29-alpine
COPY --from=frontend /src/frontend/dist /usr/share/nginx/html
COPY --from=backend /out/server /usr/local/bin/server
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --chmod=0755 deploy/entrypoint.sh /usr/local/bin/entrypoint.sh

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
    CMD wget -q -O /dev/null http://127.0.0.1/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
