# Build the backend binaries. Pure-Go throughout (modernc.org/sqlite, no cgo),
# so the result runs on a bare distroless image.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Dependencies first, so edits to source do not re-download the module cache.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/server      ./cmd/server \
 && CGO_ENABLED=0 go build -o /out/seed        ./cmd/seed \
 && CGO_ENABLED=0 go build -o /out/profile-seed ./cmd/profile-seed \
 && CGO_ENABLED=0 go build -o /out/citizen-seed ./cmd/citizen-seed

# The seed steps run several commands in sequence and write PDFs to a mounted
# host directory, so they get a stage with a shell. The server does not.
FROM alpine:3.21 AS tools
COPY --from=build /out/ /usr/local/bin/
ENTRYPOINT ["/bin/sh", "-c"]

FROM gcr.io/distroless/static-debian12:nonroot AS runtime

WORKDIR /app
COPY --from=build /out/ /usr/local/bin/

# The SQLite file and the deployed contract address both live on volumes, so a
# restart keeps the demo's state.
ENV DB_PATH=/data/credentials.db \
    DEPLOYMENT_FILE=/deployment/deployment.txt \
    RPC_URL=http://anvil:8545 \
    ADDR=:8088

EXPOSE 8088
USER nonroot
ENTRYPOINT ["/usr/local/bin/server"]
