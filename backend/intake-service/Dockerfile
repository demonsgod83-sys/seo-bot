# ============================================================
#  Multi-stage build — produces a minimal distroless image
#
#  Stage 1 (builder): full Go toolchain, compiles a static binary
#  Stage 2 (runner):  Google distroless/static — no shell, no package
#                     manager, attack surface is basically zero
# ============================================================

# ---- STAGE 1: BUILD ----
FROM golang:1.26-alpine AS builder

# Install CA certificates and git (needed for `go get` over HTTPS)
RUN apk add --no-cache ca-certificates git

WORKDIR /app

# Copy dependency files first — Docker layer-caches this step
# so `go mod download` only re-runs when go.mod/go.sum change.
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build a fully static binary — no CGO, no libc dependency.
# -ldflags "-s -w" strips debug info → smaller binary.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w" \
      -o /app/intake-service \
      ./cmd/server/main.go

# ---- STAGE 2: RUN ----
# gcr.io/distroless/static-debian12: contains only CA certs + tzdata.
# No shell, no package manager — nothing to exploit.
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the compiled binary from the builder stage
COPY --from=builder /app/intake-service /intake-service

# Run as the built-in nonroot user (uid 65532)
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/intake-service"]
