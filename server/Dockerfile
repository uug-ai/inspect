# syntax=docker/dockerfile:1.7

FROM golang:1.25-trixie AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build \
      -trimpath \
      -tags timetzdata,netgo \
      -ldflags '-s -w' \
      -o /out/inspect-server \
      .

FROM alpine:3.22

LABEL org.opencontainers.image.title="inspect" \
      org.opencontainers.image.description="Turns in-app bug reports into GitHub issues" \
      org.opencontainers.image.source="https://github.com/uug-ai/inspect" \
      org.opencontainers.image.vendor="UUG.AI"

RUN apk add --no-cache ca-certificates \
 && addgroup -S app && adduser -S -G app -H -h /app app \
 && mkdir -p /app/data && chown app:app /app/data

COPY --from=builder /out/inspect-server /app/inspect-server

USER app
WORKDIR /app
ENV PORT=8080 INSPECT_LOCAL_DIR=/app/data

EXPOSE 8080

ENTRYPOINT ["/app/inspect-server"]
