FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service ./cmd/main.go

FROM alpine:3.20

RUN apk --no-cache add ca-certificates

WORKDIR /app
COPY --from=builder /auth-service .

EXPOSE 8081 50051

CMD ["./auth-service"]
