## ---- Go backend ----
FROM golang:1.26-alpine AS go-builder
WORKDIR /src
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /out/serverdash ./cmd/serverdash

## ---- Svelte/Node dashboard ----
FROM node:22-alpine AS web-builder
WORKDIR /src
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build
RUN npm prune --omit=dev

## ---- Runtime: both processes, supervised ----
FROM node:22-alpine
# supervisor: runs the Go API and Node web processes under one PID.
# kubectl: optional Kubernetes support — a no-op if no kubeconfig is mounted.
# git, docker-cli(-compose): the git_pull and compose_up workflow blocks —
# ServerDash shells out to these against the mounted docker.sock, so it
# needs its own copies even though it never runs a container build itself.
RUN apk add --no-cache supervisor kubectl git docker-cli docker-cli-compose
WORKDIR /app

COPY --from=go-builder /out/serverdash /app/serverdash
COPY --from=web-builder /src/dist /app/web/dist
COPY --from=web-builder /src/server /app/web/server
COPY --from=web-builder /src/node_modules /app/web/node_modules
COPY --from=web-builder /src/package.json /app/web/package.json
COPY docker/supervisord.conf /etc/supervisord.conf

ENV SERVERDASH_LISTEN_ADDR=:8080 \
    SERVERDASH_API_URL=http://127.0.0.1:8080 \
    SERVERDASH_DOCKER_HOST=unix:///var/run/docker.sock \
    SERVERDASH_PUBLIC_HOST=nova.blacklink.net \
    SERVERDASH_DB_PATH=/data/serverdash.db \
    PORT=9900

# Users, sessions, nicknames, automation rules, and scripts all live here —
# mount a volume at /data so they survive container recreation.
VOLUME /data

# Only the web UI's port needs to be published; the Go API is reached
# internally over 127.0.0.1 by the Node process.
EXPOSE 9900

CMD ["supervisord", "-c", "/etc/supervisord.conf", "-n"]
