# syntax=docker/dockerfile:1.6
ARG GO_VERSION=1.25

FROM golang:${GO_VERSION}-alpine AS builder
ARG BOT
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -n "$BOT" || (echo "ERROR: --build-arg BOT=<bot-name> is required" && exit 1)
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/${BOT}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/bot /usr/local/bin/bot
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/bot"]
