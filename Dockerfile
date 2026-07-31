# Stage 1: Build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o /judge ./cmd/judge/

# Stage 2: Run
FROM alpine:latest
WORKDIR /app
COPY --from=builder /judge /judge
EXPOSE 8086
CMD ["/judge"]
