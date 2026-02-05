# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go/go.mod go/go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY go/ .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /pubsub-scheduler-worker ./examples/main.go

# Runtime stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /pubsub-scheduler-worker .

# Run the worker
CMD ["./pubsub-scheduler-worker"]
