FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./

ENV GOPROXY=https://goproxy.cn,direct
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w" \
    -trimpath \
    -o /app/bin/pigeongram \
    ./cmd/server

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata curl && \
    update-ca-certificates

RUN addgroup -g 1000 -S app && \
    adduser -u 1000 -S app -G app

WORKDIR /app

COPY --from=builder --chown=app:app /app/bin/pigeongram /app/
COPY --from=builder --chown=app:app /app/web /app/web/

RUN mkdir -p /app/logs && chown -R app:app /app/logs

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD ["./pigeongram"]