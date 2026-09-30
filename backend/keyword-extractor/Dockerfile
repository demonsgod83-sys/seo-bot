# ---- STAGE 1: BUILD ----
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache ca-certificates git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -ldflags="-s -w" \
      -o /app/keyword-extractor \
      ./cmd/server/main.go

# ---- STAGE 2: RUN ----
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /app/keyword-extractor /keyword-extractor

USER nonroot:nonroot

EXPOSE 8099

ENTRYPOINT ["/keyword-extractor"]
