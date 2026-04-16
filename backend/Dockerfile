# ── Stage 1: builder ──────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first (layer cache)
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /synapse cmd/synapse/main.go

# ── Stage 2: runtime ──────────────────────────────────────────────────────────
FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget

# Copy the compiled binary
COPY --from=builder /synapse /synapse

# Copy default MCP config (empty; override via volume mount or env)
COPY --from=builder /app/configs/ /configs/

EXPOSE 8080

ENTRYPOINT ["/synapse"]
