# Standalone deploy target: the registry server and its own private Anvil node
# in one container, talking over loopback only. Built for hosts (a single
# Railway service, in particular) where a second networked service for the
# chain is more trouble than it is worth — this removes that network path
# entirely rather than trying to make it reliable.
#
# Lives at the repo root, not backend/, because it needs both backend/ and
# contracts/ in its build context; backend/Dockerfile (used locally and by
# docker-compose.yml) is unaffected by this file.

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/. .
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM ghcr.io/foundry-rs/foundry:latest

WORKDIR /contracts
COPY contracts /contracts

COPY --from=build /out/server /usr/local/bin/server
COPY --chmod=755 deploy/standalone/start.sh /start.sh

ENV DATA_DIR=/data \
    ADDR=:8088

EXPOSE 8088
ENTRYPOINT ["/start.sh"]
