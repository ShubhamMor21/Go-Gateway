# ─────────────────────────────────────────────────────────────
# Stage 1 — Build
# Uses the official Go image. CGO is disabled for a fully static binary.
# ─────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# Install ca-certificates for TLS calls from the gateway to gRPC upstreams.
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Download dependencies first — Docker layer cache means this step only reruns
# when go.mod / go.sum change, not on every code change.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build a hardened, stripped binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -extldflags '-static'" \
    -trimpath \
    -o /gateway \
    ./cmd/server

# ─────────────────────────────────────────────────────────────
# Stage 2 — Runtime
# distroless/static has no shell, no package manager, no user tools.
# The attack surface is the gateway binary and nothing else.
# ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the CA bundle so TLS verification works.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /gateway /gateway

# API gateway port and Prometheus metrics port.
EXPOSE 3030 9090

ENTRYPOINT ["/gateway"]
