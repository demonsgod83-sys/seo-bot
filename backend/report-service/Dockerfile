FROM golang:1.26.3-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /report-service ./cmd/server/main.go

FROM alpine:3.20

WORKDIR /app
RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /report-service /report-service

EXPOSE 8105
ENTRYPOINT ["/report-service"]
