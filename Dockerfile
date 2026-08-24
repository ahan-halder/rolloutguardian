FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download || true

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /rolloutguardian-server ./cmd/rolloutguardian-server
RUN CGO_ENABLED=0 GOOS=linux go build -o /rolloutguardian ./cmd/rolloutguardian

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /rolloutguardian-server /usr/local/bin/rolloutguardian-server
COPY --from=builder /rolloutguardian /usr/local/bin/rolloutguardian
COPY .rolloutguardian.yaml.example .
COPY policies/ policies/
COPY examples/ examples/

EXPOSE 8080
ENTRYPOINT ["rolloutguardian-server"]
