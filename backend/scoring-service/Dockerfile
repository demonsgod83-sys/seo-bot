FROM golang:1.26.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /scoring-service ./cmd/server/main.go

FROM alpine:3.20

WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /scoring-service /scoring-service

EXPOSE 8100
ENTRYPOINT ["/scoring-service"]
