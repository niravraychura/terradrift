# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/terradrift ./cmd/terradrift

FROM alpine:3.24
RUN addgroup -S terradrift && adduser -S -G terradrift terradrift
RUN apk --no-cache add ca-certificates
# Terraform is intentionally not bundled; provide a trusted binary for --terraform-exec.
# Production: derive FROM this image and install a pinned terraform/tofu, or mount one on PATH.
# See README "Docker" for a derived-image example.
COPY --from=builder /out/terradrift /usr/local/bin/terradrift
USER terradrift:terradrift
ENTRYPOINT ["/usr/local/bin/terradrift"]
