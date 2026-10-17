FROM golang:1.25.5-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN apk add --no-cache gcc musl-dev

RUN go mod download

COPY . .

ENV CGO_ENABLED=1

RUN GOOS=linux GOARCH=amd64 go build -o ctfhelper -ldflags="-s -w" ./cmd/ctfhelper

FROM alpine:3.24.2

RUN apk add --no-cache ca-certificates sqlite

COPY --from=builder /app/ctfhelper /ctfhelper

ENTRYPOINT ["/ctfhelper"]
