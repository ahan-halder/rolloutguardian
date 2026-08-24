FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /rolloutguardian ./cmd/rolloutguardian

FROM alpine:latest
RUN apk --no-cache add ca-certificates curl jq
WORKDIR /root/
COPY --from=builder /rolloutguardian /usr/local/bin/rolloutguardian
COPY deploy/harness-step/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
COPY .rolloutguardian.yaml.example .rolloutguardian.yaml
COPY policies/ policies/
COPY examples/ examples/

ENTRYPOINT ["/entrypoint.sh"]
