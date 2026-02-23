# Multi-stage build для сервисов Automata.

# Stage 1: сборка Go-бинарника
FROM golang:1.24 AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG SERVICE
RUN CGO_ENABLED=0 go build -o /bin/service ./cmd/${SERVICE}/

# Stage 2: минимальный образ для запуска
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /bin/service /bin/service

ENTRYPOINT ["/bin/service"]
