FROM golang:1.27.3-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN apk add --no-cache gcc musl-dev

RUN go mod download

COPY . .

ENV CGO_ENABLED=1

ARG VERSION=dev
ARG COMMIT=unknown

RUN GOOS=linux GOARCH=amd64 go build -o ctfhelper \
    -ldflags="-s -w -X github.com/ByteTheCookies/DiscordCTFHelper/internal/config.Version=${VERSION} -X github.com/ByteTheCookies/DiscordCTFHelper/internal/config.Commit=${COMMIT}" \
    ./cmd/ctfhelper

FROM alpine:3.24.3

RUN apk add --no-cache ca-certificates sqlite && adduser -D -u 1000 ctfhelper

COPY --from=builder /app/ctfhelper /ctfhelper

USER ctfhelper

ENTRYPOINT ["/ctfhelper"]
