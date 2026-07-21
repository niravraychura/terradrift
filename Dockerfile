# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/terradrift ./cmd/terradrift

FROM alpine:3.22
RUN addgroup -S terradrift && adduser -S -G terradrift terradrift
RUN apk --no-cache add ca-certificates
# Terraform will be included when command execution is implemented.
COPY --from=builder /out/terradrift /usr/local/bin/terradrift
USER terradrift:terradrift
ENTRYPOINT ["/usr/local/bin/terradrift"]
