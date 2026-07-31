# syntax=docker/dockerfile:1

#############################
# Stage 1: build
#############################
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Dependabot-friendly: cache deps before copying source
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o judge \
    -ldflags='-s -w' \
    ./cmd/judge/

#############################
# Stage 2: run
#############################
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /src/judge /app/judge

USER 65534:65534

EXPOSE 8086

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=2 \
  CMD wget -qO- http://localhost:8086/health || exit 1

ENTRYPOINT ["/app/judge"]
