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

# The nginx base image sets SIGQUIT, which the entrypoint would have to translate for the API.
# Stopping on the signal the shell traps keeps both shutdowns on one path.
STOPSIGNAL SIGTERM

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
    CMD wget -q -O /dev/null http://127.0.0.1/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
