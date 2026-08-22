# Multi-stage build for the MLBB analyzer service.
# Produces a small static-binary runtime image that runs migrate + seed + api.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache module downloads first so rebuilds after code edits are fast.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static binaries so the runtime image needs no libc / go toolchain.
# goose is the migration binary the entrypoint uses (go tool goose peer).
RUN CGO_ENABLED=0 go build -o /out/api       ./cmd/api \
 && CGO_ENABLED=0 go build -o /out/seed      ./cmd/seed \
 && CGO_ENABLED=0 go build -o /out/collector ./cmd/collector \
 && CGO_ENABLED=0 go build -o /out/goose github.com/pressly/goose/v3/cmd/goose

# Runtime image: TLS certs (https to AI/upstream APIs) + timezone data.
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Jakarta

WORKDIR /app
COPY --from=build /out/api /out/seed /out/collector /out/goose ./
COPY migrations ./migrations
COPY data/static ./data/static
COPY deploy/docker/entrypoint.sh ./entrypoint.sh
COPY deploy/docker/cron.sh ./cron.sh
RUN chmod +x ./entrypoint.sh ./cron.sh

EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]