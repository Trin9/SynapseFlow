FROM golang:1.22-alpine AS builder

WORKDIR /app

# Copy fault-playground only
COPY . .

# Build the specific service passed via build-arg
ARG SERVICE_NAME
RUN go build -o /app/service ./cmd/${SERVICE_NAME}

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/service /app/service

EXPOSE 8080
ENTRYPOINT ["/app/service"]
