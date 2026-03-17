FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -o pigeongram ./cmd/server

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /app/pigeongram .
COPY --from=builder /app/web ./web

EXPOSE 8080
CMD ["./pigeongram"]
